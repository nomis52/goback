// Package backup provides workflow factories for the backup workloads: backing up
// Proxmox VMs and/or directories to PBS.
//
// These workflows perform work only — they do NOT power PBS on or off. Powering PBS
// on beforehand and off afterwards is handled by the separate poweron/poweroff
// workflows, composed ahead of and after the backup work by the cron config or a
// manual UI run (the runner executes a requested list of workflows in sequence).
// Keeping the backup workflows power-agnostic lets operators mix and match power and
// backup steps from config without a combinatorial explosion of workflow variants.
//
// All three factories assume PBS is already powered on.
package backup

import (
	"fmt"

	"github.com/nomis52/goback/clients/proxmoxclient"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// NewCombinedWorkflow creates the "combined-backup" workflow: back up VMs and
// directories concurrently. The two workloads have no dependency between them, so
// the orchestrator runs them at the same time. This is the one backup workflow that
// must exist as a single workflow rather than being composed from config, because
// config-level composition runs steps strictly in sequence.
// The workflow executes: {BackupVMs ∥ BackupDirs}.
func NewCombinedWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&BackupVMs{}, &BackupDirs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewComputeWorkflow creates the "compute-backup" workflow: back up VMs only.
// The workflow executes: BackupVMs.
func NewComputeWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&BackupVMs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewDirsWorkflow creates the "dir-backup" workflow: back up directories only.
// The workflow executes: BackupDirs.
func NewDirsWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newBackupOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&BackupDirs{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newBackupOrchestrator builds an orchestrator with the shared backup dependency
// (the Proxmox client) and common factories registered, but no activities added.
// BackupDirs creates its own SSH client from config, so the Proxmox client is the
// only shared dependency the backup activities need.
func newBackupOrchestrator(params workflows.Params) (*workflow.Orchestrator, error) {
	cfg := params.Config
	logger := params.Logger

	// Create orchestrator with config and logger options
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	proxmoxClient, err := proxmoxclient.New(cfg.Proxmox.Host, proxmoxclient.WithToken(cfg.Proxmox.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Register factory for the shared dependency
	workflow.Provide(o, workflow.Shared(proxmoxClient))

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

	return o, nil
}
