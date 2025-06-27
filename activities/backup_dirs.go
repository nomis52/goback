package activities

import (
	"context"
	"errors"
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

	token := a.Directories.Token
	target := a.Directories.Target
	sources := a.Directories.Sources

	if token == "" || target == "" || len(sources) == 0 {
		a.Logger.Error("Missing backup configuration", "token", token, "target", target, "sources", sources)
		return errors.New("missing backup configuration: token, target, or sources")
	}

	var firstErr error
	for _, source := range sources {
		stdout, stderr, err := a.backupDir(source)
		if err != nil {
			a.Logger.Error("Backup failed", "source", source, "error", err, "stderr", stderr)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		a.Logger.Info("Backup succeeded", "source", source, "stdout", stdout)
		if stderr != "" {
			a.Logger.Warn("Backup stderr", "source", source, "stderr", stderr)
		}
	}

	return firstErr
}

func (a *BackupDirs) backupDir(source string) (string, string, error) {
	token := a.Directories.Token
	target := a.Directories.Target

	cmd := "export PBS_PASSWORD='" + token + "' && proxmox-backup-client backup " + source + " --repository '" + target + "'"
	a.Logger.Info("Running backup command", "command", cmd)

	stdout, stderr, err := a.sshClient.Run(cmd)

	// Push metrics if MetricsClient is set
	if a.MetricsClient != nil {
		labels := map[string]string{
			"source": source,
			"target": target,
		}
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
