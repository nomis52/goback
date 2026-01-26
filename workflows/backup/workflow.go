// Package backup provides workflow factories for backup-related operations.
// It composes activities into a reusable backup workflow.
package backup

import (
	"fmt"
	"log/slog"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/clients/proxmoxclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
)

// WorkflowOption configures workflow creation.
type WorkflowOption func(*workflowOptions)

type workflowOptions struct {
	loggerFactory    workflow.Factory[*slog.Logger]
	statusCollection *activity.StatusHandler
	registry         metrics.Registry
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

// WithMetricsRegistry sets the metrics registry for activities.
// If not provided, activities will not report metrics.
func WithMetricsRegistry(registry metrics.Registry) WorkflowOption {
	return func(opts *workflowOptions) {
		opts.registry = registry
	}
}

// NewWorkflow creates a workflow that powers on PBS and performs backups.
// The workflow executes: PowerOnPBS → BackupDirs → BackupVMs
// It does NOT power off PBS after completion.
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

	// Build shared dependencies
	deps, err := buildDeps(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// Register factories for shared dependencies
	workflow.Provide(o, workflow.Shared(deps.ipmiController))
	workflow.Provide(o, workflow.Shared(deps.pbsClient))
	workflow.Provide(o, workflow.Shared(deps.proxmoxClient))

	// Metrics registry (optional - activities check for nil)
	if options.registry != nil {
		workflow.Provide(o, workflow.Shared(options.registry))
	}

	// Logger factory (per-activity, defaults to shared logger)
	workflow.Provide(o, options.loggerFactory)

	// StatusLine factory (per-activity)
	workflow.Provide(o, func(id workflow.ActivityID) *activity.StatusLine {
		activityLogger := options.loggerFactory(id)
		return activity.NewStatusLine(id, activityLogger, options.statusCollection)
	})

	// Add backup activities
	powerOnPBS := &PowerOnPBS{}
	backupDirs := &BackupDirs{}
	backupVMs := &BackupVMs{}

	if err := o.AddActivity(powerOnPBS, backupDirs, backupVMs); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// deps holds all dependencies that can be injected into workflows.
type deps struct {
	ipmiController *ipmiclient.IPMIController
	pbsClient      *pbsclient.Client
	proxmoxClient  *proxmoxclient.Client
}

// buildDeps creates all dependencies needed for backup workflows.
func buildDeps(cfg *config.Config, logger *slog.Logger) (*deps, error) {
	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	pbsClient, err := pbsclient.New(cfg.PBS.Host, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PBS client: %w", err)
	}

	proxmoxClient, err := proxmoxclient.New(cfg.Proxmox.Host, proxmoxclient.WithToken(cfg.Proxmox.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	return &deps{
		ipmiController: ctrl,
		pbsClient:      pbsClient,
		proxmoxClient:  proxmoxClient,
	}, nil
}
