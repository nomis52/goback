// Package poweroff provides workflow factories for power-off operations.
// It orchestrates the graceful shutdown of the PBS server via IPMI.
package poweroff

import (
	"fmt"
	"log/slog"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/workflow"
)

// WorkflowOption configures workflow creation.
type WorkflowOption func(*workflowOptions)

type workflowOptions struct {
	loggerFactory    workflow.Factory[*slog.Logger]
	statusCollection *activity.StatusHandler
}

// WithLoggerFactory sets a logger factory for creating activity-specific loggers.
func WithLoggerFactory(factory workflow.Factory[*slog.Logger]) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.loggerFactory = factory
	}
}

// WithStatusCollection sets a status collection for tracking activity status.
// If not provided, status updates are only logged.
func WithStatusCollection(collection *activity.StatusHandler) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.statusCollection = collection
	}
}

// WithMetricsRegistry is a no-op option for API consistency with the backup workflow.
// The poweroff workflow doesn't report activity-level metrics.
func WithMetricsRegistry(_ interface{}) WorkflowOption {
	return func(_ *workflowOptions) {
		// No-op - poweroff workflow doesn't use activity metrics
	}
}

// NewWorkflow creates a workflow that gracefully powers off PBS.
// The workflow executes: PowerOffPBS
func NewWorkflow(cfg *config.Config, logger *slog.Logger, opts ...WorkflowOption) (workflow.Workflow, error) {
	// Apply options with defaults
	options := &workflowOptions{
		loggerFactory: workflow.Shared(logger), // Default to shared logger
	}
	for _, opt := range opts {
		opt(options)
	}

	// Create orchestrator with config and logger options
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	// Create IPMI controller directly (no buildDeps needed)
	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	// Register factories for dependencies
	workflow.Provide(o, workflow.Shared(ctrl))

	// Logger factory (per-activity, defaults to shared logger)
	workflow.Provide(o, options.loggerFactory)

	// StatusLine factory (per-activity)
	workflow.Provide(o, func(id workflow.ActivityID) *activity.StatusLine {
		activityLogger := options.loggerFactory(id)
		return activity.NewStatusLine(id, activityLogger, options.statusCollection)
	})

	// Add power off activity
	powerOffPBS := &PowerOffPBS{}

	if err := o.AddActivity(powerOffPBS); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}
