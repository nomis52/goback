package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		opts        []Option
		wantURL     string
		wantPref    string
		wantJob     string
		wantInst    string
		wantTimeout time.Duration
	}{
		{
			name:        "no options",
			url:         "http://localhost:9090",
			opts:        []Option{},
			wantURL:     "http://localhost:9090/api/v1/write",
			wantPref:    "",
			wantJob:     "",
			wantInst:    "",
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "all options",
			url:         "http://localhost:9090",
			opts:        []Option{WithPrefix("test"), WithJob("testjob"), WithInstance("testinstance")},
			wantURL:     "http://localhost:9090/api/v1/write",
			wantPref:    "test",
			wantJob:     "testjob",
			wantInst:    "testinstance",
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "partial options",
			url:         "http://localhost:9090",
			opts:        []Option{WithPrefix("test"), WithJob("testjob")},
			wantURL:     "http://localhost:9090/api/v1/write",
			wantPref:    "test",
			wantJob:     "testjob",
			wantInst:    "",
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "custom timeout",
			url:         "http://localhost:9090",
			opts:        []Option{WithTimeout(5 * time.Second)},
			wantURL:     "http://localhost:9090/api/v1/write",
			wantPref:    "",
			wantJob:     "",
			wantInst:    "",
			wantTimeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.url, tt.opts...)

			assert.Equal(t, tt.wantURL, client.url)
			assert.Equal(t, tt.wantPref, client.prefix)
			assert.Equal(t, tt.wantJob, client.job)
			assert.Equal(t, tt.wantInst, client.instance)
			assert.Equal(t, tt.wantTimeout, client.timeout)
			assert.Equal(t, tt.wantTimeout, client.httpClient.Timeout)
		})
	}
}

func TestMetricToTimeSeries(t *testing.T) {
	client := NewClient("http://localhost:9090",
		WithPrefix("test"),
		WithJob("testjob"),
		WithInstance("testinstance"),
	)

	metric := Metric{
		Name:      "test_metric",
		Value:     42.0,
		Labels:    map[string]string{"custom": "label"},
		Timestamp: time.Now(),
	}

	ts := client.metricToTimeSeries(metric)

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
	assert.Equal(t, "label", findLabel(ts.Labels, "custom"))

	// Check sample value
	require.Len(t, ts.Samples, 1)
	assert.Equal(t, 42.0, ts.Samples[0].Value)
}

func TestPushMetrics(t *testing.T) {
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

	// Create a client with test options
	client := NewClient(server.URL,
		WithPrefix("test"),
		WithJob("testjob"),
		WithInstance("testinstance"),
	)

	// Create test metrics
	now := time.Now()
	metrics := []Metric{
		{
			Name:      "test_metric_1",
			Value:     42.0,
			Labels:    map[string]string{"custom": "label1"},
			Timestamp: now,
		},
		{
			Name:      "test_metric_2",
			Value:     24.0,
			Labels:    map[string]string{"custom": "label2"},
			Timestamp: now,
		},
	}

	// Push the metrics
	ctx := context.Background()
	require.NoError(t, client.PushMetrics(ctx, metrics...))

	// Wait for the server to receive the metrics
	select {
	case received := <-receivedMetrics:
		// Verify the received metrics
		require.Len(t, received, 2)

		// Helper function to find a label value
		findLabel := func(labels []prompb.Label, name string) string {
			for _, l := range labels {
				if l.Name == name {
					return l.Value
				}
			}
			return ""
		}

		// Verify each metric
		for i, ts := range received {
			// Check metric name
			expectedName := "test_test_metric_" + string(rune('1'+i))
			assert.Equal(t, expectedName, findLabel(ts.Labels, "__name__"))
			assert.Equal(t, "testjob", findLabel(ts.Labels, "job"))
			assert.Equal(t, "testinstance", findLabel(ts.Labels, "instance"))

			// Check custom label
			expectedCustom := "label" + string(rune('1'+i))
			assert.Equal(t, expectedCustom, findLabel(ts.Labels, "custom"))

			// Check sample value
			require.Len(t, ts.Samples, 1)
			expectedValue := 42.0
			if i == 1 {
				expectedValue = 24.0
			}
			assert.Equal(t, expectedValue, ts.Samples[0].Value)
			assert.Equal(t, now.UnixMilli(), ts.Samples[0].Timestamp)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for metrics to be received")
	}
}
