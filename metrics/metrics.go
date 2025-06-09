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

// Metric represents a single metric point
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
}

func NewClient(url string, prefix string) *Client {
	return &Client{
		url:        url + "/api/v1/write",
		httpClient: &http.Client{Timeout: 30 * time.Second},
		registry:   prometheus.NewRegistry(),
		prefix:     prefix,
	}
}

func (c *Client) PushMetrics(ctx context.Context, metrics []Metric) error {
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(compressed))
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

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// metricToTimeSeries converts a Metric to Prometheus TimeSeries format
func (c *Client) metricToTimeSeries(metric Metric) prompb.TimeSeries {
	// Build labels
	labels := make([]prompb.Label, 0, len(metric.Labels)+1)

	// Add metric name with prefix
	name := metric.Name
	if c.prefix != "" {
		name = c.prefix + "_" + name
	}
	labels = append(labels, prompb.Label{
		Name:  "__name__",
		Value: name,
	})

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
