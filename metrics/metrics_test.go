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

			if client.url != tt.wantURL {
				t.Errorf("NewClient() url = %v, want %v", client.url, tt.wantURL)
			}
			if client.prefix != tt.wantPref {
				t.Errorf("NewClient() prefix = %v, want %v", client.prefix, tt.wantPref)
			}
			if client.job != tt.wantJob {
				t.Errorf("NewClient() job = %v, want %v", client.job, tt.wantJob)
			}
			if client.instance != tt.wantInst {
				t.Errorf("NewClient() instance = %v, want %v", client.instance, tt.wantInst)
			}
			if client.timeout != tt.wantTimeout {
				t.Errorf("NewClient() timeout = %v, want %v", client.timeout, tt.wantTimeout)
			}
			if client.httpClient.Timeout != tt.wantTimeout {
				t.Errorf("NewClient() httpClient.Timeout = %v, want %v", client.httpClient.Timeout, tt.wantTimeout)
			}
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

	// Check metric name with prefix
	found := false
	for _, label := range ts.Labels {
		if label.Name == "__name__" && label.Value == "test_test_metric" {
			found = true
			break
		}
	}
	if !found {
		t.Error("metric name with prefix not found in labels")
	}

	// Check job label
	found = false
	for _, label := range ts.Labels {
		if label.Name == "job" && label.Value == "testjob" {
			found = true
			break
		}
	}
	if !found {
		t.Error("job label not found")
	}

	// Check instance label
	found = false
	for _, label := range ts.Labels {
		if label.Name == "instance" && label.Value == "testinstance" {
			found = true
			break
		}
	}
	if !found {
		t.Error("instance label not found")
	}

	// Check custom label
	found = false
	for _, label := range ts.Labels {
		if label.Name == "custom" && label.Value == "label" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom label not found")
	}

	// Check sample value
	if len(ts.Samples) != 1 {
		t.Errorf("expected 1 sample, got %d", len(ts.Samples))
	}
	if ts.Samples[0].Value != 42.0 {
		t.Errorf("expected sample value 42.0, got %f", ts.Samples[0].Value)
	}
}

func TestPushMetrics(t *testing.T) {
	// Create a test server that will receive and verify the metrics
	receivedMetrics := make(chan []prompb.TimeSeries, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Encoding") != "snappy" {
			t.Error("expected Content-Encoding: snappy")
		}
		if r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Error("expected Content-Type: application/x-protobuf")
		}
		if r.Header.Get("X-Prometheus-Remote-Write-Version") != "0.1.0" {
			t.Error("expected X-Prometheus-Remote-Write-Version: 0.1.0")
		}

		// Read and decompress the body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, err := snappy.Decode(nil, body)
		if err != nil {
			t.Errorf("failed to decode snappy: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Unmarshal the protobuf
		var writeReq prompb.WriteRequest
		if err := proto.Unmarshal(decoded, &writeReq); err != nil {
			t.Errorf("failed to unmarshal protobuf: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

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
	if err := client.PushMetrics(ctx, metrics...); err != nil {
		t.Fatalf("failed to push metrics: %v", err)
	}

	// Wait for the server to receive the metrics
	select {
	case received := <-receivedMetrics:
		// Verify the received metrics
		if len(received) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(received))
		}

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
			name := findLabel(ts.Labels, "__name__")
			expectedName := "test_test_metric_" + string(rune('1'+i))
			if name != expectedName {
				t.Errorf("metric %d: expected name %s, got %s", i, expectedName, name)
			}

			// Check job label
			if job := findLabel(ts.Labels, "job"); job != "testjob" {
				t.Errorf("metric %d: expected job testjob, got %s", i, job)
			}

			// Check instance label
			if instance := findLabel(ts.Labels, "instance"); instance != "testinstance" {
				t.Errorf("metric %d: expected instance testinstance, got %s", i, instance)
			}

			// Check custom label
			expectedCustom := "label" + string(rune('1'+i))
			if custom := findLabel(ts.Labels, "custom"); custom != expectedCustom {
				t.Errorf("metric %d: expected custom label %s, got %s", i, expectedCustom, custom)
			}

			// Check sample value
			if len(ts.Samples) != 1 {
				t.Errorf("metric %d: expected 1 sample, got %d", i, len(ts.Samples))
				continue
			}
			expectedValue := 42.0
			if i == 1 {
				expectedValue = 24.0
			}
			if ts.Samples[0].Value != expectedValue {
				t.Errorf("metric %d: expected value %f, got %f", i, expectedValue, ts.Samples[0].Value)
			}

			// Check timestamp
			if ts.Samples[0].Timestamp != now.UnixMilli() {
				t.Errorf("metric %d: expected timestamp %d, got %d", i, now.UnixMilli(), ts.Samples[0].Timestamp)
			}
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for metrics to be received")
	}
}
