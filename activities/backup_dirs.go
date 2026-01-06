package activities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/sshclient"
)

var (
	ErrMissingSSHConfig = errors.New("missing SSH config: host, user, or private key is not set")
	ErrSSHClientNotInit = errors.New("SSH client not initialized")
	ErrMissingBackupConfig = errors.New("missing backup configuration: token or target")
)

const (
	defaultSSHPort               = ":22"
	pbsConnectivityCheckInterval = 5 * time.Second
	pbsConnectivityMaxRetries    = 6 // 30 seconds total
	metricDirectoryLastBackup    = "directory_last_backup"
	metricDirectoryBackupFailure = "directory_backup_failure"
)

// BackupDirs manages the execution of directory backups on proxmox servers.
// Runs after the PBS server is powered on
type BackupDirs struct {
	// Dependencies
	Logger     *slog.Logger
	PowerOnPBS *PowerOnPBS

	// Configuration
	Files          config.FilesConfig `config:"files"`
	PrivateKeyPath string             `config:"files.private_key_path"`

	// SSH client for remote operations
	sshClient *sshclient.SSHClient

	// Metrics client for pushing metrics
	MetricsClient *metrics.Client
}

func (a *BackupDirs) Init() error {
	host := a.Files.Host
	user := a.Files.User
	privateKeyPath := a.PrivateKeyPath

	if host == "" || user == "" || privateKeyPath == "" {
		return ErrMissingSSHConfig
	}

	// Default to port 22 if not specified
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + defaultSSHPort
	}

	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key file %s: %w", privateKeyPath, err)
	}

	client, err := sshclient.New(host, user, string(privateKeyPEM))
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	a.sshClient = client
	return nil
}

func (a *BackupDirs) Execute(ctx context.Context) error {
	if a.sshClient == nil {
		return ErrSSHClientNotInit
	}

	if err := validateFilesConfig(a.Files); err != nil {
		return err
	}

	if len(a.Files.Sources) == 0 {
		return nil
	}

	// Test PBS connectivity before attempting backup
	if err := a.waitForPBSConnectivity(ctx); err != nil {
		a.Logger.Error("PBS not accessible for backup", "error", err)
		return err
	}

	stdout, stderr, err := a.backupAllDirs(a.Files.Sources)
	if err != nil {
		a.Logger.Error("Backup failed", "sources", a.Files.Sources, "error", err, "stderr", stderr)
		return err
	}
	a.Logger.Info("Backup succeeded", "sources", a.Files.Sources, "stdout", stdout)
	if stderr != "" {
		a.Logger.Warn("Backup stderr", "sources", a.Files.Sources, "stderr", stderr)
	}

	return nil
}

// backupAllDirs executes a single backup command with all sources combined
// This enables PBS deduplication across all directories
func (a *BackupDirs) backupAllDirs(sources []string) (string, string, error) {
	token := a.Files.Token
	target := a.Files.Target

	// Build the command with all sources in a single backup command
	cmd := buildBackupCommand(token, target, sources)

	a.Logger.Info("Running consolidated backup command", "command", cmd, "source_count", len(sources))

	stdout, stderr, err := a.sshClient.Run(cmd)

	// Push metrics if MetricsClient is set
	if a.MetricsClient != nil {
		// Create a consolidated metric for all sources
		labels := make(map[string]string, len(sources)+1)
		labels["target"] = target
		// Add source count to labels
		labels["source_count"] = fmt.Sprintf("%d", len(sources))

		var metricName string
		var metricValue float64
		if err != nil {
			metricName = metricDirectoryBackupFailure
			metricValue = 1
		} else {
			metricName = metricDirectoryLastBackup
			metricValue = float64(time.Now().Unix())
		}
		metric := metrics.Metric{
			Name:      metricName,
			Value:     metricValue,
			Labels:    labels,
			Timestamp: time.Now(),
		}
		if pushErr := a.MetricsClient.PushMetrics(context.Background(), metric); pushErr != nil {
			a.Logger.Error("Failed to push backup metric", "error", pushErr)
		}
	}

	return stdout, stderr, err
}

// buildBackupCommand constructs the PBS backup command with all sources
func buildBackupCommand(token, target string, sources []string) string {
	cmd := "export PBS_PASSWORD='" + token + "' && proxmox-backup-client backup"
	for _, source := range sources {
		cmd += " " + source
	}
	cmd += " --repository '" + target + "'"
	return cmd
}

// validateFilesConfig validates the file backup configuration
func validateFilesConfig(config config.FilesConfig) error {
	if config.Token == "" || config.Target == "" {
		return ErrMissingBackupConfig
	}
	return nil
}

// waitForPBSConnectivity tests if PBS is reachable from the remote host before starting backup
func (a *BackupDirs) waitForPBSConnectivity(ctx context.Context) error {
	// Extract hostname from target (format: user@host!datastore@hostname:port)
	target := a.Files.Target
	pbsHost := extractPBSHostFromTarget(target)
	if pbsHost == "" {
		return fmt.Errorf("could not extract PBS host from target: %s", target)
	}

	a.Logger.Info("Testing PBS connectivity", "pbs_host", pbsHost, "max_retries", pbsConnectivityMaxRetries)

	for attempt := 1; attempt <= pbsConnectivityMaxRetries; attempt++ {
		// Test connectivity using a simple nc (netcat) command
		cmd := fmt.Sprintf("nc -z -w5 %s 8007 2>/dev/null", pbsHost)
		_, _, err := a.sshClient.Run(cmd)
		if err == nil {
			a.Logger.Info("PBS connectivity test successful", "pbs_host", pbsHost, "attempts", attempt)
			return nil
		}

		a.Logger.Warn("PBS connectivity test failed, retrying", "pbs_host", pbsHost, "attempt", attempt, "max_retries", pbsConnectivityMaxRetries, "error", err)

		if attempt < pbsConnectivityMaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pbsConnectivityCheckInterval):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("PBS not reachable after %d attempts", pbsConnectivityMaxRetries)
}

// extractPBSHostFromTarget extracts the PBS hostname from a target string
// Format: user@host!datastore@hostname:port -> hostname
func extractPBSHostFromTarget(target string) string {
	// Find the @ after the !
	if exclamationIdx := strings.Index(target, "!"); exclamationIdx != -1 {
		afterExclamation := target[exclamationIdx+1:]
		if atIdx := strings.Index(afterExclamation, "@"); atIdx != -1 {
			hostPart := afterExclamation[atIdx+1:]
			// Remove port if present
			if colonIdx := strings.Index(hostPart, ":"); colonIdx != -1 {
				return hostPart[:colonIdx]
			}
			return hostPart
		}
	}
	return ""
}
