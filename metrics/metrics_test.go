package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPushRegistry(t *testing.T) {
	tests := []struct {
		name string
		cfg  PushConfig
	}{
		{
			name: "minimal config",
			cfg: PushConfig{
				URL: "http://localhost:9090",
			},
		},
		{
			name: "full config",
			cfg: PushConfig{
				URL:      "http://localhost:9090",
				Prefix:   "test",
				Job:      "testjob",
				Instance: "testinstance",
				Timeout:  5 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewPushRegistry(tt.cfg)
			require.NotNil(t, registry)
			require.NotNil(t, registry.pusher)
		})
	}
}

func TestPushRegistry_NewGauge(t *testing.T) {
	registry := NewPushRegistry(PushConfig{URL: "http://localhost:9090"})

	gauge, err := registry.NewGauge(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge",
	})

	require.NoError(t, err)
	require.NotNil(t, gauge)
}

func TestPushRegistry_NewGaugeVec(t *testing.T) {
	registry := NewPushRegistry(PushConfig{URL: "http://localhost:9090"})

	gaugeVec, err := registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_vec",
		Help: "A test gauge vector",
	}, []string{"label1", "label2"})

	require.NoError(t, err)
	require.NotNil(t, gaugeVec)

	// Get a gauge with labels
	gauge := gaugeVec.With(prometheus.Labels{"label1": "value1", "label2": "value2"})
	require.NotNil(t, gauge)
}

func TestPushRegistry_NewCounter(t *testing.T) {
	registry := NewPushRegistry(PushConfig{URL: "http://localhost:9090"})

	counter, err := registry.NewCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})

	require.NoError(t, err)
	require.NotNil(t, counter)
}

func TestPushRegistry_NewCounterVec(t *testing.T) {
	registry := NewPushRegistry(PushConfig{URL: "http://localhost:9090"})

	counterVec, err := registry.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter_vec",
		Help: "A test counter vector",
	}, []string{"label1"})

	require.NoError(t, err)
	require.NotNil(t, counterVec)

	// Get a counter with labels
	counter := counterVec.With(prometheus.Labels{"label1": "value1"})
	require.NotNil(t, counter)
}

func TestPushGauge_Set(t *testing.T) {
	// Create a test server that will receive and verify the metrics
	receivedMetrics := make(chan []prompb.TimeSeries, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		assert.Equal(t, "0.1.0", r.Header.Get("X-Prometheus-Remote-Write-Version"))

		// Read and decompress the body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		decoded, err := snappy.Decode(nil, body)
		require.NoError(t, err)

		// Unmarshal the protobuf
		var writeReq prompb.WriteRequest
		require.NoError(t, proto.Unmarshal(decoded, &writeReq))

		// Send the received metrics to the channel
		receivedMetrics <- writeReq.Timeseries
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create registry with test options
	registry := NewPushRegistry(PushConfig{
		URL:      server.URL,
		Prefix:   "test",
		Job:      "testjob",
		Instance: "testinstance",
	})

	// Create and set a gauge
	gauge, err := registry.NewGauge(prometheus.GaugeOpts{
		Name: "test_metric",
		Help: "A test metric",
	})
	require.NoError(t, err)
	gauge.Set(42.0)

	// Wait for the server to receive the metric
	select {
	case received := <-receivedMetrics:
		require.Len(t, received, 1)
		ts := received[0]

		// Helper function to find a label value
		findLabel := func(labels []prompb.Label, name string) string {
			for _, l := range labels {
				if l.Name == name {
					return l.Value
				}
			}
			return ""
		}

		// Check metric name with prefix
		assert.Equal(t, "test_test_metric", findLabel(ts.Labels, "__name__"))
		assert.Equal(t, "testjob", findLabel(ts.Labels, "job"))
		assert.Equal(t, "testinstance", findLabel(ts.Labels, "instance"))

		// Check sample value
		require.Len(t, ts.Samples, 1)
		assert.Equal(t, 42.0, ts.Samples[0].Value)

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for metrics to be received")
	}
}

func TestPushGaugeVec_WithLabels(t *testing.T) {
	// Create a test server that will receive metrics
	receivedMetrics := make(chan []prompb.TimeSeries, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		decoded, err := snappy.Decode(nil, body)
		require.NoError(t, err)

		var writeReq prompb.WriteRequest
		require.NoError(t, proto.Unmarshal(decoded, &writeReq))

		receivedMetrics <- writeReq.Timeseries
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := NewPushRegistry(PushConfig{URL: server.URL})

	gaugeVec, err := registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_vec",
		Help: "A test gauge vector",
	}, []string{"vmid", "name"})
	require.NoError(t, err)

	// Set a gauge with labels
	gaugeVec.With(prometheus.Labels{"vmid": "100", "name": "testvm"}).Set(123.0)

	// Wait for the metric
	select {
	case received := <-receivedMetrics:
		require.Len(t, received, 1)
		ts := received[0]

		findLabel := func(labels []prompb.Label, name string) string {
			for _, l := range labels {
				if l.Name == name {
					return l.Value
				}
			}
			return ""
		}

		assert.Equal(t, "test_gauge_vec", findLabel(ts.Labels, "__name__"))
		assert.Equal(t, "100", findLabel(ts.Labels, "vmid"))
		assert.Equal(t, "testvm", findLabel(ts.Labels, "name"))
		require.Len(t, ts.Samples, 1)
		assert.Equal(t, 123.0, ts.Samples[0].Value)

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for metrics to be received")
	}
}

