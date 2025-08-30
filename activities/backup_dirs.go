package activities

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/sshclient"
)

var ErrMissingSSHConfig = errors.New("missing SSH config: host, user, or private key is not set")

const (
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
	Directories    config.DirectoryConfig `config:"directories"`
	PrivateKeyPath string                 `config:"directories.private_key_path"`

	// SSH client for remote operations
	sshClient *sshclient.SSHClient

	// Metrics client for pushing metrics
	MetricsClient *metrics.Client
}

func (a *BackupDirs) Init() error {
	host := a.Directories.Host
	user := a.Directories.User
	privateKeyPath := a.PrivateKeyPath

	if host == "" || user == "" || privateKeyPath == "" {
		return ErrMissingSSHConfig
	}

	// Default to port 22 if not specified
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":22"
	}

	privateKeyPEM, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return err
	}

	client, err := sshclient.New(host, user, string(privateKeyPEM))
	if err != nil {
		return err
	}
	a.sshClient = client
	return nil
}

func (a *BackupDirs) Execute(ctx context.Context) error {
	if a.sshClient == nil {
		return errors.New("SSH client not initialized")
	}

	if err := validateDirectoryConfig(a.Directories); err != nil {
		return err
	}

	if len(a.Directories.Sources) == 0 {
		return nil
	}

	stdout, stderr, err := a.backupAllDirs(a.Directories.Sources)
	if err != nil {
		a.Logger.Error("Backup failed", "sources", a.Directories.Sources, "error", err, "stderr", stderr)
		return err
	}
	a.Logger.Info("Backup succeeded", "sources", a.Directories.Sources, "stdout", stdout)
	if stderr != "" {
		a.Logger.Warn("Backup stderr", "sources", a.Directories.Sources, "stderr", stderr)
	}

	return nil
}

// backupAllDirs executes a single backup command with all sources combined
// This enables PBS deduplication across all directories
func (a *BackupDirs) backupAllDirs(sources []string) (string, string, error) {
	token := a.Directories.Token
	target := a.Directories.Target

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

// validateDirectoryConfig validates the directory backup configuration
func validateDirectoryConfig(config config.DirectoryConfig) error {
	if config.Token == "" || config.Target == "" {
		return errors.New("missing backup configuration: token or target")
	}
	return nil
}
