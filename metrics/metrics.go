package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/prompb"
)

const (
	// DefaultTimeout is the default timeout for HTTP requests
	DefaultTimeout = 30 * time.Second
)

// Metric represents a single metric endpoint
type Metric struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

type Client struct {
	url        string
	httpClient *http.Client
	registry   *prometheus.Registry
	prefix     string
	job        string
	instance   string
	timeout    time.Duration
}

// Option is a function that configures a Client
type Option func(*Client)

// WithPrefix sets the metric name prefix. All metric names will be prefixed with this value
// followed by an underscore. For example, with prefix "app" and metric name "requests",
// the final metric name will be "app_requests".
func WithPrefix(prefix string) Option {
	return func(c *Client) {
		c.prefix = prefix
	}
}

// WithJob sets the job label for all metrics. This is typically used to identify
// the service or application that the metrics belong to.
func WithJob(job string) Option {
	return func(c *Client) {
		c.job = job
	}
}

// WithInstance sets the instance label for all metrics. This is typically used to
// identify the specific instance of the service that generated the metrics.
func WithInstance(instance string) Option {
	return func(c *Client) {
		c.instance = instance
	}
}

// WithTimeout sets the HTTP client timeout for metric pushes. The default timeout
// is DefaultTimeout. This timeout applies to the entire HTTP request, including
// connection establishment, request writing, and response reading.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

func NewClient(url string, opts ...Option) *Client {
	client := &Client{
		url:        url + "/api/v1/write",
		httpClient: &http.Client{Timeout: DefaultTimeout},
		registry:   prometheus.NewRegistry(),
		timeout:    DefaultTimeout,
	}
	for _, opt := range opts {
		opt(client)
	}
	client.httpClient.Timeout = client.timeout
	return client
}

func (c *Client) PushMetrics(ctx context.Context, metrics ...Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Convert to Prometheus TimeSeries format
	timeseries := make([]prompb.TimeSeries, 0, len(metrics))

	for _, metric := range metrics {
		ts := c.metricToTimeSeries(metric)
		timeseries = append(timeseries, ts)
	}

	req := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling write request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// metricToTimeSeries converts a Metric to Prometheus TimeSeries format
func (c *Client) metricToTimeSeries(metric Metric) prompb.TimeSeries {
	// Build labels
	labels := make([]prompb.Label, 0, len(metric.Labels)+3)

	// Add metric name with prefix
	name := metric.Name
	if c.prefix != "" {
		name = c.prefix + "_" + name
	}
	labels = append(labels, prompb.Label{
		Name:  "__name__",
		Value: name,
	})

	// Add job and instance labels
	if c.job != "" {
		labels = append(labels, prompb.Label{
			Name:  "job",
			Value: c.job,
		})
	}
	if c.instance != "" {
		labels = append(labels, prompb.Label{
			Name:  "instance",
			Value: c.instance,
		})
	}

	// Add custom labels
	for k, v := range metric.Labels {
		labels = append(labels, prompb.Label{
			Name:  k,
			Value: v,
		})
	}

	// Create sample
	timestamp := metric.Timestamp.UnixMilli()
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	sample := prompb.Sample{
		Value:     metric.Value,
		Timestamp: timestamp,
	}

	return prompb.TimeSeries{
		Labels:  labels,
		Samples: []prompb.Sample{sample},
	}
}
