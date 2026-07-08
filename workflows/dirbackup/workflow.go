// Package dirbackup provides a workflow factory that powers on PBS, backs up
// directories only, then powers PBS back off.
//
// It composes the directories-only backup workflow (from the backup package)
// with the power-off workflow (from the poweroff package). Power-off runs even
// if the directory backup fails, because workflow.Compose executes composed
// workflows in sequence and continues on failure.
package dirbackup

import (
	"fmt"

	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
	"github.com/nomis52/goback/workflows/backup"
	"github.com/nomis52/goback/workflows/poweroff"
)

// NewWorkflow creates a workflow that powers on PBS, backs up directories only,
// then powers PBS off.
// The workflow executes: (PowerOnPBS → BackupDirs) then PowerOffPBS.
// Power-off runs even if the directory backup fails.
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	dirBackup, err := backup.NewDirsWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory backup workflow: %w", err)
	}

	powerOff, err := poweroff.NewWorkflow(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create power off workflow: %w", err)
	}

	return workflow.Compose(dirBackup, powerOff), nil
}
