// Package backup provides workflow factories for backup-related operations.
// It composes activities from the activities package into reusable workflows.
package backup

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nomis52/goback/activities"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/pbsclient"
	"github.com/nomis52/goback/proxmoxclient"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

// NewBackupWorkflow creates a workflow that powers on PBS and performs backups.
// The workflow executes: PowerOnPBS → BackupDirs → BackupVMs
// It does NOT power off PBS after completion.
func NewBackupWorkflow(cfg *config.Config, logger *slog.Logger, statusReporter *statusreporter.StatusReporter) (workflow.Workflow, error) {
	// Create orchestrator
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	// Build and inject dependencies
	deps, err := buildDeps(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

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
	powerOnPBS := &activities.PowerOnPBS{}
	backupDirs := &activities.BackupDirs{}
	backupVMs := &activities.BackupVMs{}

	if err := o.AddActivity(powerOnPBS, backupDirs, backupVMs); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewPowerOffWorkflow creates a workflow that gracefully powers off PBS.
// The workflow executes: PowerOffPBS
func NewPowerOffWorkflow(cfg *config.Config, logger *slog.Logger, statusReporter *statusreporter.StatusReporter) (workflow.Workflow, error) {
	// Create orchestrator
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	// Build and inject dependencies (only need IPMI controller and logger for power off)
	deps, err := buildDeps(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	if err := o.Inject(
		logger,
		deps.ipmiController,
		statusReporter,
	); err != nil {
		return nil, fmt.Errorf("failed to inject dependencies: %w", err)
	}

	// Add power off activity
	powerOffPBS := &activities.PowerOffPBS{}

	if err := o.AddActivity(powerOffPBS); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// deps holds all dependencies that can be injected into workflows.
type deps struct {
	ipmiController *ipmi.IPMIController
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

	ctrl := ipmi.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmi.WithUsername(cfg.PBS.IPMI.Username),
		ipmi.WithPassword(cfg.PBS.IPMI.Password),
		ipmi.WithLogger(logger),
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
