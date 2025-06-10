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
	"io/ioutil"
	"net/http"
)

// Client represents a Proxmox Backup Server API client.
// Use New() to create a new client for a given PBS host.
type Client struct {
	Host string
}

// New creates a new Client for the given Proxmox Backup Server host.
// The host should include the scheme (e.g., "https://pbs.example.com").
func New(host string) *Client {
	return &Client{Host: host}
}

// Ping checks the connectivity to the Proxmox Backup Server by calling the /api2/json/ping endpoint.
// It returns the raw response body as a string, or an error if the request fails.
func (c *Client) Ping() (string, error) {
	url := fmt.Sprintf("%s/api2/json/ping", c.Host)
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
