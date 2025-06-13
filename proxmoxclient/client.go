// Package proxmoxclient provides a simple client for interacting with a Proxmox VE server via its HTTP API.
//
// Example usage:
//
//	client := proxmoxclient.New("https://proxmox.example.com")
//	version, err := client.Version()
//	vms, err := client.ListVMs(ctx)
package proxmoxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// VM represents a virtual machine or container in Proxmox.
type VM struct {
	VMID     int    `json:"vmid"`
	Name     string `json:"name"`
	Node     string `json:"node"`
	Status   string `json:"status"`
	Template int    `json:"template"`
	Type     string `json:"type"`
	MaxMem   int64  `json:"maxmem"`
	MaxDisk  int64  `json:"maxdisk"`
	CPU      float64 `json:"cpu"`
	Mem      int64  `json:"mem"`
	Uptime   int64  `json:"uptime"`
}

// LXC represents an LXC container in Proxmox.
type LXC struct {
	VMID     int    `json:"vmid"`
	Name     string `json:"name"`
	Node     string `json:"node"`
	Status   string `json:"status"`
	Template int    `json:"template"`
	Type     string `json:"type"`
	MaxMem   int64  `json:"maxmem"`
	MaxDisk  int64  `json:"maxdisk"`
	CPU      float64 `json:"cpu"`
	Mem      int64  `json:"mem"`
	Uptime   int64  `json:"uptime"`
}

// clusterResourcesResponse represents the response from /api2/json/cluster/resources
type clusterResourcesResponse struct {
	Data []json.RawMessage `json:"data"`
}

// Client represents a Proxmox VE API client.
// Use New() to create a new client for a given Proxmox host.
type Client struct {
	Host string
}

// New creates a new Client for the given Proxmox VE host.
// The host should include the scheme (e.g., "https://proxmox.example.com").
func New(host string) *Client {
	return &Client{Host: host}
}

// Version retrieves the Proxmox VE version information by calling the /api2/json/version endpoint.
// It returns the raw response body as a string, or an error if the request fails.
func (c *Client) Version() (string, error) {
	url := fmt.Sprintf("%s/api2/json/version", c.Host)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ListVMs retrieves all VMs across the entire Proxmox cluster.
// It queries the /api2/json/cluster/resources?type=vm endpoint to get cluster-wide VM information.
func (c *Client) ListVMs(ctx context.Context) ([]VM, error) {
	url := fmt.Sprintf("%s/api2/json/cluster/resources?type=vm", c.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response clusterResourcesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	vms := make([]VM, 0, len(response.Data))
	for _, rawVM := range response.Data {
		var vm VM
		if err := json.Unmarshal(rawVM, &vm); err != nil {
			// Skip invalid entries rather than failing completely
			continue
		}
		vms = append(vms, vm)
	}

	return vms, nil
}

// ListLXCs retrieves all LXC containers across the entire Proxmox cluster.
// It queries the /api2/json/cluster/resources?type=lxc endpoint to get cluster-wide LXC information.
func (c *Client) ListLXCs(ctx context.Context) ([]LXC, error) {
	url := fmt.Sprintf("%s/api2/json/cluster/resources?type=lxc", c.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response clusterResourcesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	lxcs := make([]LXC, 0, len(response.Data))
	for _, rawLXC := range response.Data {
		var lxc LXC
		if err := json.Unmarshal(rawLXC, &lxc); err != nil {
			// Skip invalid entries rather than failing completely
			continue
		}
		lxcs = append(lxcs, lxc)
	}

	return lxcs, nil
}

// Resource represents either a VM or LXC container for unified handling.
type Resource struct {
	VMID     int    `json:"vmid"`
	Name     string `json:"name"`
	Node     string `json:"node"`
	Status   string `json:"status"`
	Template int    `json:"template"`
	Type     string `json:"type"` // "qemu" for VMs, "lxc" for containers
	MaxMem   int64  `json:"maxmem"`
	MaxDisk  int64  `json:"maxdisk"`
	CPU      float64 `json:"cpu"`
	Mem      int64  `json:"mem"`
	Uptime   int64  `json:"uptime"`
}

// ListAllResources retrieves all VMs and LXC containers across the entire Proxmox cluster.
// This is a convenience method that combines both VMs and LXCs into a unified list.
// It queries the /api2/json/cluster/resources endpoint without a type filter.
func (c *Client) ListAllResources(ctx context.Context) ([]Resource, error) {
	url := fmt.Sprintf("%s/api2/json/cluster/resources", c.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response clusterResourcesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	resources := make([]Resource, 0, len(response.Data))
	for _, rawResource := range response.Data {
		var resource Resource
		if err := json.Unmarshal(rawResource, &resource); err != nil {
			// Skip invalid entries rather than failing completely
			continue
		}
		// Only include VMs and LXCs, skip other resource types like nodes, storage
		if resource.Type == "qemu" || resource.Type == "lxc" {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}
