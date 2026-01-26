package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ScrapeRegistry implements Registry for scrape-based metrics collection.
// Metrics are registered with a Prometheus registry and exposed via HTTP.
type ScrapeRegistry struct {
	prom      *prometheus.Registry
	startTime time.Time
}

// NewScrapeRegistry creates a new ScrapeRegistry.
func NewScrapeRegistry() (*ScrapeRegistry, error) {
	reg := prometheus.NewRegistry()

	// Register standard Go collectors
	if err := reg.Register(collectors.NewGoCollector()); err != nil {
		return nil, fmt.Errorf("registering go collector: %w", err)
	}
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("registering process collector: %w", err)
	}

	return &ScrapeRegistry{
		prom:      reg,
		startTime: time.Now(),
	}, nil
}

// Handler returns an http.Handler for the /metrics endpoint.
func (r *ScrapeRegistry) Handler() http.Handler {
	return promhttp.HandlerFor(r.prom, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// PrometheusRegistry returns the underlying Prometheus registry.
// This is useful for advanced use cases.
func (r *ScrapeRegistry) PrometheusRegistry() *prometheus.Registry {
	return r.prom
}

// NewGauge creates and registers a new Gauge.
func (r *ScrapeRegistry) NewGauge(opts prometheus.GaugeOpts) (Gauge, error) {
	g := prometheus.NewGauge(opts)
	if err := r.prom.Register(g); err != nil {
		return nil, fmt.Errorf("registering gauge %q: %w", opts.Name, err)
	}
	return &scrapeGauge{gauge: g}, nil
}

// NewGaugeVec creates and registers a new GaugeVec.
func (r *ScrapeRegistry) NewGaugeVec(opts prometheus.GaugeOpts, labels []string) (GaugeVec, error) {
	g := prometheus.NewGaugeVec(opts, labels)
	if err := r.prom.Register(g); err != nil {
		return nil, fmt.Errorf("registering gauge vec %q: %w", opts.Name, err)
	}
	return &scrapeGaugeVec{gaugeVec: g}, nil
}

// NewCounter creates and registers a new Counter.
func (r *ScrapeRegistry) NewCounter(opts prometheus.CounterOpts) (Counter, error) {
	c := prometheus.NewCounter(opts)
	if err := r.prom.Register(c); err != nil {
		return nil, fmt.Errorf("registering counter %q: %w", opts.Name, err)
	}
	return &scrapeCounter{counter: c}, nil
}

// NewCounterVec creates and registers a new CounterVec.
func (r *ScrapeRegistry) NewCounterVec(opts prometheus.CounterOpts, labels []string) (CounterVec, error) {
	c := prometheus.NewCounterVec(opts, labels)
	if err := r.prom.Register(c); err != nil {
		return nil, fmt.Errorf("registering counter vec %q: %w", opts.Name, err)
	}
	return &scrapeCounterVec{counterVec: c}, nil
}

// scrapeGauge wraps prometheus.Gauge to implement Gauge interface.
type scrapeGauge struct {
	gauge prometheus.Gauge
}

func (g *scrapeGauge) Set(v float64) {
	g.gauge.Set(v)
}

// scrapeGaugeVec wraps prometheus.GaugeVec to implement GaugeVec interface.
type scrapeGaugeVec struct {
	gaugeVec *prometheus.GaugeVec
}

func (g *scrapeGaugeVec) With(labels prometheus.Labels) Gauge {
	return &scrapeGauge{gauge: g.gaugeVec.With(labels)}
}

// scrapeCounter wraps prometheus.Counter to implement Counter interface.
type scrapeCounter struct {
	counter prometheus.Counter
}

func (c *scrapeCounter) Inc() {
	c.counter.Inc()
}

func (c *scrapeCounter) Add(v float64) {
	c.counter.Add(v)
}

// scrapeCounterVec wraps prometheus.CounterVec to implement CounterVec interface.
type scrapeCounterVec struct {
	counterVec *prometheus.CounterVec
}

func (c *scrapeCounterVec) With(labels prometheus.Labels) Counter {
	return &scrapeCounter{counter: c.counterVec.With(labels)}
}
