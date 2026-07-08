// Package backup provides workflow factories for backup-related operations.
// It composes activities into reusable backup workflows.
//
// Every public factory guarantees the power-on → do work → power-off cycle:
// the work orchestrators include PowerOnPBS, and withPowerOff appends a
// PowerOffPBS step that always runs (even if the work fails). Routing all
// factories through withPowerOff makes the power cycle structural, so a backup
// workflow cannot be defined that forgets to power PBS back off.
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
	"github.com/nomis52/goback/workflows/poweroff"
)

// NewFullConcurrentWorkflow creates the "backup-full-concur" workflow: power on PBS,
// back up VMs and directories concurrently, then power PBS back off.
// The workflow executes: PowerOnPBS → {BackupVMs ∥ BackupDirs} → PowerOffPBS.
// Power-off runs even if a backup fails.
func NewFullConcurrentWorkflow(params workflows.Params) (workflow.Workflow, error) {
	work, err := newConcurrentWork(params)
	if err != nil {
		return nil, err
	}
	return withPowerOff(params, work)
}

// NewFullSequentialWorkflow creates the "backup-full-seq" workflow: power on PBS,
// back up VMs, then back up directories (serialised so the two workloads never
// overlap and contend for pool IO), then power PBS back off.
// The workflow executes: (PowerOnPBS → BackupVMs) then (PowerOnPBS → BackupDirs)
// then PowerOffPBS. Power-off runs even if a backup fails.
func NewFullSequentialWorkflow(params workflows.Params) (workflow.Workflow, error) {
	vmWork, err := newVMsWork(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM backup workflow: %w", err)
	}

	dirWork, err := newDirsWork(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory backup workflow: %w", err)
	}

	return withPowerOff(params, workflow.Compose(vmWork, dirWork))
}

// NewComputeWorkflow creates the "backup-compute" workflow: power on PBS, back up
// VMs only, then power PBS back off.
// The workflow executes: PowerOnPBS → BackupVMs → PowerOffPBS.
// Power-off runs even if the VM backup fails.
func NewComputeWorkflow(params workflows.Params) (workflow.Workflow, error) {
	work, err := newVMsWork(params)
	if err != nil {
		return nil, err
	}
	return withPowerOff(params, work)
}

// NewDirsWorkflow creates the "backup-dirs" workflow: power on PBS, back up
// directories only, then power PBS back off.
// The workflow executes: PowerOnPBS → BackupDirs → PowerOffPBS.
// Power-off runs even if the directory backup fails.
func NewDirsWorkflow(params workflows.Params) (workflow.Workflow, error) {
	work, err := newDirsWork(params)
	if err != nil {
		return nil, err
	}
	return withPowerOff(params, work)
}

// withPowerOff wraps a work workflow so PBS is always powered off afterwards, even
// if the work fails (workflow.Compose runs sub-workflows in sequence and continues
// on failure). Power-on is performed by the work workflow's own PowerOnPBS activity.
func withPowerOff(params workflows.Params, work workflow.Workflow) (workflow.Workflow, error) {
	powerOff, err := poweroff.NewWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create power off workflow: %w", err)
	}
	return workflow.Compose(work, powerOff), nil
}

// newConcurrentWork builds the work orchestrator that powers on PBS and backs up
// both VMs and directories. BackupDirs and BackupVMs both depend only on PowerOnPBS,
// so they run concurrently. This is work only — it does NOT power off PBS.
func newConcurrentWork(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&PowerOnPBS{}, &BackupDirs{}, &BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newVMsWork builds the work orchestrator that powers on PBS and backs up VMs only.
// This is work only — it does NOT power off PBS.
func newVMsWork(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&PowerOnPBS{}, &BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newDirsWork builds the work orchestrator that powers on PBS and backs up
// directories only. This is work only — it does NOT power off PBS.
func newDirsWork(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&PowerOnPBS{}, &BackupDirs{}); err != nil {
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
