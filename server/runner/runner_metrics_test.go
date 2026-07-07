package runner

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type metricsConfigProvider struct{}

func (metricsConfigProvider) Config() *config.Config { return &config.Config{} }

// blockingWorkflow blocks in Execute until release is closed, letting a test
// observe the runner while a workflow is in flight.
type blockingWorkflow struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingWorkflow) Execute(context.Context) error {
	close(b.started)
	<-b.release
	return nil
}

func (b *blockingWorkflow) GetAllResults() map[workflow.ActivityID]*workflow.Result {
	return nil
}

// scrape returns the current /metrics output from the scrape registry.
func scrape(t *testing.T, registry *metrics.ScrapeRegistry) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	registry.Handler().ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	return w.Body.String()
}

func TestRunner_WorkflowRunningMetric(t *testing.T) {
	registry, err := metrics.NewScrapeRegistry()
	require.NoError(t, err)

	wf := &blockingWorkflow{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	factories := map[string]WorkflowFactory{
		"blocking": func(workflows.Params) (workflow.Workflow, error) { return wf, nil },
	}

	r := New(slog.Default(), metricsConfigProvider{}, factories, WithMetricsRegistry(registry))

	require.NoError(t, r.Run([]string{"blocking"}))

	// Wait until the workflow is actually executing.
	select {
	case <-wf.started:
	case <-time.After(2 * time.Second):
		t.Fatal("workflow did not start")
	}

	// While running, the gauge should report 1.
	assert.Contains(t, scrape(t, registry), `workflow_running{workflow="blocking"} 1`)

	// Let the workflow finish and wait for the run to settle.
	close(wf.release)
	require.Eventually(t, func() bool {
		return !r.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Once idle, the gauge should report 0.
	assert.Contains(t, scrape(t, registry), `workflow_running{workflow="blocking"} 0`)
}
