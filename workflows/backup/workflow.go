// Package backup provides workflow factories for backup-related operations.
// It composes activities into a reusable backup workflow.
package backup

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/clients/proxmoxclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
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

	// Metrics client (optional - might be nil)
	if deps.metricsClient != nil {
		workflow.Provide(o, workflow.Shared(deps.metricsClient))
	}

	// Logger factory (per-activity, defaults to shared logger)
	workflow.Provide(o, options.loggerFactory)

	// StatusLine factory (per-activity)
	workflow.Provide(o, func(id workflow.ActivityID) *statusreporter.StatusLine {
		return statusreporter.NewStatusLine(id, logger, options.statusCollection)
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
	metricsClient  *metrics.Client
}

// buildDeps creates all dependencies needed for backup workflows.
func buildDeps(cfg *config.Config, logger *slog.Logger) (*deps, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

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

	metricsClient := metrics.NewClient(
		cfg.Monitoring.VictoriaMetricsURL,
		metrics.WithPrefix(cfg.Monitoring.MetricsPrefix),
		metrics.WithJob(cfg.Monitoring.JobName),
		metrics.WithInstance(hostname),
	)

	return &deps{
		ipmiController: ctrl,
		pbsClient:      pbsClient,
		proxmoxClient:  proxmoxClient,
		metricsClient:  metricsClient,
	}, nil
}
