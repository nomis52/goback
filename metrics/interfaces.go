// Package metrics provides interfaces and implementations for Prometheus-compatible metrics.
//
// The package supports two modes of operation:
//   - Scrape mode (server): Metrics are registered with a Prometheus registry and exposed via HTTP
//   - Push mode (CLI): Metrics are pushed to VictoriaMetrics/Prometheus remote write endpoint
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Gauge is a metric that represents a single numerical value that can go up and down.
type Gauge interface {
	// Set sets the Gauge to the given value.
	Set(float64)
}

// Counter is a metric that represents a single monotonically increasing counter.
type Counter interface {
	// Inc increments the counter by 1.
	Inc()
	// Add adds the given value to the counter. It panics if the value is negative.
	Add(float64)
}

// GaugeVec is a Gauge with labels.
type GaugeVec interface {
	// With returns the Gauge for the given Labels.
	With(prometheus.Labels) Gauge
}

// CounterVec is a Counter with labels.
type CounterVec interface {
	// With returns the Counter for the given Labels.
	With(prometheus.Labels) Counter
}

// Registry creates and registers metrics.
// Implementations handle the differences between push and scrape modes.
type Registry interface {
	// NewGauge creates and registers a new Gauge.
	NewGauge(opts prometheus.GaugeOpts) (Gauge, error)

	// NewGaugeVec creates and registers a new GaugeVec.
	NewGaugeVec(opts prometheus.GaugeOpts, labels []string) (GaugeVec, error)

	// NewCounter creates and registers a new Counter.
	NewCounter(opts prometheus.CounterOpts) (Counter, error)

	// NewCounterVec creates and registers a new CounterVec.
	NewCounterVec(opts prometheus.CounterOpts, labels []string) (CounterVec, error)
}
