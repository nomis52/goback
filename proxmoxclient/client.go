// Package proxmoxclient provides a simple client for interacting with a Proxmox VE server via its HTTP API.
//
// Example usage:
//
//	client, err := proxmoxclient.New("https://proxmox.example.com")
//	if err != nil {
//		log.Fatal(err)
//	}
//	version, err := client.Version()
//	vms, err := client.ListVMs(ctx)
package proxmoxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/url"
)

// Types

// Resource represents a virtual machine or container in Proxmox.
type Resource struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Node     string  `json:"node"`
	Status   string  `json:"status"`
	Template int     `json:"template"`
	Type     string  `json:"type"`
	MaxMem   int64   `json:"maxmem"`
	MaxDisk  int64   `json:"maxdisk"`
	CPU      float64 `json:"cpu"`
	Mem      int64   `json:"mem"`
	Uptime   int64   `json:"uptime"`
}

// VM represents a virtual machine in Proxmox.
type VM Resource

// LXC represents an LXC container in Proxmox.
type LXC Resource

// Option is a function that configures a Client
type Option func(*Client)

// Client represents a Proxmox VE API client.
// Use New() to create a new client for a given Proxmox host.
type Client struct {
	baseURL *url.URL
	token   string
	logger  *slog.Logger
}

// clusterResourcesResponse represents the response from /api2/json/cluster/resources
type clusterResourcesResponse struct {
	Data []json.RawMessage `json:"data"`
}

// New and Options

// WithToken sets the API token for authentication
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// WithLogger sets the logger for the client
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// New creates a new Client for the given Proxmox VE host.
// The host should include the scheme (e.g., "https://proxmox.example.com").
// Options can be provided to configure the client.
func New(host string, opts ...Option) (*Client, error) {
	baseURL, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	client := &Client{
		baseURL: baseURL,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Exported Methods

// Version retrieves the Proxmox VE version information by calling the /api2/json/version endpoint.
// It returns the raw response body as a string, or an error if the request fails.
func (c *Client) Version() (string, error) {
	resp, err := c.doRequest(context.Background(), http.MethodGet, "/api2/json/version")
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

// ListComputeResources retrieves all resources (VMs and LXCs) across the entire Proxmox cluster.
// It queries the /api2/json/cluster/resources endpoint to get cluster-wide resource information.
func (c *Client) ListComputeResources(ctx context.Context) ([]Resource, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api2/json/cluster/resources?type=vm")
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
		resources = append(resources, resource)
	}

	return resources, nil
}

// Non-exported Methods

// buildURL constructs a proper URL by joining the base host with the given path.
// It handles cases where the host may or may not have a trailing slash.
func (c *Client) buildURL(path string) (string, error) {
	pathURL, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	resolvedURL := c.baseURL.ResolveReference(pathURL)
	return resolvedURL.String(), nil
}

// doRequest performs an HTTP request with the configured authentication
func (c *Client) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	url, err := c.buildURL(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.token))
	}

	// Log the request details
	c.logger.Info("Proxmox API request",
		"method", method,
		"url", url,
		"path", path)

	return http.DefaultClient.Do(req)
}
