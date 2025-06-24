package proxmoxclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":{"version":"7.1-10","release":"2021-11-23"}}`))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	resp, err := client.Version()
	if err != nil {
		require.NoError(t, err, "Version failed")
	}
	assert.Equal(t, `{"data":{"version":"7.1-10","release":"2021-11-23"}}`, resp)
}

func TestListComputeResources(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/cluster/resources", r.URL.Path)
		assert.Equal(t, "vm", r.URL.Query().Get("type"))
		assert.Equal(t, http.MethodGet, r.Method)

		response := `{
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
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	resources, err := client.ListComputeResources(ctx)

	require.NoError(t, err)
	require.Len(t, resources, 2)

	// Verify first VM
	assert.Equal(t, VMID(100), resources[0].VMID)
	assert.Equal(t, "web-server", resources[0].Name)
	assert.Equal(t, "pve-node1", resources[0].Node)
	assert.Equal(t, "running", resources[0].Status)
	assert.Equal(t, 0, resources[0].Template)
	assert.Equal(t, "qemu", resources[0].Type)
	assert.Equal(t, int64(2147483648), resources[0].MaxMem)
	assert.Equal(t, int64(34359738368), resources[0].MaxDisk)
	assert.Equal(t, 0.123, resources[0].CPU)
	assert.Equal(t, int64(1073741824), resources[0].Mem)
	assert.Equal(t, int64(12345), resources[0].Uptime)

	// Verify second VM
	assert.Equal(t, VMID(101), resources[1].VMID)
	assert.Equal(t, "database", resources[1].Name)
	assert.Equal(t, "pve-node2", resources[1].Node)
	assert.Equal(t, "stopped", resources[1].Status)
	assert.Equal(t, "qemu", resources[1].Type)
}

func TestListComputeResources_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	resources, err := client.ListComputeResources(ctx)

	require.NoError(t, err)
	assert.Len(t, resources, 0)
}

func TestListComputeResources_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	resources, err := client.ListComputeResources(ctx)

	assert.Error(t, err)
	assert.Nil(t, resources)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestListComputeResources_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	resources, err := client.ListComputeResources(ctx)

	assert.Error(t, err)
	assert.Nil(t, resources)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestListComputeResources_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resources, err := client.ListComputeResources(ctx)

	assert.Error(t, err)
	assert.Nil(t, resources)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestListComputeResources_PartiallyInvalidData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response with one valid VM and one invalid entry
		response := `{
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
				"invalid json string"
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	resources, err := client.ListComputeResources(ctx)

	// Should succeed and return only the valid VM
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, VMID(100), resources[0].VMID)
	assert.Equal(t, "web-server", resources[0].Name)
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
			host:     "https://pve.example.com:8006",
			path:     "/api2/json/version",
			expected: "https://pve.example.com:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "host with trailing slash",
			host:     "https://pve.example.com:8006/",
			path:     "/api2/json/version",
			expected: "https://pve.example.com:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "host with path and trailing slash",
			host:     "https://pve.example.com:8006/proxmox/",
			path:     "/api2/json/version",
			expected: "https://pve.example.com:8006/api2/json/version",
			wantErr:  false,
		},
		{
			name:     "path with query parameters",
			host:     "https://pve.example.com:8006",
			path:     "/api2/json/cluster/resources?type=vm",
			expected: "https://pve.example.com:8006/api2/json/cluster/resources?type=vm",
			wantErr:  false,
		},
		{
			name:     "invalid host URL",
			host:     "://invalid-url",
			path:     "/api2/json/version",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.host)
			if tt.wantErr {
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

func TestBackup_Basic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/nodes/pve2/vzdump", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "100", r.URL.Query().Get("vmid"))
		assert.Equal(t, "pbs", r.URL.Query().Get("storage"))
		assert.Equal(t, "", r.URL.Query().Get("mode"))
		assert.Equal(t, "", r.URL.Query().Get("compress"))

		response := `{"data":"UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890"}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs")

	require.NoError(t, err)
	assert.Equal(t, "UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890", string(taskID))
}

func TestBackup_WithMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/nodes/pve2/vzdump", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "100", r.URL.Query().Get("vmid"))
		assert.Equal(t, "pbs", r.URL.Query().Get("storage"))
		assert.Equal(t, "snapshot", r.URL.Query().Get("mode"))
		assert.Equal(t, "", r.URL.Query().Get("compress"))

		response := `{"data":"UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890"}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs", WithMode("snapshot"))

	require.NoError(t, err)
	assert.Equal(t, "UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890", string(taskID))
}

func TestBackup_WithCompress(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/nodes/pve2/vzdump", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "100", r.URL.Query().Get("vmid"))
		assert.Equal(t, "pbs", r.URL.Query().Get("storage"))
		assert.Equal(t, "", r.URL.Query().Get("mode"))
		assert.Equal(t, "1", r.URL.Query().Get("compress"))

		response := `{"data":"UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890"}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs", WithCompress("1"))

	require.NoError(t, err)
	assert.Equal(t, "UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890", string(taskID))
}

func TestBackup_WithModeAndCompress(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/nodes/pve2/vzdump", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "100", r.URL.Query().Get("vmid"))
		assert.Equal(t, "pbs", r.URL.Query().Get("storage"))
		assert.Equal(t, "suspend", r.URL.Query().Get("mode"))
		assert.Equal(t, "zstd", r.URL.Query().Get("compress"))

		response := `{"data":"UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890"}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs", WithMode("suspend"), WithCompress("zstd"))

	require.NoError(t, err)
	assert.Equal(t, "UPID:pve2:00000001:00000002:12345678:vzdump:100:user@host:1234567890", string(taskID))
}

func TestBackup_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs")

	assert.Error(t, err)
	assert.Empty(t, taskID)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestBackup_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer ts.Close()

	client, err := New(ts.URL)
	require.NoError(t, err)
	ctx := context.Background()
	taskID, err := client.Backup(ctx, "pve2", 100, "pbs")

	assert.Error(t, err)
	assert.Empty(t, taskID)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}
