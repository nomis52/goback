package activities

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/proxmoxclient"
)

const (
	backupStatusCheckInterval = 10 * time.Second
	metricLastBackup          = "last_backup"
	metricBackupFailure       = "backup_failure"
)

// BackupVMs manages the execution of Proxmox backups
type BackupVMs struct {
	// Dependencies
	ProxmoxClient *proxmoxclient.Client
	Logger        *slog.Logger
	PowerOnPBS    *PowerOnPBS
	MetricsClient *metrics.Client

	// Configuration
	BackupTimeout time.Duration `config:"proxmox.backup_timeout"`
	Storage       string        `config:"proxmox.storage"`
	MaxBackupAge  time.Duration `config:"compute.max_backup_age"`
	Mode          string        `config:"compute.mode"`
	Compress      string        `config:"compute.compress"`
}

func (a *BackupVMs) Init() error {
	return nil
}

func (a *BackupVMs) Execute(ctx context.Context) error {
	// Get and log Proxmox version
	version, err := a.ProxmoxClient.Version()
	if err != nil {
		a.Logger.Error("Failed to get Proxmox version", "error", err)
		return err
	}
	a.Logger.Info("Proxmox version", "version", version)

	// Determine which resources need to be backed up
	resourcesToBackup, err := a.determineBackups(ctx)
	if err != nil {
		a.Logger.Error("Failed to determine resources to backup", "error", err)
		return err
	}
	a.Logger.Info("Resources that need backup", "count", len(resourcesToBackup))

	// Use wait group to track all backup operations
	var wg sync.WaitGroup
	errChan := make(chan error, len(resourcesToBackup))

	// Launch backup operations concurrently
	for _, resource := range resourcesToBackup {
		wg.Add(1)
		go func(r proxmoxclient.Resource) {
			defer wg.Done()
			if err := a.performBackupWithMetrics(ctx, r); err != nil {
				a.Logger.Error("Failed to perform backup",
					"vmid", r.VMID,
					"name", r.Name,
					"node", r.Node,
					"error", err)
				errChan <- fmt.Errorf("backup failed for VMID %d: %w", r.VMID, err)
			}
		}(resource)
	}

	// Wait for all backups to complete
	wg.Wait()
	close(errChan)

	// Collect any errors that occurred
	errors := make([]error, 0)
	for err := range errChan {
		errors = append(errors, err)
	}

	// If any errors occurred, return a combined error
	if len(errors) > 0 {
		errMsg := fmt.Sprintf("%d backup(s) failed:", len(errors))
		for _, err := range errors {
			errMsg += "\n  - " + err.Error()
		}
		return fmt.Errorf(errMsg)
	}

	return nil // All backups completed successfully!
}

// performBackupWithMetrics wraps performBackup and pushes metrics based on the result.
func (a *BackupVMs) performBackupWithMetrics(ctx context.Context, resource proxmoxclient.Resource) error {
	err := a.performBackup(ctx, resource)
	if a.MetricsClient != nil {
		labels := map[string]string{
			"vmid": fmt.Sprintf("%d", resource.VMID),
			"name": resource.Name,
		}
		var metric metrics.Metric
		if err != nil {
			metric = metrics.Metric{
				Name:      metricBackupFailure,
				Value:     1,
				Labels:    labels,
				Timestamp: time.Now(),
			}
		} else {
			metric = metrics.Metric{
				Name:      metricLastBackup,
				Value:     float64(time.Now().Unix()),
				Labels:    labels,
				Timestamp: time.Now(),
			}
		}
		if pushErr := a.MetricsClient.PushMetrics(ctx, metric); pushErr != nil {
			a.Logger.Error("Failed to push backup metric", "error", pushErr)
		}
	}
	return err
}

