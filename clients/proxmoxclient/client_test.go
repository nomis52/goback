package proxmoxclient

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		status         int
		wantErr        string
		verifyFn       func(t *testing.T, resp string)
	}{
		{
			name:           "success",
			serverResponse: `{"data":{"version":"7.1-10","release":"2021-11-23"}}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, resp string) {
				assert.Equal(t, `{"data":{"version":"7.1-10","release":"2021-11-23"}}`, resp)
			},
		},
		{
			name:           "http error",
			serverResponse: "internal server error",
			status:         http.StatusInternalServerError,
			wantErr:        "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/version", r.URL.Path)
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.serverResponse))
			}))
			defer ts.Close()

			client, err := New(ts.URL)
			require.NoError(t, err)
			resp, err := client.Version()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.verifyFn != nil {
					tt.verifyFn(t, resp)
				}
			}
		})
	}
}

func TestListComputeResources(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		status         int
		wantErr        string
		verifyFn       func(t *testing.T, resources []Resource)
	}{
		{
			name: "success",
			serverResponse: `{
				"data": [
					{
						"vmid": 100,
						"name": "web-server",
						"node": "pve-node1",
						"status": "running",
						"template": 0,
						"type": "qemu",
						"maxmem": 2147483648,
						"maxdisk": 34359738368,
						"cpu": 0.123,
						"mem": 1073741824,
						"uptime": 12345
					},
					{
						"vmid": 101,
						"name": "database",
						"node": "pve-node2",
						"status": "stopped",
						"template": 0,
						"type": "qemu",
						"maxmem": 4294967296,
						"maxdisk": 68719476736,
						"cpu": 0.0,
						"mem": 0,
						"uptime": 0
					}
				]
			}`,
			status: http.StatusOK,
			verifyFn: func(t *testing.T, resources []Resource) {
				require.Len(t, resources, 2)
				assert.Equal(t, VMID(100), resources[0].VMID)
				assert.Equal(t, "web-server", resources[0].Name)
			},
		},
		{
			name:           "empty response",
			serverResponse: `{"data": []}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, resources []Resource) {
				assert.Empty(t, resources)
			},
		},
		{
			name:           "invalid json",
			serverResponse: `invalid json`,
			status:         http.StatusOK,
			wantErr:        "failed to unmarshal response",
		},
		{
			name:           "http error",
			serverResponse: "",
			status:         http.StatusInternalServerError,
			wantErr:        "unexpected status code: 500",
		},
		{
			name:           "partially invalid data",
			serverResponse: `{"data": [{"vmid": 100, "name": "web-server"}, "invalid"]}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, resources []Resource) {
				assert.Len(t, resources, 1)
				assert.Equal(t, VMID(100), resources[0].VMID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/cluster/resources", r.URL.Path)
				assert.Equal(t, "vm", r.URL.Query().Get("type"))

				w.WriteHeader(tt.status)
				w.Write([]byte(tt.serverResponse))
			}))
			defer ts.Close()

			client, err := New(ts.URL)
			require.NoError(t, err)

			resources, err := client.ListComputeResources(context.Background())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.verifyFn != nil {
					tt.verifyFn(t, resources)
				}
			}
		})
	}
}

func TestListBackups(t *testing.T) {
	tests := []struct {
		name           string
		node           string
		storage        string
		serverResponse string
		status         int
		wantErr        string
		verifyFn       func(t *testing.T, backups []Backup)
	}{
		{
			name:    "success",
			node:    "pve2",
			storage: "pbs",
			serverResponse: `{
				"data": [
					{
						"content": "backup",
						"ctime": 1640995200,
						"format": "pbs-vm",
						"size": 1073741824,
						"volid": "pbs:backup/vm/100/2022-01-01T00:00:00Z",
						"vmid": 100
					}
				]
			}`,
			status: http.StatusOK,
			verifyFn: func(t *testing.T, backups []Backup) {
				assert.Len(t, backups, 1)
			},
		},
		{
			name:           "http error",
			node:           "pve2",
			storage:        "pbs",
			serverResponse: "",
			status:         http.StatusInternalServerError,
			wantErr:        "unexpected status code: 500",
		},
		{
			name:           "invalid json",
			node:           "pve2",
			storage:        "pbs",
			serverResponse: "invalid json",
			status:         http.StatusOK,
			wantErr:        "failed to unmarshal response",
		},
		{
			name:           "partially invalid data",
			node:           "pve2",
			storage:        "pbs",
			serverResponse: `{"data": [{"volid": "valid"}, "invalid"]}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, backups []Backup) {
				assert.Len(t, backups, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api2/json/nodes/" + tt.node + "/storage/" + tt.storage + "/content"
				assert.Equal(t, expectedPath, r.URL.Path)
				assert.Equal(t, "backup", r.URL.Query().Get("content"))

				w.WriteHeader(tt.status)
				w.Write([]byte(tt.serverResponse))
			}))
			defer ts.Close()

			client, err := New(ts.URL)
			require.NoError(t, err)

			backups, err := client.ListBackups(context.Background(), tt.node, tt.storage)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.verifyFn != nil {
					tt.verifyFn(t, backups)
				}
			}
		})
	}
}

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		name           string
		node           string
		taskID         TaskID
		serverResponse string
		status         int
		wantErr        string
		verifyFn       func(t *testing.T, status *TaskStatus)
	}{
		{
			name:           "success",
			node:           "pve2",
			taskID:         "UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890",
			serverResponse: `{"data": {"upid": "UPID:...", "status": "stopped", "exitstatus": "OK"}}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, status *TaskStatus) {
				require.NotNil(t, status)
				assert.Equal(t, "stopped", status.Status)
			},
		},
		{
			name:           "http error",
			node:           "pve2",
			taskID:         "UPID:...",
			serverResponse: "",
			status:         http.StatusNotFound,
			wantErr:        "unexpected status code: 404",
		},
		{
			name:           "invalid json",
			node:           "pve2",
			taskID:         "UPID:...",
			serverResponse: "invalid",
			status:         http.StatusOK,
			wantErr:        "failed to unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api2/json/nodes/" + tt.node + "/tasks/" + string(tt.taskID) + "/status"
				assert.Equal(t, expectedPath, r.URL.Path)

				w.WriteHeader(tt.status)
				w.Write([]byte(tt.serverResponse))
			}))
			defer ts.Close()

			client, err := New(ts.URL)
			require.NoError(t, err)

			status, err := client.TaskStatus(context.Background(), tt.node, tt.taskID)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.verifyFn != nil {
					tt.verifyFn(t, status)
				}
			}
		})
	}
}

func TestNewAndOptions(t *testing.T) {
	t.Run("New with valid URL", func(t *testing.T) {
		client, err := New("https://pve.test")
		require.NoError(t, err)
		assert.Equal(t, "https://pve.test", client.baseURL.String())
	})

	t.Run("New with invalid URL", func(t *testing.T) {
		client, err := New("::invalid")
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("WithToken", func(t *testing.T) {
		token := "user@pve!token=uuid"
		client, err := New("https://pve.test", WithToken(token))
		require.NoError(t, err)
		assert.Equal(t, token, client.token)

		// Verify it's used in request
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PVEAPIToken="+token, r.Header.Get("Authorization"))
			w.Write([]byte(`{"data":{"version":"1.0"}}`))
		}))
		defer ts.Close()

		client, _ = New(ts.URL, WithToken(token))
		_, err = client.Version()
		require.NoError(t, err)
	})

	t.Run("WithLogger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		client, err := New("https://pve.test", WithLogger(logger))
		require.NoError(t, err)
		assert.Equal(t, logger, client.logger)
	})
}

func TestBackup_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"content": "backup",
		"ctime": 1640995200,
		"volid": "pbs:backup/vm/100/2022-01-01T00:00:00Z",
		"vmid": 100
	}`

	var b Backup
	err := json.Unmarshal([]byte(jsonData), &b)
	require.NoError(t, err)

	assert.Equal(t, "backup", b.Content)
	assert.Equal(t, VMID(100), b.VMID)
	assert.Equal(t, int64(1640995200), b.CTime.Unix())
	assert.Equal(t, "2022-01-01T00:00:00Z", b.CTime.UTC().Format(time.RFC3339))
}

func TestDoRequest_Errors(t *testing.T) {
	t.Run("invalid method", func(t *testing.T) {
		client, _ := New("https://pve.test")
		// Use a method that is invalid for http.NewRequestWithContext
		resp, err := client.doRequest(context.Background(), " ", "/api2/json/version")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("invalid path", func(t *testing.T) {
		client, _ := New("https://pve.test")
		resp, err := client.doRequest(context.Background(), "GET", ":%gh")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to build URL")
	})
}

func TestHost(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://pve2.test:8006", "pve2"},
		{"https://pve-node1:8006", "pve-node1"},
		{"https://192.168.1.100:8006", "192"},
		{"http://localhost", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			client, _ := New(tt.url)
			assert.Equal(t, tt.expected, client.Host())
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "host without trailing slash",
			host:     "https://pve.test:8006",
			path:     "/api2/json/version",
			expected: "https://pve.test:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "host with trailing slash",
			host:     "https://pve.test:8006/",
			path:     "/api2/json/version",
			expected: "https://pve.test:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "host with path and trailing slash",
			host:     "https://pve.test:8006/proxmox/",
			path:     "/api2/json/version",
			expected: "https://pve.test:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "path with query parameters",
			host:     "https://pve.test:8006",
			path:     "/api2/json/cluster/resources?type=vm",
			expected: "https://pve.test:8006/api2/json/cluster/resources?type=vm",
			wantErr:  false,
		},
		{
			name:     "invalid host URL",
			host:     "://invalid-url",
			path:     "/api2/json/version",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid path",
			host:     "https://pve.test",
			path:     ":%gh", // invalid escape sequence
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.host)
			if tt.name == "invalid host URL" {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			result, err := client.buildURL(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBackup(t *testing.T) {
	tests := []struct {
		name           string
		node           string
		vmid           VMID
		storage        string
		opts           []BackupOption
		expectedParams map[string]string
		serverResponse string
		status         int
		wantErr        string
		verifyFn       func(t *testing.T, taskID TaskID)
	}{
		{
			name:    "basic success",
			node:    "pve2",
			vmid:    100,
			storage: "pbs",
			expectedParams: map[string]string{
				"vmid":    "100",
				"storage": "pbs",
			},
			serverResponse: `{"data":"UPID:..."}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, taskID TaskID) {
				assert.NotEmpty(t, taskID)
			},
		},
		{
			name:    "with options",
			node:    "pve2",
			vmid:    101,
			storage: "local",
			opts: []BackupOption{
				WithMode("snapshot"),
				WithCompress("zstd"),
				WithMailNotification("always"),
			},
			expectedParams: map[string]string{
				"vmid":             "101",
				"storage":          "local",
				"mode":             "snapshot",
				"compress":         "zstd",
				"mailnotification": "always",
			},
			serverResponse: `{"data":"UPID:..."}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, taskID TaskID) {
				assert.NotEmpty(t, taskID)
			},
		},
		{
			name:    "WithCompress as first option",
			node:    "pve2",
			vmid:    100,
			storage: "pbs",
			opts: []BackupOption{
				WithCompress("gzip"),
			},
			expectedParams: map[string]string{
				"vmid":     "100",
				"storage":  "pbs",
				"compress": "gzip",
			},
			serverResponse: `{"data":"UPID:..."}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, taskID TaskID) {
				assert.NotEmpty(t, taskID)
			},
		},
		{
			name:    "WithMailNotification as first option",
			node:    "pve2",
			vmid:    100,
			storage: "pbs",
			opts: []BackupOption{
				WithMailNotification("failure"),
			},
			expectedParams: map[string]string{
				"vmid":             "100",
				"storage":          "pbs",
				"mailnotification": "failure",
			},
			serverResponse: `{"data":"UPID:..."}`,
			status:         http.StatusOK,
			verifyFn: func(t *testing.T, taskID TaskID) {
				assert.NotEmpty(t, taskID)
			},
		},
		{
			name:           "http error",
			node:           "pve2",
			vmid:           100,
			storage:        "pbs",
			serverResponse: "",
			status:         http.StatusInternalServerError,
			wantErr:        "unexpected status code: 500",
		},
		{
			name:           "invalid json",
			node:           "pve2",
			vmid:           100,
			storage:        "pbs",
			serverResponse: "invalid",
			status:         http.StatusOK,
			wantErr:        "failed to unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/nodes/"+tt.node+"/vzdump", r.URL.Path)
				assert.Equal(t, http.MethodPost, r.Method)

				for k, v := range tt.expectedParams {
					assert.Equal(t, v, r.URL.Query().Get(k), "Parameter %s", k)
				}

				w.WriteHeader(tt.status)
				w.Write([]byte(tt.serverResponse))
			}))
			defer ts.Close()

			client, err := New(ts.URL)
			require.NoError(t, err)

			taskID, err := client.Backup(context.Background(), tt.node, tt.vmid, tt.storage, tt.opts...)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.verifyFn != nil {
					tt.verifyFn(t, taskID)
				}
			}
		})
	}
}
