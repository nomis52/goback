// Package workflows provides application-specific workflow definitions and infrastructure.
// Unlike the generic workflow package (which handles orchestration),
// this package contains types specific to the goback application.
package workflows

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
)

// Params contains common parameters for workflow construction.
// Workflows may use all or a subset of these fields depending on their needs.
type Params struct {
	// Config is the application configuration. Some workflows (e.g., test) may not need this.
	Config *config.Config

	// Logger is the base logger for the workflow.
	Logger *slog.Logger

	// StatusCollection tracks activity status updates. May be nil if status tracking is not needed.
	StatusCollection *activity.StatusHandler

	// LoggerFactory creates per-activity loggers. If nil, Logger is used for all activities.
	LoggerFactory func(workflow.ActivityID) *slog.Logger

	// Registry is used for activity-level metrics. May be nil if metrics are not needed.
	Registry metrics.Registry
}

// InjectInto registers common factories into an orchestrator.
// This eliminates duplication across workflow constructors by providing
// the standard logger factory, metrics registry, and status line factories.
func (p Params) InjectInto(o *workflow.Orchestrator) {
	// Default logger factory to shared logger if not provided
	loggerFactory := p.LoggerFactory
	if loggerFactory == nil {
		loggerFactory = workflow.Shared(p.Logger)
	}

	// Metrics registry (optional - activities check for nil)
	if p.Registry != nil {
		workflow.Provide(o, workflow.Shared(p.Registry))
		p.registerStepRunningMetric(o)
	}

	// Logger factory (per-activity, defaults to shared logger)
	workflow.Provide(o, loggerFactory)

	// StatusLine factory (per-activity)
	workflow.Provide(o, func(id workflow.ActivityID) *activity.StatusLine {
		activityLogger := loggerFactory(id)
		return activity.NewStatusLine(id, activityLogger, p.StatusCollection)
	})
}

// registerStepRunningMetric installs a step_running gauge that reports 1 while a
// step (activity) is executing and 0 once it finishes. It observes activity state
// transitions on the orchestrator so every step is covered without per-activity
// wiring. The caller must ensure p.Registry is non-nil.
func (p Params) registerStepRunningMetric(o *workflow.Orchestrator) {
	stepRunning, err := p.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: "step_running",
		Help: "Whether a workflow step is currently running (1) or not (0)",
	}, []string{"step"})
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error("failed to create step_running metric", "error", err)
		}
		return
	}

	o.SetActivityStateObserver(func(id workflow.ActivityID, state workflow.ActivityState) {
		// Label by the bare step name (the activity's type) so a step reused
		// across workflows maps to a single series.
		gauge := stepRunning.With(prometheus.Labels{"step": id.Type})
		switch state {
		case workflow.Running:
			gauge.Set(1)
		case workflow.Completed, workflow.Skipped:
			gauge.Set(0)
		}
	})
}
