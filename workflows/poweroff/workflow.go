// Package poweroff provides workflow factories for power-off operations.
// It orchestrates the graceful shutdown of the PBS server via IPMI.
package poweroff

import (
	"fmt"
	"log/slog"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

// WorkflowOption configures workflow creation.
type WorkflowOption func(*workflowOptions)

type workflowOptions struct {
	loggerHook logging.LoggerHook
}

// WithLoggerHook sets a logger hook for capturing activity logs.
func WithLoggerHook(hook logging.LoggerHook) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.loggerHook = hook
	}
}

// NewWorkflow creates a workflow that gracefully powers off PBS.
// The workflow executes: PowerOffPBS
func NewWorkflow(cfg *config.Config, logger *slog.Logger, statusReporter *statusreporter.StatusReporter, opts ...WorkflowOption) (workflow.Workflow, error) {
	// Apply options
	options := &workflowOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Create orchestrator options
	orchOpts := []workflow.OrchestratorOption{
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	}
	if options.loggerHook != nil {
		orchOpts = append(orchOpts, workflow.WithLogHook(options.loggerHook))
	}

	// Create orchestrator
	o := workflow.NewOrchestrator(orchOpts...)

	// Create IPMI controller directly (no buildDeps needed)
	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	// Inject dependencies
	if err := o.Inject(logger, ctrl, statusReporter); err != nil {
		return nil, fmt.Errorf("failed to inject dependencies: %w", err)
	}

	// Add power off activity
	powerOffPBS := &PowerOffPBS{}

	if err := o.AddActivity(powerOffPBS); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}
