package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
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

// PushRegistry implements Registry for push-based metrics collection.
// Metrics are pushed to a VictoriaMetrics/Prometheus remote write endpoint.
type PushRegistry struct {
	pusher *pusher
}

// PushConfig configures a PushRegistry.
type PushConfig struct {
	// URL is the base URL of the remote write endpoint (e.g., "http://localhost:9090").
	URL string
	// Prefix is the metric name prefix. All metric names will be prefixed with this value
	// followed by an underscore.
	Prefix string
	// Job is the job label for all metrics.
	Job string
	// Instance is the instance label for all metrics.
	Instance string
	// Timeout is the HTTP client timeout. Defaults to DefaultTimeout.
	Timeout time.Duration
}

// NewPushRegistry creates a new PushRegistry that pushes metrics to the given URL.
func NewPushRegistry(cfg PushConfig) *PushRegistry {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	p := &pusher{
		url:        cfg.URL + "/api/v1/write",
		httpClient: &http.Client{Timeout: timeout},
		prefix:     cfg.Prefix,
		job:        cfg.Job,
		instance:   cfg.Instance,
		timeout:    timeout,
	}
	return &PushRegistry{pusher: p}
}

// NewGauge creates a new push-based Gauge.
func (r *PushRegistry) NewGauge(opts prometheus.GaugeOpts) (Gauge, error) {
	return &pushGauge{
		pusher: r.pusher,
		name:   opts.Name,
	}, nil
}

// NewGaugeVec creates a new push-based GaugeVec.
func (r *PushRegistry) NewGaugeVec(opts prometheus.GaugeOpts, labels []string) (GaugeVec, error) {
	return &pushGaugeVec{
		pusher: r.pusher,
		name:   opts.Name,
		labels: labels,
	}, nil
}

// NewCounter creates a new push-based Counter.
func (r *PushRegistry) NewCounter(opts prometheus.CounterOpts) (Counter, error) {
	return &pushCounter{
		pusher: r.pusher,
		name:   opts.Name,
	}, nil
}

// NewCounterVec creates a new push-based CounterVec.
func (r *PushRegistry) NewCounterVec(opts prometheus.CounterOpts, labels []string) (CounterVec, error) {
	return &pushCounterVec{
		pusher: r.pusher,
		name:   opts.Name,
		labels: labels,
	}, nil
}

// pusher handles remote write to VictoriaMetrics/Prometheus.
type pusher struct {
	url        string
	httpClient *http.Client
	prefix     string
	job        string
	instance   string
	timeout    time.Duration
}

// push sends a single metric to the remote write endpoint.
func (p *pusher) push(name string, value float64, labels map[string]string) error {
	ts := p.metricToTimeSeries(name, value, labels)

	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{ts},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling write request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := p.httpClient.Do(httpReq)
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

// metricToTimeSeries converts a metric to Prometheus TimeSeries format.
func (p *pusher) metricToTimeSeries(name string, value float64, labels map[string]string) prompb.TimeSeries {
	// Build labels
	promLabels := make([]prompb.Label, 0, len(labels)+3)

	// Add metric name with prefix
	metricName := name
	if p.prefix != "" {
		metricName = p.prefix + "_" + name
	}
	promLabels = append(promLabels, prompb.Label{
		Name:  "__name__",
		Value: metricName,
	})

	// Add job and instance labels
	if p.job != "" {
		promLabels = append(promLabels, prompb.Label{
			Name:  "job",
			Value: p.job,
		})
	}
	if p.instance != "" {
		promLabels = append(promLabels, prompb.Label{
			Name:  "instance",
			Value: p.instance,
		})
	}

	// Add custom labels
	for k, v := range labels {
		promLabels = append(promLabels, prompb.Label{
			Name:  k,
			Value: v,
		})
	}

	// Create sample
	sample := prompb.Sample{
		Value:     value,
		Timestamp: time.Now().UnixMilli(),
	}

	return prompb.TimeSeries{
		Labels:  promLabels,
		Samples: []prompb.Sample{sample},
	}
}

// pushGauge implements Gauge for push mode.
type pushGauge struct {
	pusher *pusher
	name   string
	labels map[string]string
}

func (g *pushGauge) Set(v float64) {
	// Fire and forget - errors are logged but not returned
	_ = g.pusher.push(g.name, v, g.labels)
}

// pushGaugeVec implements GaugeVec for push mode.
type pushGaugeVec struct {
	pusher *pusher
	name   string
	labels []string
}

func (g *pushGaugeVec) With(labels prometheus.Labels) Gauge {
	return &pushGauge{
		pusher: g.pusher,
		name:   g.name,
		labels: labels,
	}
}

// pushCounter implements Counter for push mode.
type pushCounter struct {
	mu     sync.Mutex
	pusher *pusher
	name   string
	labels map[string]string
	value  float64
}

func (c *pushCounter) Inc() {
	c.Add(1)
}

func (c *pushCounter) Add(v float64) {
	c.mu.Lock()
	c.value += v
	value := c.value
	c.mu.Unlock()
	_ = c.pusher.push(c.name, value, c.labels)
}

// pushCounterVec implements CounterVec for push mode.
type pushCounterVec struct {
	mu       sync.Mutex
	pusher   *pusher
	name     string
	labels   []string
	counters map[string]*pushCounter
}

func (c *pushCounterVec) With(labels prometheus.Labels) Counter {
	// Create a key from labels
	key := labelsToKey(labels)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.counters == nil {
		c.counters = make(map[string]*pushCounter)
	}

	if counter, ok := c.counters[key]; ok {
		return counter
	}

	counter := &pushCounter{
		pusher: c.pusher,
		name:   c.name,
		labels: labels,
	}
	c.counters[key] = counter
	return counter
}

// labelsToKey creates a string key from labels for map lookup.
func labelsToKey(labels prometheus.Labels) string {
	var key string
	for k, v := range labels {
		key += k + "=" + v + ","
	}
	return key
}
