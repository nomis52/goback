package workflows

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingActivity blocks in Execute until release is closed, letting the test
// observe the step_running gauge while the step is mid-flight.
type blockingActivity struct {
	started chan struct{}
	release chan struct{}
}

func (a *blockingActivity) Init() error { return nil }

func (a *blockingActivity) Execute(context.Context) error {
	close(a.started)
	<-a.release
	return nil
}

func scrapeBody(t *testing.T, registry *metrics.ScrapeRegistry) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	registry.Handler().ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	return w.Body.String()
}

func TestInjectInto_StepRunningMetric(t *testing.T) {
	registry, err := metrics.NewScrapeRegistry()
	require.NoError(t, err)

	act := &blockingActivity{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	o := workflow.NewOrchestrator(workflow.WithLogger(slog.Default()))
	require.NoError(t, o.AddActivity(act))

	Params{Logger: slog.Default(), Registry: registry}.InjectInto(o)

	done := make(chan error, 1)
	go func() { done <- o.Execute(context.Background()) }()

	// Wait until the step is actually executing.
	select {
	case <-act.started:
	case <-time.After(2 * time.Second):
		t.Fatal("activity did not start")
	}

	// While running, the gauge should report 1 for this step.
	assert.Contains(t, scrapeBody(t, registry), `step_running{step="workflows.blockingActivity"} 1`)

	// Let the step finish and wait for the workflow to complete.
	close(act.release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not finish")
	}

	// Once finished, the gauge should report 0.
	assert.Contains(t, scrapeBody(t, registry), `step_running{step="workflows.blockingActivity"} 0`)
}

func TestInjectInto_NoRegistry(t *testing.T) {
	// Without a registry, InjectInto must not install an observer or panic.
	o := workflow.NewOrchestrator(workflow.WithLogger(slog.Default()))
	require.NoError(t, o.AddActivity(&blockingActivityNoBlock{}))

	Params{Logger: slog.Default()}.InjectInto(o)
	require.NoError(t, o.Execute(context.Background()))
}

// blockingActivityNoBlock is a trivial activity used where blocking is not wanted.
type blockingActivityNoBlock struct{}

func (a *blockingActivityNoBlock) Init() error                   { return nil }
func (a *blockingActivityNoBlock) Execute(context.Context) error { return nil }
