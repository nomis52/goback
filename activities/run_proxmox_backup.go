package activities

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/proxmoxclient"
)

// RunProxmoxBackup manages the execution of Proxmox backups
type RunProxmoxBackup struct {
	// Dependencies
	ProxmoxClient *proxmoxclient.Client
	Logger        *slog.Logger

	BackupTimeout time.Duration `config:"proxmox.backup_timeout"`
}

func (a *RunProxmoxBackup) Init() error {
	return nil
}

func (a *RunProxmoxBackup) Run(ctx context.Context) (orchestrator.Result, error) {
	// TODO: Implement backup logic
	// This will need to:
	// 1. Check if a backup is already running
	// 2. Start the backup if none is running
	// 3. Monitor the backup progress
	// 4. Return success when backup completes or failure if it fails

	return orchestrator.NewSuccessResult(), nil
}
