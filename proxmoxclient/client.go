// Package proxmoxclient provides a simple client for interacting with a Proxmox VE server via its HTTP API.
//
// Example usage:
//
//	client := proxmoxclient.New("https://proxmox.example.com")
//	version, err := client.Version()
package proxmoxclient

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

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
