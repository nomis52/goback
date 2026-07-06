// Package backup provides workflow factories for backup-related operations.
// It composes activities into a reusable backup workflow.
package backup

import (
	"fmt"
	"log/slog"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/clients/proxmoxclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// NewWorkflow creates a workflow that powers on PBS and performs backups.
// The workflow executes: PowerOnPBS → BackupDirs → BackupVMs
// It does NOT power off PBS after completion.
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	// Add backup activities
	if err := o.AddActivity(&PowerOnPBS{}, &BackupDirs{}, &BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewVMsWorkflow creates a workflow that powers on PBS and backs up VMs only,
// skipping directory/file backups.
// The workflow executes: PowerOnPBS → BackupVMs
// It does NOT power off PBS after completion.
func NewVMsWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	// Add backup activities (VMs only, no directory backups)
	if err := o.AddActivity(&PowerOnPBS{}, &BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newBackupOrchestrator builds an orchestrator with the shared backup
// dependencies (IPMI controller, PBS client, Proxmox client) and common
// factories registered, but no activities added.
func newBackupOrchestrator(params workflows.Params) (*workflow.Orchestrator, error) {
	cfg := params.Config
	logger := params.Logger

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

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

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

	pbsClient, err := pbsclient.New(cfg.PBS.Host, pbsclient.WithLogger(logger))
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