// performBackup initiates a backup for a given resource and waits for it to complete.
// It returns an error if the backup fails or times out.
func (a *BackupVMs) performBackup(ctx context.Context, resource proxmoxclient.Resource) error {
	// Build backup options based on configuration
	var backupOpts []proxmoxclient.BackupOption
	if a.Mode != "" {
		backupOpts = append(backupOpts, proxmoxclient.WithMode(a.Mode))
	}
	if a.Compress != "" {
		backupOpts = append(backupOpts, proxmoxclient.WithCompress(a.Compress))
	}

	// Start the backup
	a.Logger.Info("Starting backup for resource",
		"vmid", resource.VMID,
		"name", resource.Name,
		"node", resource.Node,
		"storage", a.Storage,
		"mode", a.Mode,
		"compress", a.Compress)

	taskID, err := a.ProxmoxClient.Backup(ctx, resource.Node, resource.VMID, a.Storage, backupOpts...)
	if err != nil {
		a.Logger.Error("Failed to start backup",
			"vmid", resource.VMID,
			"name", resource.Name,
			"node", resource.Node,
			"mode", a.Mode,
			"compress", a.Compress,
			"error", err)
		return err
	}

	// Poll for task completion
	ticker := time.NewTicker(backupStatusCheckInterval)
	defer ticker.Stop()

	timeout := time.After(a.BackupTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("backup timed out after %v for VMID %d", a.BackupTimeout, resource.VMID)
		case <-ticker.C:
			status, err := a.ProxmoxClient.TaskStatus(ctx, resource.Node, taskID)
			if err != nil {
				a.Logger.Error("Failed to get task status",
					"vmid", resource.VMID,
					"name", resource.Name,
					"node", resource.Node,
					"task_id", taskID,
					"error", err)
				return err
			}

			a.Logger.Debug("Backup task status",
				"vmid", resource.VMID,
				"name", resource.Name,
				"node", resource.Node,
				"task_id", taskID,
				"status", status.Status,
				"exit_status", status.ExitStatus)

			// Check if task is complete
			if status.Status == "stopped" {
				if status.ExitStatus != "OK" {
					return fmt.Errorf("backup failed with exit status: %s", status.ExitStatus)
				}
				a.Logger.Info("Backup completed successfully",
					"vmid", resource.VMID,
					"name", resource.Name,
					"node", resource.Node,
					"task_id", taskID)
				return nil
			}
		}
	}
}

// determineBackups analyzes resources and their backup status to decide which ones need backing up.
// It returns the resources that need to be backed up.
func (a *BackupVMs) determineBackups(ctx context.Context) ([]proxmoxclient.Resource, error) {
	resources, err := a.ProxmoxClient.ListComputeResources(ctx)
	if err != nil {
		a.Logger.Error("Failed to get list of resources", "error", err)
		return nil, err
	}

	backups, err := a.ProxmoxClient.ListBackups(ctx, a.ProxmoxClient.Host(), a.Storage)
	if err != nil {
		a.Logger.Error("Failed to get list of backups", "error", err)
		return nil, err
	}

	resourceMap := make(map[proxmoxclient.VMID]proxmoxclient.Resource, len(resources))
	for _, resource := range resources {
		resourceMap[resource.VMID] = resource
	}

	var resourcesToBackup []proxmoxclient.Resource
	for vmID, lastBackup := range getMostRecentBackupTimes(backups, resources) {
		if lastBackup.IsZero() || time.Since(lastBackup) > a.MaxBackupAge {
			if resource, exists := resourceMap[vmID]; exists {
				resourcesToBackup = append(resourcesToBackup, resource)
			}
		}
	}

	return resourcesToBackup, nil
}

// getMostRecentBackupTimes returns a map of VMID to the most recent backup time.
// If a resource has no backups, it returns the zero time (time.Time{}).
func getMostRecentBackupTimes(backups []proxmoxclient.Backup, resources []proxmoxclient.Resource) map[proxmoxclient.VMID]time.Time {
	result := make(map[proxmoxclient.VMID]time.Time, len(resources))

	for _, r := range resources {
		result[r.VMID] = time.Time{}
	}

	for _, backup := range backups {
		if current, exists := result[backup.VMID]; exists && (current.IsZero() || backup.CTime.After(current)) {
			result[backup.VMID] = backup.CTime
		}
	}

	return result
}
