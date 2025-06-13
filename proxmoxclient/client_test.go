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

	client := New(ts.URL)
	resp, err := client.Version()
	if err != nil {
		require.NoError(t, err, "Version failed")
	}
	assert.Equal(t, `{"data":{"version":"7.1-10","release":"2021-11-23"}}`, resp)
}

func TestListVMs(t *testing.T) {
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

	client := New(ts.URL)
	ctx := context.Background()
	vms, err := client.ListVMs(ctx)

	require.NoError(t, err)
	require.Len(t, vms, 2)

	// Verify first VM
	assert.Equal(t, 100, vms[0].VMID)
	assert.Equal(t, "web-server", vms[0].Name)
	assert.Equal(t, "pve-node1", vms[0].Node)
	assert.Equal(t, "running", vms[0].Status)
	assert.Equal(t, 0, vms[0].Template)
	assert.Equal(t, "qemu", vms[0].Type)
	assert.Equal(t, int64(2147483648), vms[0].MaxMem)
	assert.Equal(t, int64(34359738368), vms[0].MaxDisk)
	assert.Equal(t, 0.123, vms[0].CPU)
	assert.Equal(t, int64(1073741824), vms[0].Mem)
	assert.Equal(t, int64(12345), vms[0].Uptime)

	// Verify second VM
	assert.Equal(t, 101, vms[1].VMID)
	assert.Equal(t, "database", vms[1].Name)
	assert.Equal(t, "pve-node2", vms[1].Node)
	assert.Equal(t, "stopped", vms[1].Status)
	assert.Equal(t, "qemu", vms[1].Type)
}

func TestListLXCs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/cluster/resources", r.URL.Path)
		assert.Equal(t, "lxc", r.URL.Query().Get("type"))
		assert.Equal(t, http.MethodGet, r.Method)

		response := `{
			"data": [
				{
					"vmid": 200,
					"name": "proxy-container",
					"node": "pve-node1",
					"status": "running",
					"template": 0,
					"type": "lxc",
					"maxmem": 1073741824,
					"maxdisk": 8589934592,
					"cpu": 0.05,
					"mem": 536870912,
					"uptime": 5432
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	lxcs, err := client.ListLXCs(ctx)

	require.NoError(t, err)
	require.Len(t, lxcs, 1)

	// Verify LXC container
	assert.Equal(t, 200, lxcs[0].VMID)
	assert.Equal(t, "proxy-container", lxcs[0].Name)
	assert.Equal(t, "pve-node1", lxcs[0].Node)
	assert.Equal(t, "running", lxcs[0].Status)
	assert.Equal(t, 0, lxcs[0].Template)
	assert.Equal(t, "lxc", lxcs[0].Type)
	assert.Equal(t, int64(1073741824), lxcs[0].MaxMem)
	assert.Equal(t, int64(8589934592), lxcs[0].MaxDisk)
	assert.Equal(t, 0.05, lxcs[0].CPU)
	assert.Equal(t, int64(536870912), lxcs[0].Mem)
	assert.Equal(t, int64(5432), lxcs[0].Uptime)
}

func TestListVMs_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	vms, err := client.ListVMs(ctx)

	require.NoError(t, err)
	assert.Len(t, vms, 0)
}

func TestListVMs_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	vms, err := client.ListVMs(ctx)

	assert.Error(t, err)
	assert.Nil(t, vms)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestListVMs_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	vms, err := client.ListVMs(ctx)

	assert.Error(t, err)
	assert.Nil(t, vms)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestListVMs_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	vms, err := client.ListVMs(ctx)

	assert.Error(t, err)
	assert.Nil(t, vms)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestListVMs_PartiallyInvalidData(t *testing.T) {
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
					"type": "qemu"
				},
				{
					"invalid": "entry"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	vms, err := client.ListVMs(ctx)

	// Should succeed and return only the valid VM
	require.NoError(t, err)
	require.Len(t, vms, 1)
	assert.Equal(t, 100, vms[0].VMID)
	assert.Equal(t, "web-server", vms[0].Name)
}

func TestListLXCs_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	lxcs, err := client.ListLXCs(ctx)

	assert.Error(t, err)
	assert.Nil(t, lxcs)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestListLXCs_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	lxcs, err := client.ListLXCs(ctx)

	assert.Error(t, err)
	assert.Nil(t, lxcs)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestListAllResources(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/cluster/resources", r.URL.Path)
		assert.Equal(t, "", r.URL.Query().Get("type")) // No type filter
		assert.Equal(t, http.MethodGet, r.Method)

		// Mixed response with VMs, LXCs, and other resource types
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
					"vmid": 200,
					"name": "proxy-container",
					"node": "pve-node1",
					"status": "running",
					"template": 0,
					"type": "lxc",
					"maxmem": 1073741824,
					"maxdisk": 8589934592,
					"cpu": 0.05,
					"mem": 536870912,
					"uptime": 5432
				},
				{
					"node": "pve-node1",
					"type": "node",
					"status": "online"
				},
				{
					"storage": "local",
					"type": "storage",
					"node": "pve-node1"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	resources, err := client.ListAllResources(ctx)

	require.NoError(t, err)
	// Should only return VM and LXC resources, not node or storage
	require.Len(t, resources, 2)

	// Verify VM resource
	assert.Equal(t, 100, resources[0].VMID)
	assert.Equal(t, "web-server", resources[0].Name)
	assert.Equal(t, "qemu", resources[0].Type)
	assert.Equal(t, "running", resources[0].Status)

	// Verify LXC resource
	assert.Equal(t, 200, resources[1].VMID)
	assert.Equal(t, "proxy-container", resources[1].Name)
	assert.Equal(t, "lxc", resources[1].Type)
	assert.Equal(t, "running", resources[1].Status)
}

func TestListAllResources_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	client := New(ts.URL)
	ctx := context.Background()
	resources, err := client.ListAllResources(ctx)

	require.NoError(t, err)
	assert.Len(t, resources, 0)
}
