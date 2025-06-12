package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nomis52/goback/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	ctx := context.Background()
	err := orchestrator.Execute(ctx)
	require.NoError(t, err, "Expected no error")

	assert.Equal(t, "http://localhost:8428", activity.VictoriaMetricsURL)
	assert.Equal(t, "test_prefix", activity.MetricsPrefix)
	assert.NotNil(t, activity.client)
	assert.NotEmpty(t, activity.hostname)
	assert.False(t, activity.startTime.IsZero())
}

func TestMetricsActivity_Run(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/import/prometheus")
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

	ctx := context.Background()
	err := orchestrator.Execute(ctx)
	require.NoError(t, err, "Expected no error")
	assert.NotEmpty(t, receivedBody, "No metrics were sent to the server")

	expectedMetrics := []string{
		"test_orchestrator_runtime_seconds",
		"test_orchestrator_success",
		"test_last_run_timestamp",
	}
	for _, metric := range expectedMetrics {
		assert.Contains(t, receivedBody, metric)
	}

	expectedLabels := []string{
		`job="pbs_automation"`,
		`instance=`,
	}
	for _, label := range expectedLabels {
		assert.Contains(t, receivedBody, label)
	}
}

func TestMetricsActivity_CreateMetric(t *testing.T) {
	config := &TestMetricsConfig{}
	config.Monitoring.VictoriaMetricsURL = "http://localhost:8428"
	config.Monitoring.MetricsPrefix = "test"

	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	ctx := context.Background()
	err := orchestrator.Execute(ctx)
	require.NoError(t, err, "Expected no error during setup")

	additionalLabels := map[string]string{
		"activity": "test_activity",
		"status":   "success",
	}

	metric := activity.CreateMetric("test_metric", 42.5, additionalLabels)
	assert.Equal(t, "test_metric", metric.Name)
	assert.Equal(t, 42.5, metric.Value)
	assert.Equal(t, "pbs_automation", metric.Labels["job"])
	assert.NotEmpty(t, metric.Labels["instance"])
	assert.Equal(t, "test_activity", metric.Labels["activity"])
	assert.Equal(t, "success", metric.Labels["status"])
	assert.LessOrEqual(t, time.Since(metric.Timestamp), time.Second)
}

func TestMetricsActivity_MissingConfig(t *testing.T) {
	config := &TestMetricsConfig{}
	activity := &MetricsActivity{}
	orchestrator := orchestrator.NewOrchestrator(config)
	orchestrator.AddActivity(activity)

	ctx := context.Background()
	err := orchestrator.Execute(ctx)
	require.Error(t, err, "Expected error due to missing VictoriaMetrics URL")
	assert.Contains(t, err.Error(), "VictoriaMetrics URL is required")
}
