// Package pbsclient provides a client for the Proxmox Backup Server (PBS) HTTP API.
//
// Example usage:
//
//	client, _ := pbsclient.New("https://pbs.example.com:8007",
//		pbsclient.WithToken("goback@pbs!maintenance:secret"))
//	_, err := client.Ping()
//	task, err := client.RunPruneJob(ctx, "goback-prune")
//	status, err := client.WaitForTask(ctx, task, 10*time.Second)
//
// Ping needs no authentication. The maintenance calls (RunPruneJob,
// StartGarbageCollection, TaskStatus and WaitForTask) need an API token with the
// matching Datastore privileges on PBS, supplied with WithToken.
package pbsclient

import (
	"context"
	"encoding/json"
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

	// maxErrorBodyBytes caps how much of an error response body is kept in an
	// error message, so a stray HTML page can't flood the logs.
	maxErrorBodyBytes = 512

	// taskNode is the node name used in task API paths. PBS is single-node and
	// accepts "localhost" for the local node regardless of its hostname.
	taskNode = "localhost"
)

// Option is a function that configures a Client
type Option func(*Client)

// WithLogger sets a custom logger for the Client
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		c.Logger = logger
	}
}

// WithToken sets the API token used to authenticate requests, in PBS's
// "user@realm!tokenname:secret" form. Requests are sent with an
// "Authorization: PBSAPIToken=..." header. Ping works without a token; the
// maintenance calls do not.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// Client represents a Proxmox Backup Server API client.
// Use New() to create a new client for a given PBS host.
type Client struct {
	Host   string
	Logger *slog.Logger
	client *http.Client
	token  string
}

// New creates a new Client for the given Proxmox Backup Server host.
// The host should include the scheme (e.g., "https://pbs.example.com").
func New(host string, opts ...Option) (*Client, error) {
	// Ensure host has a scheme
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		return nil, fmt.Errorf("host URL must include scheme (http:// or https://): %s", host)
	}

	// Validate the URL
	_, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	c := &Client{
		Host: host,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Set default logger if not provided
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	// Ensure all logs from this client are tagged with the component name
	c.Logger = c.Logger.With("component", "pbsclient")

	return c, nil
}

// Ping checks the connectivity to the Proxmox Backup Server by calling the /api2/json/ping endpoint.
// It returns the raw response body as a string, or an error if the request fails.
func (c *Client) Ping() (string, error) {
	url := fmt.Sprintf("%s/api2/json/ping", c.Host)
	c.Logger.Debug("pinging PBS server", "url", url)

	resp, err := c.client.Get(url)
	if err != nil {
		c.Logger.Debug("failed to ping PBS server", "error", err, "url", url)
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

// RunPruneJob starts a configured PBS prune job by its ID, the same as "Run now"
// in the PBS UI, and returns the ID of the resulting task. The job runs even when
// it is disabled in its configuration: the disable flag only stops the PBS
// scheduler from starting it, which makes a disabled job a convenient way to keep
// the retention settings in PBS while leaving the timing to the caller.
// It calls POST /api2/json/admin/prune/{id}/run, which needs Datastore.Modify
// on the job's datastore.
func (c *Client) RunPruneJob(ctx context.Context, jobID string) (TaskID, error) {
	path := fmt.Sprintf("/api2/json/admin/prune/%s/run", url.PathEscape(jobID))
	return c.startTask(ctx, path)
}

// StartGarbageCollection starts garbage collection on the given datastore and
// returns the ID of the resulting task. Garbage collection is what actually frees
// the chunks that pruning made unreferenced.
// It calls POST /api2/json/admin/datastore/{store}/gc, which needs
// Datastore.Modify on the datastore.
func (c *Client) StartGarbageCollection(ctx context.Context, datastore string) (TaskID, error) {
	path := fmt.Sprintf("/api2/json/admin/datastore/%s/gc", url.PathEscape(datastore))
	return c.startTask(ctx, path)
}

// TaskStatus retrieves the status of a task by its ID.
// It calls GET /api2/json/nodes/localhost/tasks/{upid}/status.
func (c *Client) TaskStatus(ctx context.Context, taskID TaskID) (*TaskStatus, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/status", taskNode, url.PathEscape(string(taskID)))

	resp, err := c.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("failed to execute task status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response struct {
		Data TaskStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response.Data, nil
}

// WaitForTask polls TaskStatus every pollInterval until the task has finished or
// ctx is done, and returns the final status. It does not judge the outcome: check
// the returned status with Succeeded. A status request that fails ends the wait
// with that error. Cancelling ctx does not stop the task on PBS.
func (c *Client) WaitForTask(ctx context.Context, taskID TaskID, pollInterval time.Duration) (*TaskStatus, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("waiting for task %s: %w", taskID, err)
		}

		status, err := c.TaskStatus(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if status.Finished() {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for task %s: %w", taskID, ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

// startTask POSTs to an endpoint that starts a PBS task and returns the task ID
// from the response.
func (c *Client) startTask(ctx context.Context, path string) (TaskID, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, path)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response taskResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if response.Data == "" {
		return "", fmt.Errorf("PBS did not return a task ID")
	}

	c.Logger.Debug("started PBS task", "path", path, "upid", response.Data)
	return TaskID(response.Data), nil
}

// doRequest performs an HTTP request against the PBS API with the configured
// authentication.
func (c *Client) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	url := c.Host + path

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("PBSAPIToken=%s", c.token))
	}

	c.Logger.Debug("PBS API request", "method", method, "url", url)

	return c.client.Do(req)
}

// statusError builds an error for a non-200 response. PBS reports the underlying
// failure in the HTTP reason phrase (e.g. "400 no such prune job 'x'") and often in
// a JSON body too, so both are included: a bare status code gives no clue whether
// the problem is the request, the token's privileges, or the datastore.
func statusError(resp *http.Response) error {
	msg := strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprintf("%d", resp.StatusCode)))

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	if err == nil {
		if detail := strings.TrimSpace(string(body)); detail != "" && detail != `{"data":null}` {
			if msg != "" {
				msg += ": "
			}
			msg += detail
		}
	}

	if msg == "" {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return fmt.Errorf("unexpected status code: %d (%s)", resp.StatusCode, msg)
}