func TestPushCounter_Inc(t *testing.T) {
	// Create a test server
	receivedMetrics := make(chan []prompb.TimeSeries, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		decoded, err := snappy.Decode(nil, body)
		require.NoError(t, err)

		var writeReq prompb.WriteRequest
		require.NoError(t, proto.Unmarshal(decoded, &writeReq))

		receivedMetrics <- writeReq.Timeseries
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := NewPushRegistry(PushConfig{URL: server.URL})

	counter, err := registry.NewCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})
	require.NoError(t, err)

	// Increment counter twice
	counter.Inc()
	counter.Inc()

	// We should receive two pushes
	for i := 0; i < 2; i++ {
		select {
		case received := <-receivedMetrics:
			require.Len(t, received, 1)
			ts := received[0]
			require.Len(t, ts.Samples, 1)
			// Counter should increment: 1, then 2
			assert.Equal(t, float64(i+1), ts.Samples[0].Value)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for metric %d", i+1)
		}
	}
}

func TestScrapeRegistry(t *testing.T) {
	registry, err := NewScrapeRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Create some metrics
	gauge, err := registry.NewGauge(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge",
	})
	require.NoError(t, err)
	gauge.Set(42.0)

	counter, err := registry.NewCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})
	require.NoError(t, err)
	counter.Inc()

	// Get the HTTP handler
	handler := registry.Handler()
	require.NotNil(t, handler)

	// Make a request to the handler
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "test_gauge 42")
	assert.Contains(t, body, "test_counter 1")
}

func TestScrapeRegistry_DuplicateRegistration(t *testing.T) {
	registry, err := NewScrapeRegistry()
	require.NoError(t, err)

	t.Run("Gauge", func(t *testing.T) {
		gauge1, err := registry.NewGauge(prometheus.GaugeOpts{
			Name: "duplicate_gauge",
			Help: "A test gauge",
		})
		require.NoError(t, err)
		gauge1.Set(10.0)

		// Register again - should return existing gauge without error
		gauge2, err := registry.NewGauge(prometheus.GaugeOpts{
			Name: "duplicate_gauge",
			Help: "A test gauge",
		})
		require.NoError(t, err)

		// Both should reference the same underlying metric
		gauge2.Set(20.0)

		// Verify the value is 20 (from gauge2)
		handler := registry.Handler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Contains(t, w.Body.String(), "duplicate_gauge 20")
	})

	t.Run("GaugeVec", func(t *testing.T) {
		gaugeVec1, err := registry.NewGaugeVec(prometheus.GaugeOpts{
			Name: "duplicate_gauge_vec",
			Help: "A test gauge vec",
		}, []string{"label"})
		require.NoError(t, err)
		gaugeVec1.With(prometheus.Labels{"label": "a"}).Set(100.0)

		// Register again - should return existing gauge vec without error
		gaugeVec2, err := registry.NewGaugeVec(prometheus.GaugeOpts{
			Name: "duplicate_gauge_vec",
			Help: "A test gauge vec",
		}, []string{"label"})
		require.NoError(t, err)

		// Both should reference the same underlying metric
		gaugeVec2.With(prometheus.Labels{"label": "a"}).Set(200.0)

		handler := registry.Handler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Contains(t, w.Body.String(), `duplicate_gauge_vec{label="a"} 200`)
	})

	t.Run("Counter", func(t *testing.T) {
		counter1, err := registry.NewCounter(prometheus.CounterOpts{
			Name: "duplicate_counter",
			Help: "A test counter",
		})
		require.NoError(t, err)
		counter1.Inc()

		// Register again - should return existing counter without error
		counter2, err := registry.NewCounter(prometheus.CounterOpts{
			Name: "duplicate_counter",
			Help: "A test counter",
		})
		require.NoError(t, err)

		// Both should reference the same underlying metric
		counter2.Inc()

		handler := registry.Handler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Contains(t, w.Body.String(), "duplicate_counter 2")
	})

	t.Run("CounterVec", func(t *testing.T) {
		counterVec1, err := registry.NewCounterVec(prometheus.CounterOpts{
			Name: "duplicate_counter_vec",
			Help: "A test counter vec",
		}, []string{"label"})
		require.NoError(t, err)
		counterVec1.With(prometheus.Labels{"label": "b"}).Inc()

		// Register again - should return existing counter vec without error
		counterVec2, err := registry.NewCounterVec(prometheus.CounterOpts{
			Name: "duplicate_counter_vec",
			Help: "A test counter vec",
		}, []string{"label"})
		require.NoError(t, err)

		// Both should reference the same underlying metric
		counterVec2.With(prometheus.Labels{"label": "b"}).Add(2)

		handler := registry.Handler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Contains(t, w.Body.String(), `duplicate_counter_vec{label="b"} 3`)
	})
}
