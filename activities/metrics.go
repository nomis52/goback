package activities

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/orchestrator"
)

// MetricsActivity provides metrics client as a service to other activities
// Should be one of the first activities to ensure metrics are available early
type MetricsActivity struct {
	// Config
	VictoriaMetricsURL string `config:"monitoring.victoriametrics_url"`
	MetricsPrefix      string `config:"monitoring.metrics_prefix"`
	JobName            string `config:"monitoring.jobname"`

	// Internal state - available to dependent activities
	client    *metrics.Client
	hostname  string
	startTime time.Time
}

func (a *MetricsActivity) Init() error {
	if a.VictoriaMetricsURL == "" {
		return fmt.Errorf("VictoriaMetrics URL is required")
	}

	// Initialize metrics client
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	a.hostname = hostname

	a.client = metrics.NewClient(a.VictoriaMetricsURL, a.MetricsPrefix, a.JobName, a.hostname)

	// Record start time for duration metrics
	a.startTime = time.Now()

	return nil
}

func (a *MetricsActivity) Run(ctx context.Context) (orchestrator.Result, error) {
	// Push initial orchestrator metrics to indicate we've started
	initialMetrics := []metrics.Metric{
		a.CreateMetric("orchestrator_started", 1, map[string]string{
			"start_time": a.startTime.Format(time.RFC3339),
		}),
		a.CreateMetric("orchestrator_start_timestamp", float64(a.startTime.Unix()), nil),
	}

	// Push startup metrics
	if err := a.client.PushMetrics(ctx, initialMetrics...); err != nil {
		return orchestrator.NewFailureResult(), fmt.Errorf("failed to push startup metrics: %w", err)
	}

	return orchestrator.NewSuccessResult(), nil
}

// CreateMetric constructs a metrics.Metric with base and additional labels
func (a *MetricsActivity) CreateMetric(name string, value float64, additionalLabels map[string]string) metrics.Metric {
	labels := make(map[string]string)
	if a.JobName != "" {
		labels["job"] = a.JobName
	}
	if a.hostname != "" {
		labels["instance"] = a.hostname
	}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return metrics.Metric{
		Name:      name,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}
