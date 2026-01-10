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
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/metrics"
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

// NewWorkflow creates a workflow that powers on PBS and performs backups.
// The workflow executes: PowerOnPBS → BackupDirs → BackupVMs
// It does NOT power off PBS after completion.
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

	// Build and inject dependencies
	deps, err := buildDeps(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// Inject dependencies (logger will be wrapped by LoggerHook if provided)
	if err := o.Inject(
		logger,
		deps.metricsClient,
		deps.ipmiController,
		deps.pbsClient,
		deps.proxmoxClient,
		statusReporter,
	); err != nil {
		return nil, fmt.Errorf("failed to inject dependencies: %w", err)
	}

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
