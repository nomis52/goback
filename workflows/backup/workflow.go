// Package backup provides workflow factories for the backup workloads: backing up
// Proxmox VMs and/or directories to PBS.
//
// Each backup workflow powers PBS on first (via a PowerOnPBS dependency) and then
// runs its backup work. PowerOnPBS is a no-op when PBS is already on, so composing
// several backup workflows in one run is cheap, and — because the backup activities
// depend on PowerOnPBS — a power-on failure Skips the backup work rather than letting
// it grind against an unavailable PBS.
//
// These workflows do NOT power PBS off. Powering off is handled by the separate
// poweroff workflow, stacked after the backup by the cron config or a manual UI run.
// Power-off is kept separate (rather than a dependency) so it still runs even when the
// backup fails: the runner composes the requested workflows with continue-on-failure
// semantics, whereas an in-orchestrator dependent would be Skipped on failure.
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
	"github.com/nomis52/goback/workflows/poweron"
)

// NewCombinedWorkflow creates the "combined-backup" workflow: power on PBS, then back
// up VMs and directories concurrently. The two backup workloads depend only on
// PowerOnPBS (not on each other), so they run at the same time. This is the one
// backup workflow that must exist as a single workflow rather than being composed
// from config, because config-level composition runs steps strictly in sequence.
// The workflow executes: PowerOnPBS → {BackupVMs ∥ BackupDirs}.
func NewCombinedWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&poweron.PowerOnPBS{}, &BackupVMs{}, &BackupDirs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewSequentialWorkflow creates the "sequential-backup" workflow: power on PBS, then back
// up VMs, then back up directories — one after another rather than concurrently (as
// combined-backup does). It composes the compute and dirs workflows, so each self-powers-on
// (the second power-on is a no-op) and, per Compose's continue-on-failure semantics, the
// directory backup still runs even if the VM backup fails.
// The workflow executes: (PowerOnPBS → BackupVMs) then (PowerOnPBS → BackupDirs).
func NewSequentialWorkflow(params workflows.Params) (workflow.Workflow, error) {
	compute, err := NewComputeWorkflow(params)
	if err != nil {
		return nil, err
	}

	dirs, err := NewDirsWorkflow(params)
	if err != nil {
		return nil, err
	}

	return workflow.Compose(compute, dirs), nil
}

// NewComputeWorkflow creates the "compute-backup" workflow: power on PBS, then back
// up VMs only.
// The workflow executes: PowerOnPBS → BackupVMs.
func NewComputeWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&poweron.PowerOnPBS{}, &BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewDirsWorkflow creates the "dir-backup" workflow: power on PBS, then back up
// directories only.
// The workflow executes: PowerOnPBS → BackupDirs.
func NewDirsWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&poweron.PowerOnPBS{}, &BackupDirs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newBackupOrchestrator builds an orchestrator with the shared backup dependencies
// (IPMI controller and PBS client for PowerOnPBS, Proxmox client for BackupVMs) and
// common factories registered, but no activities added. BackupDirs creates its own
// SSH client from config.
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

// deps holds the shared dependencies injected into the backup activities.
type deps struct {
	ipmiController *ipmiclient.IPMIController
	pbsClient      *pbsclient.Client
	proxmoxClient  *proxmoxclient.Client
}

// buildDeps creates the dependencies needed for the backup workflows: the IPMI
// controller and PBS client used by PowerOnPBS, and the Proxmox client used by
// BackupVMs.
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
