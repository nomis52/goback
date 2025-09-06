// Package pbsclient provides a simple client for interacting with a Proxmox Backup Server (PBS) via its HTTP API.
//
// Example usage:
//
//	client := pbsclient.New("https://pbs.example.com")
//	resp, err := client.Ping()
//
// Currently, only the Ping method is implemented.
package pbsclient

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests to PBS
	defaultHTTPTimeout = 10 * time.Second
)

// Client represents a Proxmox Backup Server API client.
// Use New() to create a new client for a given PBS host.
type Client struct {
	Host   string
	Logger *slog.Logger
	client *http.Client
}

// New creates a new Client for the given Proxmox Backup Server host.
// The host should include the scheme (e.g., "https://pbs.example.com").
func New(host string, logger *slog.Logger) (*Client, error) {
	// Ensure host has a scheme
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		return nil, fmt.Errorf("host URL must include scheme (http:// or https://): %s", host)
	}

	// Validate the URL
	_, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	return &Client{
		Host:   host,
		Logger: logger.With("component", "pbsclient"),
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}, nil
}

// Ping checks the connectivity to the Proxmox Backup Server by calling the /api2/json/ping endpoint.
// It returns the raw response body as a string, or an error if the request fails.
func (c *Client) Ping() (string, error) {
	url := fmt.Sprintf("%s/api2/json/ping", c.Host)
	c.Logger.Debug("pinging PBS server", "url", url)

	resp, err := c.client.Get(url)
	if err != nil {
		c.Logger.Error("failed to ping PBS server", "error", err, "url", url)
		return "", fmt.Errorf("failed to ping PBS server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("PBS server returned non-200 status", "status", resp.StatusCode, "url", url)
		return "", fmt.Errorf("PBS server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Logger.Error("failed to read response body", "error", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	c.Logger.Debug("successfully pinged PBS server", "response", string(body))
	return string(body), nil
}
