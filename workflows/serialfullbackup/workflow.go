// Package serialfullbackup provides a workflow factory that powers on PBS, backs
// up VMs, then backs up directories, then powers PBS back off.
//
// Unlike the backup package's NewWorkflow (which adds BackupVMs and BackupDirs to
// a single orchestrator, where they run concurrently and contend for the same pool
// IO), this workflow composes the VM and directory backups as separate sub-workflows
// so they run strictly one after the other. workflow.Compose executes composed
// workflows in sequence, so the directory backup starts only after the VM backups
// finish, and power-off runs even if a backup step fails.
package serialfullbackup

import (
	"fmt"

	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
	"github.com/nomis52/goback/workflows/backup"
	"github.com/nomis52/goback/workflows/poweroff"
)

// NewWorkflow creates a workflow that powers on PBS, backs up VMs, then backs up
// directories (serialised so the two workloads never overlap), then powers PBS off.
// The workflow executes: (PowerOnPBS → BackupVMs) then (PowerOnPBS → BackupDirs)
// then PowerOffPBS. Power-off runs even if a backup step fails.
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	vmBackup, err := backup.NewVMsWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM backup workflow: %w", err)
	}

	dirBackup, err := backup.NewDirsWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory backup workflow: %w", err)
	}

	powerOff, err := poweroff.NewWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create power off workflow: %w", err)
	}

	return workflow.Compose(vmBackup, dirBackup, powerOff), nil
}
