package activities

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/proxmoxclient"
)

// RunProxmoxBackup manages the execution of Proxmox backups
type RunProxmoxBackup struct {
	// Dependencies
	ProxmoxClient *proxmoxclient.Client
	Logger        *slog.Logger
	PowerOnPBS    *PowerOnPBS

	// Configuration
	BackupTimeout time.Duration `config:"proxmox.backup_timeout"`
}

func (a *RunProxmoxBackup) Init() error {
	return nil
}

func (a *RunProxmoxBackup) Execute(ctx context.Context) error {
	// Get and log Proxmox version
	version, err := a.ProxmoxClient.Version()
	if err != nil {
		a.Logger.Error("Failed to get Proxmox version", "error", err)
		return err
	}
	a.Logger.Info("Proxmox version", "version", version)

	// Get list of Compute resources: VMs, LXCs
	resources, err := a.ProxmoxClient.GetResources(ctx)
	if err != nil {
		a.Logger.Error("Failed to get list of resources", "error", err)
		return err
	}
	a.Logger.Info("Found resources", "count", len(vms))
	for _, resource := range resources {
		a.Logger.Info("VM details",
			"vmid", vm.VMID,
			"name", vm.Name,
			"node", vm.Node,
			"status", vm.Status,
			"type", vm.Type)
	}

	return nil // Success!
}
