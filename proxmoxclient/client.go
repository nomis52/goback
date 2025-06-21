// Package proxmoxclient provides a simple client for interacting with a Proxmox VE server via its HTTP API.
//
// Example usage:
//
//	client, err := proxmoxclient.New("https://proxmox.example.com")
//	if err != nil {
//		log.Fatal(err)
//	}
//	version, err := client.Version()
//	vms, err := client.ListComputeResources(ctx)
//	backups, err := client.ListBackups(ctx, "pve2", "pbs")
package proxmoxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// Option is a function that configures a Client
type Option func(*Client)

// Client represents a Proxmox VE API client.
// Use New() to create a new client for a given Proxmox host.
type Client struct {
	baseURL *url.URL
	token   string
	logger  *slog.Logger
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

// Host returns the host part of the URL the client is using.
// This returns just the hostname (e.g., "pve2" from "https://pve2.d.ne4.org").
func (c *Client) Host() string {
	hostname := c.baseURL.Hostname()
	// Extract just the first part before the first dot
	if dotIndex := strings.Index(hostname, "."); dotIndex != -1 {
		return hostname[:dotIndex]
	}
	return hostname
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

	var response struct {
		Data []json.RawMessage `json:"data"`
	}
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

// ListBackups retrieves all backups from a specific storage on a specific node.
// It queries the /api2/json/nodes/{node}/storage/{storage}/content endpoint with content=backup filter.
// The node parameter specifies which Proxmox node to query (e.g., "pve2").
// The storage parameter specifies which storage to query (e.g., "pbs").
func (c *Client) ListBackups(ctx context.Context, node, storage string) ([]Backup, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/storage/%s/content?content=backup", node, storage)

	resp, err := c.doRequest(ctx, http.MethodGet, path)
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

	var response struct {
		Data []Backup `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Data, nil
}

// Backup creates a backup of a virtual machine and returns the task ID.
// It calls the /api2/json/nodes/{node}/vzdump endpoint to initiate the backup process.
// The VMID parameter specifies which VM to backup.
// The storage parameter specifies the storage target for the backup.
// The node parameter specifies which Proxmox node to use for the backup.
func (c *Client) Backup(ctx context.Context, node string, vmid VMID, storage string) (TaskID, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/vzdump", node)

	// Build query parameters
	params := url.Values{}
	params.Set("vmid", fmt.Sprintf("%d", vmid))
	params.Set("storage", storage)

	// Add parameters to path
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path)
	if err != nil {
		return "", fmt.Errorf("failed to execute backup request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response
	c.logger.Info("Proxmox backup response",
		"vmid", vmid,
		"storage", storage,
		"node", node,
		"response", string(body))

	var response backupTaskResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return TaskID(response.Data), nil
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
