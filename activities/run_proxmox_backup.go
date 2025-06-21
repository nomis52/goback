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
	Storage       string        `config:"proxmox.storage"`
	MaxAge        time.Duration `config:"backup.max_age"`
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
	resources, err := a.ProxmoxClient.ListComputeResources(ctx)
	if err != nil {
		a.Logger.Error("Failed to get list of resources", "error", err)
		return err
	}
	a.Logger.Info("Found resources", "count", len(resources))
	for _, resource := range resources {
		a.Logger.Info("Resource details",
			"vmid", resource.VMID,
			"name", resource.Name,
			"node", resource.Node,
			"status", resource.Status,
			"type", resource.Type)
	}

	// Get list of backups from storage
	a.Logger.Info("Querying backups", "node", "pve2", "storage", a.Storage)
	backups, err := a.ProxmoxClient.ListBackups(ctx, a.ProxmoxClient.Host(), a.Storage)
	if err != nil {
		a.Logger.Error("Failed to get list of backups", "error", err)
		return err
	}
	a.Logger.Info("Found backups", "count", len(backups))

	// getMostRecentBackupTimes returns a map of VMID to the most recent backup time
	// If a resource has no backups, it returns the zero time (time.Time{})
	mostRecentBackupTimes := getMostRecentBackupTimes(backups, resources)

	// Log the backup status for each resource and check for old backups
	a.Logger.Info("max age", "max_age", a.MaxAge)
	for vmID, lastBackup := range mostRecentBackupTimes {
		if lastBackup.IsZero() {
			a.Logger.Warn("Resource has no backups", "vmid", vmID)
		} else {
			// Check if backup is older than MaxAge
			if time.Since(lastBackup) > a.MaxAge {
				a.Logger.Warn("Resource backup is older than MaxAge", "vmid", vmID, "age", time.Since(lastBackup))
			}
		}
	}

	return nil

	// Backup the first VMID if there are any resources
	if len(resources) > 0 {
		firstResource := resources[0]
		a.Logger.Info("Starting backup for first resource",
			"vmid", firstResource.VMID,
			"name", firstResource.Name,
			"node", firstResource.Node,
			"storage", a.Storage)

		taskID, err := a.ProxmoxClient.Backup(ctx, firstResource.Node, firstResource.VMID, a.Storage)
		if err != nil {
			a.Logger.Error("Failed to start backup",
				"vmid", firstResource.VMID,
				"name", firstResource.Name,
				"node", firstResource.Node,
				"error", err)
			return err
		}

		a.Logger.Info("Backup started successfully",
			"vmid", firstResource.VMID,
			"name", firstResource.Name,
			"node", firstResource.Node,
			"task_id", taskID)
	}

	return nil // Success!
}

// getMostRecentBackupTimes returns a map of VMID to the most recent backup time.
// If a resource has no backups, it returns the zero time (time.Time{}).
func getMostRecentBackupTimes(backups []proxmoxclient.Backup, resources []proxmoxclient.Resource) map[proxmoxclient.VMID]time.Time {
	result := make(map[proxmoxclient.VMID]time.Time)

	// Initialize all resources with zero time
	for _, r := range resources {
		result[r.VMID] = time.Time{}
	}

	// Find the most recent backup for each VMID
	for _, backup := range backups {
		if current, exists := result[backup.VMID]; exists && (current.IsZero() || backup.CTime.After(current)) {
			result[backup.VMID] = backup.CTime
		}
	}

	return result
}
