package maintenance

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
)

const (
	testPruneUPID = "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:"
	testGCUPID    = "UPID:pbs:00001234:0000ABCD:00000002:6900A1B2:garbage_collection:tank:root@pam:"
	testTimeout   = time.Minute
	// testShortTimeout makes the wait give up on its first poll without sleeping
	// in the test: the task is reported as running and the deadline is already past.
	testShortTimeout = time.Nanosecond
)

func TestPrunePBSInit(t *testing.T) {
	tests := []struct {
		name     string
		activity PrunePBS
		wantErr  string
	}{
		{
			name:     "valid",
			activity: PrunePBS{Token: "t", PruneJob: "goback-prune", TaskTimeout: testTimeout},
		},
		{
			name:     "missing prune job",
			activity: PrunePBS{Token: "t", TaskTimeout: testTimeout},
			wantErr:  "missing PBS config: prune_job is not set",
		},
		{
			name:     "missing token",
			activity: PrunePBS{PruneJob: "goback-prune", TaskTimeout: testTimeout},
			wantErr:  "missing PBS config: token is not set",
		},
		{
			name:     "non-positive timeout",
			activity: PrunePBS{Token: "t", PruneJob: "goback-prune"},
			wantErr:  "PBS task_timeout must be positive, got 0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.activity.Registry = newTestRegistry(t)
			err := tt.activity.Init()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGarbageCollectPBSInit(t *testing.T) {
	tests := []struct {
		name     string
		activity GarbageCollectPBS
		wantErr  string
	}{
		{
			name:     "valid",
			activity: GarbageCollectPBS{Token: "t", Datastore: "tank", TaskTimeout: testTimeout},
		},
		{
			name:     "missing datastore",
			activity: GarbageCollectPBS{Token: "t", TaskTimeout: testTimeout},
			wantErr:  "missing PBS config: datastore is not set",
		},
		{
			name:     "missing token",
			activity: GarbageCollectPBS{Datastore: "tank", TaskTimeout: testTimeout},
			wantErr:  "missing PBS config: token is not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.activity.Registry = newTestRegistry(t)
			err := tt.activity.Init()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrunePBSExecute(t *testing.T) {
	tests := []struct {
		name    string
		server  *fakePBS
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "prune job finishes ok",
			server:  &fakePBS{runStatus: http.StatusOK, taskExit: "OK"},
			timeout: testTimeout,
		},
		{
			name:    "prune job finishes with warnings",
			server:  &fakePBS{runStatus: http.StatusOK, taskExit: "WARNINGS: 1"},
			timeout: testTimeout,
		},
		{
			name:    "prune job fails",
			server:  &fakePBS{runStatus: http.StatusOK, taskExit: "unable to acquire lock on group vm/101"},
			timeout: testTimeout,
			wantErr: "prune job goback-prune failed after ",
		},
		{
			name:    "prune job cannot be started",
			server:  &fakePBS{runStatus: http.StatusForbidden},
			timeout: testTimeout,
			wantErr: "failed to start prune job goback-prune: unexpected status code: 403 (Forbidden)",
		},
		{
			name:    "prune job does not finish in time",
			server:  &fakePBS{runStatus: http.StatusOK, taskRunning: true},
			timeout: testShortTimeout,
			wantErr: "prune job goback-prune did not finish within 1ns; task " + testPruneUPID + " keeps running on PBS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.server.upid = testPruneUPID
			ts := httptest.NewServer(tt.server)
			defer ts.Close()

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			client, err := pbsclient.New(ts.URL, pbsclient.WithToken("goback@pbs!maint:secret"), pbsclient.WithLogger(logger))
			require.NoError(t, err)

			a := &PrunePBS{
				PBSClient:   client,
				Logger:      logger,
				StatusLine:  activity.NewStatusLine(workflow.ActivityID{Module: "maintenance", Type: "PrunePBS"}, logger, nil),
				Registry:    newTestRegistry(t),
				Token:       "goback@pbs!maint:secret",
				PruneJob:    "goback-prune",
				TaskTimeout: tt.timeout,
			}
			require.NoError(t, a.Init())

			err = a.Execute(context.Background())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, "/api2/json/admin/prune/goback-prune/run", tt.server.runPath)
			assert.Equal(t, "PBSAPIToken=goback@pbs!maint:secret", tt.server.auth)
		})
	}
}

func TestGarbageCollectPBSExecute(t *testing.T) {
	tests := []struct {
		name    string
		server  *fakePBS
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "gc finishes ok",
			server:  &fakePBS{runStatus: http.StatusOK, taskExit: "OK"},
			timeout: testTimeout,
		},
		{
			name:    "gc already running",
			server:  &fakePBS{runStatus: http.StatusBadRequest, runBody: `{"data":null,"message":"Garbage collection already running"}`},
			timeout: testTimeout,
			wantErr: `failed to start garbage collection on tank: unexpected status code: 400 (Bad Request: {"data":null,"message":"Garbage collection already running"})`,
		},
		{
			name:    "gc does not finish in time",
			server:  &fakePBS{runStatus: http.StatusOK, taskRunning: true},
			timeout: testShortTimeout,
			wantErr: "garbage collection on tank did not finish within 1ns; task " + testGCUPID + " keeps running on PBS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.server.upid = testGCUPID
			ts := httptest.NewServer(tt.server)
			defer ts.Close()

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			client, err := pbsclient.New(ts.URL, pbsclient.WithToken("goback@pbs!maint:secret"), pbsclient.WithLogger(logger))
			require.NoError(t, err)

			a := &GarbageCollectPBS{
				PBSClient:   client,
				Logger:      logger,
				StatusLine:  activity.NewStatusLine(workflow.ActivityID{Module: "maintenance", Type: "GarbageCollectPBS"}, logger, nil),
				Registry:    newTestRegistry(t),
				Token:       "goback@pbs!maint:secret",
				Datastore:   "tank",
				TaskTimeout: tt.timeout,
			}
			require.NoError(t, a.Init())

			err = a.Execute(context.Background())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, "/api2/json/admin/datastore/tank/gc", tt.server.runPath)
		})
	}
}

// Test helper types

// fakePBS is a minimal PBS API: one endpoint that starts a task (any POST) and the
// task status endpoint. It records the POST path and Authorization header.
type fakePBS struct {
	upid        string
	runStatus   int    // HTTP status for the start request
	runBody     string // optional body for a non-200 start response
	taskExit    string // exit status reported once the task has stopped
	taskRunning bool   // report the task as still running on every status call

	runPath string
	auth    string
}

func (f *fakePBS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		f.runPath = r.URL.Path
		f.auth = r.Header.Get("Authorization")
		w.WriteHeader(f.runStatus)
		if f.runStatus != http.StatusOK {
			w.Write([]byte(f.runBody))
			return
		}
		w.Write([]byte(`{"data":"` + f.upid + `"}`))
	case http.MethodGet:
		if f.taskRunning {
			w.Write([]byte(`{"data":{"upid":"` + f.upid + `","status":"running"}}`))
			return
		}
		w.Write([]byte(`{"data":{"upid":"` + f.upid + `","status":"stopped","exitstatus":"` + f.taskExit + `"}}`))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// newTestRegistry returns a fresh metrics registry so each test can register the
// activity's metrics without colliding with another test's.
func newTestRegistry(t *testing.T) metrics.Registry {
	t.Helper()
	registry, err := metrics.NewScrapeRegistry()
	require.NoError(t, err)
	return registry
}
