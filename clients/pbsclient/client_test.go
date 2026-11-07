package pbsclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name    string
		host    string
		opts    []Option
		wantErr string
	}{
		{
			name: "valid https host",
			host: "https://pbs.example.com",
			opts: []Option{WithLogger(logger)},
		},
		{
			name: "valid http host",
			host: "http://192.168.1.100:8007",
			// Should use default logger
		},
		{
			name:    "missing scheme",
			host:    "pbs.example.com",
			wantErr: "host URL must include scheme (http:// or https://): pbs.example.com",
		},
		{
			name:    "invalid url",
			host:    "http://:invalid",
			wantErr: "invalid host URL: parse \"http://:invalid\": invalid port \":invalid\" after host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.host, tt.opts...)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.host, client.Host)
				assert.NotNil(t, client.Logger)
			}
		})
	}
}

func TestPing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name         string
		handler      http.HandlerFunc
		modifyClient func(*Client)
		wantResp     string
		wantErr      string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/ping", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":"pong"}`))
			},
			wantResp: `{"data":"pong"}`,
		},
		{
			name: "non-200 status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr: "PBS server returned status 401",
		},
		{
			name:    "connection error",
			handler: func(w http.ResponseWriter, r *http.Request) {},
			modifyClient: func(c *Client) {
				c.Host = "http://localhost:1" // Likely to fail
			},
			wantErr: "failed to ping PBS server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			if tt.modifyClient != nil {
				tt.modifyClient(client)
			}

			resp, err := client.Ping()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, resp)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestPingReadError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, _ := New("http://example.test", WithLogger(logger))

	// Mocking the http.Client to return a response with a faulty body
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &errorReader{},
			}, nil
		}),
	}

	resp, err := client.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read response body: read error")
	assert.Empty(t, resp)
}

func TestWithToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name     string
		opts     []Option
		wantAuth string
	}{
		{
			name:     "token sent as PBSAPIToken header",
			opts:     []Option{WithToken("goback@pbs!maint:secret")},
			wantAuth: "PBSAPIToken=goback@pbs!maint:secret",
		},
		{
			name:     "no token sends no Authorization header",
			wantAuth: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Write([]byte(`{"data":"UPID:pbs:1:2:3:4:prunejob:x:root@pam:"}`))
			}))
			defer ts.Close()

			client, err := New(ts.URL, append(tt.opts, WithLogger(logger))...)
			require.NoError(t, err)

			_, err = client.RunPruneJob(context.Background(), "x")
			require.NoError(t, err)
			assert.Equal(t, tt.wantAuth, gotAuth)
		})
	}
}

func TestRunPruneJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name     string
		jobID    string
		handler  http.HandlerFunc
		wantTask TaskID
		wantErr  string
	}{
		{
			name:  "success",
			jobID: "goback-prune",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api2/json/admin/prune/goback-prune/run", r.URL.Path)
				w.Write([]byte(`{"data":"UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:"}`))
			},
			wantTask: "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:",
		},
		{
			name:  "unknown job",
			jobID: "missing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"data":null,"message":"no such prune job 'missing'"}`))
			},
			wantErr: `unexpected status code: 400 (Bad Request: {"data":null,"message":"no such prune job 'missing'"})`,
		},
		{
			name:  "permission denied",
			jobID: "goback-prune",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr: "unexpected status code: 403 (Forbidden)",
		},
		{
			name:  "empty task id",
			jobID: "goback-prune",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"data":null}`))
			},
			wantErr: "PBS did not return a task ID",
		},
		{
			name:  "malformed response",
			jobID: "goback-prune",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`not json`))
			},
			wantErr: "failed to unmarshal response: invalid character 'o' in literal null (expecting 'u')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			task, err := client.RunPruneJob(context.Background(), tt.jobID)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Empty(t, task)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTask, task)
			}
		})
	}
}

func TestRunPruneJobConnectionError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := New("http://localhost:1", WithLogger(logger))
	require.NoError(t, err)

	task, err := client.RunPruneJob(context.Background(), "goback-prune")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
	assert.Empty(t, task)
}

func TestStartGarbageCollection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		datastore string
		handler   http.HandlerFunc
		wantTask  TaskID
		wantErr   string
	}{
		{
			name:      "success",
			datastore: "tank",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api2/json/admin/datastore/tank/gc", r.URL.Path)
				w.Write([]byte(`{"data":"UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:garbage_collection:tank:root@pam:"}`))
			},
			wantTask: "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:garbage_collection:tank:root@pam:",
		},
		{
			name:      "already running",
			datastore: "tank",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"data":null,"message":"Garbage collection already running"}`))
			},
			wantErr: `unexpected status code: 400 (Bad Request: {"data":null,"message":"Garbage collection already running"})`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			task, err := client.StartGarbageCollection(context.Background(), tt.datastore)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Empty(t, task)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTask, task)
			}
		})
	}
}

func TestTaskStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const upid = "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:"

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus *TaskStatus
		wantErr    string
	}{
		{
			name: "running",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api2/json/nodes/localhost/tasks/"+upid+"/status", r.URL.Path)
				w.Write([]byte(`{"data":{"upid":"` + upid + `","status":"running","starttime":1761800000,"worker_type":"prunejob","worker_id":"goback-prune","user":"root@pam","node":"pbs","pid":4660,"pstart":43981}}`))
			},
			wantStatus: &TaskStatus{
				UPID:       upid,
				Status:     "running",
				StartTime:  1761800000,
				WorkerType: "prunejob",
				WorkerID:   "goback-prune",
				User:       "root@pam",
			},
		},
		{
			name: "stopped ok",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"data":{"upid":"` + upid + `","status":"stopped","exitstatus":"OK","starttime":1761800000,"endtime":1761800300,"worker_type":"prunejob","worker_id":"goback-prune","user":"root@pam"}}`))
			},
			wantStatus: &TaskStatus{
				UPID:       upid,
				Status:     "stopped",
				ExitStatus: "OK",
				StartTime:  1761800000,
				EndTime:    1761800300,
				WorkerType: "prunejob",
				WorkerID:   "goback-prune",
				User:       "root@pam",
			},
		},
		{
			name: "unknown task",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"data":null,"message":"unable to parse UPID"}`))
			},
			wantErr: `unexpected status code: 400 (Bad Request: {"data":null,"message":"unable to parse UPID"})`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			status, err := client.TaskStatus(context.Background(), upid)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Nil(t, status)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatus, status)
			}
		})
	}
}

func TestWaitForTask(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const upid = "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:"
	const pollInterval = time.Millisecond

	tests := []struct {
		name      string
		handler   func(calls int32) string // response body for the Nth status call (1-based)
		cancelCtx bool
		wantCalls int32
		wantExit  string
		wantErr   string
	}{
		{
			name: "polls until stopped",
			handler: func(calls int32) string {
				if calls < 3 {
					return `{"data":{"upid":"` + upid + `","status":"running"}}`
				}
				return `{"data":{"upid":"` + upid + `","status":"stopped","exitstatus":"OK"}}`
			},
			wantCalls: 3,
			wantExit:  "OK",
		},
		{
			name: "returns a failed task without error",
			handler: func(calls int32) string {
				return `{"data":{"upid":"` + upid + `","status":"stopped","exitstatus":"unable to acquire lock"}}`
			},
			wantCalls: 1,
			wantExit:  "unable to acquire lock",
		},
		{
			name: "context cancelled while running",
			handler: func(calls int32) string {
				return `{"data":{"upid":"` + upid + `","status":"running"}}`
			},
			cancelCtx: true,
			wantCalls: 0,
			wantErr:   "waiting for task " + upid + ": context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tt.handler(calls.Add(1))))
			}))
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelCtx {
				cancel()
			}

			status, err := client.WaitForTask(ctx, upid, pollInterval)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Nil(t, status)
			} else {
				require.NoError(t, err)
				assert.True(t, status.Finished())
				assert.Equal(t, tt.wantExit, status.ExitStatus)
			}
			assert.Equal(t, tt.wantCalls, calls.Load())
		})
	}
}

func TestTaskStatusPredicates(t *testing.T) {
	tests := []struct {
		name          string
		status        TaskStatus
		wantFinished  bool
		wantSucceeded bool
		wantWarnings  bool
	}{
		{
			name:   "running",
			status: TaskStatus{Status: "running"},
		},
		{
			name:          "stopped ok",
			status:        TaskStatus{Status: "stopped", ExitStatus: "OK"},
			wantFinished:  true,
			wantSucceeded: true,
		},
		{
			name:          "stopped with warnings",
			status:        TaskStatus{Status: "stopped", ExitStatus: "WARNINGS: 2"},
			wantFinished:  true,
			wantSucceeded: true,
			wantWarnings:  true,
		},
		{
			name:         "stopped with error",
			status:       TaskStatus{Status: "stopped", ExitStatus: "unable to acquire lock"},
			wantFinished: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantFinished, tt.status.Finished())
			assert.Equal(t, tt.wantSucceeded, tt.status.Succeeded())
			assert.Equal(t, tt.wantWarnings, tt.status.HasWarnings())
		})
	}
}

// Test helper types

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorReader) Close() error {
	return nil
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
