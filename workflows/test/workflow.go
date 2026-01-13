// Package test provides a simple test workflow with 3 sequential activities
// for testing the orchestrator's status reporting and execution flow.
package test

import (
	"log/slog"

	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

// WorkflowOption configures workflow creation.
type WorkflowOption func(*workflowOptions)

type workflowOptions struct {
	loggerFactory    workflow.Factory[*slog.Logger]
	statusCollection *statusreporter.StatusCollection
}

// WithLoggerFactory sets a logger factory for creating activity-specific loggers.
func WithLoggerFactory(factory workflow.Factory[*slog.Logger]) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.loggerFactory = factory
	}
}

// WithStatusCollection sets a status collection for tracking activity status.
// If not provided, status updates are only logged.
func WithStatusCollection(collection *statusreporter.StatusCollection) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.statusCollection = collection
	}
}

// NewWorkflow creates a test workflow with 3 sequential activities.
// Each activity sets status messages and sleeps to simulate work.
func NewWorkflow(logger *slog.Logger, opts ...WorkflowOption) (workflow.Workflow, error) {
	// Apply options with defaults
	options := &workflowOptions{
		loggerFactory: workflow.Shared(logger), // Default to shared logger
	}
	for _, opt := range opts {
		opt(options)
	}

	o := workflow.NewOrchestrator(workflow.WithLogger(logger))

	// Logger factory (per-activity, defaults to shared logger)
	workflow.Provide(o, options.loggerFactory)

	// StatusLine factory (per-activity)
	workflow.Provide(o, func(id workflow.ActivityID) *statusreporter.StatusLine {
		return statusreporter.NewStatusLine(id, logger, options.statusCollection)
	})

	step1 := &Step1{}
	step2 := &Step2{}
	step3 := &Step3{}

	if err := o.AddActivity(step1, step2, step3); err != nil {
		return nil, err
	}

	return o, nil
}
