package backup

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/proxmoxclient"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
)

const (
	testBackupUPID    = "UPID:pve6:00244B05:02965010:6B065F29:vzdump:106:backup@pve!goback:"
	testBackupTimeout = 10 * time.Second
	testPollInterval  = time.Millisecond
)

// Canned task-status responses. A poll either answers with a Proxmox task status
// or fails with an HTTP status code, the way a dropped proxied connection (596) or
// a pveproxy error (500) does.
var (
	pollRunning     = taskPoll{status: "running"}
	pollOK          = taskPoll{status: "stopped", exitStatus: "OK"}
	pollFailed      = taskPoll{status: "stopped", exitStatus: "command 'rsync' failed: exit code 23"}
	pollBrokenPipe  = taskPoll{httpStatus: 596}
	pollServerError = taskPoll{httpStatus: http.StatusInternalServerError}
)

// TestBackupVMsExecutePolling drives a single guest's backup through a fake
// Proxmox API and scripts the task-status polls. Transient poll failures must not
// fail the backup, because the vzdump task keeps running on the node regardless;
// only a run of them long enough to mean the API is really gone may.
func TestBackupVMsExecutePolling(t *testing.T) {
	tests := []struct {
		name      string
		polls     []taskPoll
		wantErr   string
		wantPolls int
	}{
		{
			name:      "task finishes ok",
			polls:     []taskPoll{pollRunning, pollRunning, pollOK},
			wantPolls: 3,
		},
		{
			name:      "task fails",
			polls:     []taskPoll{pollRunning, pollFailed},
			wantErr:   "1 backup(s) failed:\n  - backup failed for VMID 106: backup failed with exit status: command 'rsync' failed: exit code 23",
			wantPolls: 2,
		},
		{
			name:      "transient poll failures are retried",
			polls:     []taskPoll{pollRunning, pollBrokenPipe, pollServerError, pollRunning, pollOK},
			wantPolls: 5,
		},
		{
			name:      "a successful poll resets the failure count",
			polls:     append(append(repeatPoll(pollBrokenPipe, taskStatusMaxConsecutiveFailures-1), pollRunning), append(repeatPoll(pollBrokenPipe, taskStatusMaxConsecutiveFailures-1), pollOK)...),
			wantPolls: 2 * taskStatusMaxConsecutiveFailures,
		},
		{
			name:      "persistent poll failures give up",
			polls:     repeatPoll(pollBrokenPipe, taskStatusMaxConsecutiveFailures),
			wantErr:   "1 backup(s) failed:\n  - backup failed for VMID 106: task status unavailable after 30 consecutive polls: unexpected status code: 596 (status code 596)",
			wantPolls: taskStatusMaxConsecutiveFailures,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pve := &fakeProxmox{polls: tt.polls}
			ts := httptest.NewServer(pve)
			defer ts.Close()

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			client, err := proxmoxclient.New(ts.URL, proxmoxclient.WithLogger(logger))
			require.NoError(t, err)

			a := &BackupVMs{
				ProxmoxClient:       client,
				Logger:              logger,
				Registry:            newTestRegistry(t),
				StatusLine:          activity.NewStatusLine(workflow.ActivityID{Module: "backup", Type: "BackupVMs"}, logger, nil),
				BackupTimeout:       testBackupTimeout,
				Storage:             "pbs",
				MaxBackupAge:        time.Hour,
				Mode:                "snapshot",
				StatusCheckInterval: testPollInterval,
			}
			require.NoError(t, a.Init())

			err = a.Execute(context.Background())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, 1, pve.backupsStarted(), "vzdump requests")
			assert.Equal(t, tt.wantPolls, pve.pollCount(), "status polls")
		})
	}
}

// repeatPoll returns n copies of p.
func repeatPoll(p taskPoll, n int) []taskPoll {
	polls := make([]taskPoll, 0, n)
	for i := 0; i < n; i++ {
		polls = append(polls, p)
	}
	return polls
}

// newTestRegistry returns a fresh metrics registry so each test can register the
// activity's metrics without colliding with another test's.
func newTestRegistry(t *testing.T) metrics.Registry {
	t.Helper()
	registry, err := metrics.NewScrapeRegistry()
	require.NoError(t, err)
	return registry
}

// taskPoll is one scripted answer to a task-status poll. A non-zero httpStatus
// fails the request with that code; otherwise the task status is returned.
type taskPoll struct {
	httpStatus int
	status     string
	exitStatus string
}

// fakeProxmox serves just enough of the PVE API for BackupVMs to back up one
// running container (vmid 106 on pve6) that has never been backed up: version,
// cluster resources, an empty backup listing, the vzdump call, and a scripted
// sequence of task-status answers. Once the script runs out the last answer
// repeats, so a test cannot hang on an exhausted script.
type fakeProxmox struct {
	polls []taskPoll

	mu      sync.Mutex
	backups int
	polled  int
}

func (f *fakeProxmox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api2/json/version":
		w.Write([]byte(`{"data":{"release":"9.2","version":"9.2.28"}}`))
	case r.Method == http.MethodGet && r.URL.Path == "/api2/json/cluster/resources":
		w.Write([]byte(`{"data":[{"vmid":106,"name":"victoriametrics","node":"pve6","status":"running","type":"lxc"}]}`))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/storage/pbs/content"):
		w.Write([]byte(`{"data":[]}`))
	case r.Method == http.MethodPost && r.URL.Path == "/api2/json/nodes/pve6/vzdump":
		f.mu.Lock()
		f.backups++
		f.mu.Unlock()
		w.Write([]byte(`{"data":"` + testBackupUPID + `"}`))
	case r.Method == http.MethodGet && r.URL.Path == "/api2/json/nodes/pve6/tasks/"+testBackupUPID+"/status":
		f.mu.Lock()
		poll := f.polls[min(f.polled, len(f.polls)-1)]
		f.polled++
		f.mu.Unlock()
		if poll.httpStatus != 0 {
			w.WriteHeader(poll.httpStatus)
			return
		}
		w.Write([]byte(`{"data":{"upid":"` + testBackupUPID + `","status":"` + poll.status + `","exitstatus":"` + poll.exitStatus + `"}}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// backupsStarted returns how many vzdump requests were made.
func (f *fakeProxmox) backupsStarted() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.backups
}

// pollCount returns how many task-status polls were answered.
func (f *fakeProxmox) pollCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.polled
}
