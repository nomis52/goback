package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nomis52/goback/orchestrator"
)

// Test config for metrics activity
type TestMetricsConfig struct {
	Monitoring struct {
		VictoriaMetricsURL string
		MetricsPrefix      string
	}
}

func TestMetricsActivity_Init(t *testing.T) {
	config := &TestMetricsConfig{}
	config.Monitoring.VictoriaMetricsURL = "http://localhost:8428"
	config.Monitoring.MetricsPrefix = "test_prefix"

	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	// Just test dependency injection and init
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config injection worked
	if activity.VictoriaMetricsURL != "http://localhost:8428" {
		t.Errorf("Expected VictoriaMetricsURL 'http://localhost:8428', got '%s'", activity.VictoriaMetricsURL)
	}
	if activity.MetricsPrefix != "test_prefix" {
		t.Errorf("Expected MetricsPrefix 'test_prefix', got '%s'", activity.MetricsPrefix)
	}

	// Verify internal state
	if activity.Client == nil {
		t.Error("Metrics client should be initialized")
	}
	if activity.Hostname == "" {
		t.Error("Hostname should be set")
	}
	if activity.StartTime.IsZero() {
		t.Error("Start time should be set")
	}
}

func TestMetricsActivity_Run(t *testing.T) {
	// Create a test HTTP server to mock VictoriaMetrics
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/import/prometheus") {
			t.Errorf("Expected prometheus import endpoint, got %s", r.URL.Path)
		}

		// Read the request body to verify metrics were sent
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		receivedBody = string(buf)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &TestMetricsConfig{}
	config.Monitoring.VictoriaMetricsURL = server.URL
	config.Monitoring.MetricsPrefix = "test"

	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify metrics were sent
	if receivedBody == "" {
		t.Error("No metrics were sent to the server")
	}

	// Check for expected metrics in the body
	expectedMetrics := []string{
		"test_orchestrator_runtime_seconds",
		"test_orchestrator_success",
		"test_last_run_timestamp",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(receivedBody, metric) {
			t.Errorf("Expected metric '%s' not found in request body", metric)
		}
	}

	// Verify labels are present
	expectedLabels := []string{
		`job="pbs_automation"`,
		`instance=`,
	}

	for _, label := range expectedLabels {
		if !strings.Contains(receivedBody, label) {
			t.Errorf("Expected label '%s' not found in request body", label)
		}
	}
}

func TestMetricsActivity_CreateMetric(t *testing.T) {
	config := &TestMetricsConfig{}
	config.Monitoring.VictoriaMetricsURL = "http://localhost:8428"
	config.Monitoring.MetricsPrefix = "test"

	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	// Initialize the activity
	ctx := context.Background()
	err := orchestrator.Execute(ctx)
	if err != nil {
		t.Fatalf("Expected no error during setup, got: %v", err)
	}

	// Test CreateMetric helper
	additionalLabels := map[string]string{
		"activity": "test_activity",
		"status":   "success",
	}

	metric := activity.CreateMetric("test_metric", 42.5, additionalLabels)

	// Verify metric properties
	if metric.Name != "test_metric" {
		t.Errorf("Expected name 'test_metric', got '%s'", metric.Name)
	}
	if metric.Value != 42.5 {
		t.Errorf("Expected value 42.5, got %f", metric.Value)
	}

	// Verify base labels are present
	if metric.Labels["job"] != "pbs_automation" {
		t.Errorf("Expected job label 'pbs_automation', got '%s'", metric.Labels["job"])
	}
	if metric.Labels["instance"] == "" {
		t.Error("Expected instance label to be set")
	}

	// Verify additional labels are present
	if metric.Labels["activity"] != "test_activity" {
		t.Errorf("Expected activity label 'test_activity', got '%s'", metric.Labels["activity"])
	}
	if metric.Labels["status"] != "success" {
		t.Errorf("Expected status label 'success', got '%s'", metric.Labels["status"])
	}

	// Verify timestamp is recent
	if time.Since(metric.Timestamp) > time.Second {
		t.Error("Metric timestamp should be recent")
	}
}

func TestMetricsActivity_MissingConfig(t *testing.T) {
	config := &TestMetricsConfig{}
	// Missing VictoriaMetricsURL

	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	// Should fail due to missing config
	if err == nil {
		t.Fatal("Expected error due to missing VictoriaMetrics URL, got none")
	}

	expected := "VictoriaMetrics URL is required"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain '%s', got: %v", expected, err)
	}
}
