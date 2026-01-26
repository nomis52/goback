package backup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/sshclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
)

var (
	ErrMissingSSHConfig    = errors.New("missing SSH config: host, user, or private key is not set")
	ErrSSHClientNotInit    = errors.New("SSH client not initialized")
	ErrMissingBackupConfig = errors.New("missing backup configuration: token or target")
)

const (
	defaultSSHPort               = ":22"
	pbsConnectivityCheckInterval = 5 * time.Second
	pbsConnectivityMaxRetries    = 6 // 30 seconds total
	metricDirectoryLastBackup    = "directory_last_backup"
	metricDirectoryBackupFailure = "directory_backup_failure"
)

// lineLogger is an io.Writer that logs each complete line to a logger.
type lineLogger struct {
	logger     *slog.Logger
	level      slog.Level
	scanner    *bufio.Scanner
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

// newLineLogger creates a new lineLogger that logs complete lines at the specified level.
func newLineLogger(logger *slog.Logger, level slog.Level) *lineLogger {
	pr, pw := io.Pipe()
	ll := &lineLogger{
		logger:     logger,
		level:      level,
		scanner:    bufio.NewScanner(pr),
		pipeReader: pr,
		pipeWriter: pw,
	}

	// Start goroutine to process lines
	go ll.processLines()

	return ll
}

// Write implements io.Writer by writing to the pipe.
func (ll *lineLogger) Write(p []byte) (n int, err error) {
	return ll.pipeWriter.Write(p)
}

// processLines reads lines from the scanner and logs them.
func (ll *lineLogger) processLines() {
	for ll.scanner.Scan() {
		line := ll.scanner.Text()
		ll.logger.Log(context.Background(), ll.level, line)
	}
}

// Close closes the pipe writer, signaling completion.
func (ll *lineLogger) Close() error {
	return ll.pipeWriter.Close()
}

// BackupDirs manages the execution of directory backups on proxmox servers.
// Runs after the PBS server is powered on
type BackupDirs struct {
	// Dependencies
	Logger     *slog.Logger
	PowerOnPBS *PowerOnPBS
	StatusLine *activity.StatusLine
	Registry   metrics.Registry

	// Configuration
	Files          config.FilesConfig `config:"files"`
	PrivateKeyPath string             `config:"files.private_key_path"`

	// SSH client for remote operations
	sshClient *sshclient.SSHClient

	// Metrics (initialized in Init)
	lastBackupGauge metrics.GaugeVec
	failureCounter  metrics.CounterVec
}

func (a *BackupDirs) Init() error {
	var err error
	a.lastBackupGauge, err = a.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricDirectoryLastBackup,
		Help: "Unix timestamp of last successful directory backup",
	}, []string{"target"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricDirectoryLastBackup, err)
	}

	a.failureCounter, err = a.Registry.NewCounterVec(prometheus.CounterOpts{
		Name: metricDirectoryBackupFailure,
		Help: "Count of directory backup failures",
	}, []string{"target"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricDirectoryBackupFailure, err)
	}

	if a.Files.Target == "" {
		return nil // nothing configured
	}

	if a.Files.Token == "" || a.Files.Target == "" {
		return ErrMissingBackupConfig
	}

	host := a.Files.Host

	if host == "" || a.Files.User == "" || a.PrivateKeyPath == "" {
		return ErrMissingSSHConfig
	}

	// Default to port 22 if not specified
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + defaultSSHPort
	}

	privateKeyPEM, err := os.ReadFile(a.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key file %s: %w", a.PrivateKeyPath, err)
	}

	client, err := sshclient.New(host, a.Files.User, string(privateKeyPEM))
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	a.sshClient = client
	return nil
}

func (a *BackupDirs) Execute(ctx context.Context) error {
	if a.Files.Target == "" {
		return nil // nothing configured
	}

	return activity.CaptureError(a.StatusLine, func() error {
		if a.sshClient == nil {
			return ErrSSHClientNotInit
		}

		if len(a.Files.Sources) == 0 {
			a.StatusLine.Set("no directories to backup")
			return nil
		}

		a.StatusLine.Set(fmt.Sprintf("waiting for the PBS host to become available from %s", a.Files.Host))

		// Test SSH connectivity before attempting backup
		if err := a.waitForPBSHost(ctx); err != nil {
			a.Logger.Error("host not accessible via SSH", "error", err)
			return err
		}

		a.StatusLine.Set(fmt.Sprintf("backing up %d directories", len(a.Files.Sources)))

		err := a.backupAllDirs(a.Files.Sources)
		if err != nil {
			a.Logger.Error("Backup failed", "sources", a.Files.Sources, "error", err)
			return err
		}
		a.Logger.Debug("Backup succeeded", "sources", a.Files.Sources)

		a.StatusLine.Set("directory backup complete")
		return nil
	})
}

// backupAllDirs executes a single backup command with all sources combined
// This enables PBS deduplication across all directories
func (a *BackupDirs) backupAllDirs(sources []string) error {
	token := a.Files.Token
	target := a.Files.Target

	// Build the command with all sources in a single backup command
	cmd := buildBackupCommand(token, target, sources)

	a.Logger.Debug("Running consolidated backup command", "command", cmd, "source_count", len(sources))

	// Create line loggers for stdout and stderr
	stdoutLogger := newLineLogger(a.Logger, slog.LevelDebug)
	stderrLogger := newLineLogger(a.Logger, slog.LevelInfo)
	defer stdoutLogger.Close()
	defer stderrLogger.Close()

	err := a.sshClient.RunWithWriter(cmd, stdoutLogger, stderrLogger)

	labels := prometheus.Labels{"target": target}
	if err != nil {
		a.failureCounter.With(labels).Inc()
	} else {
		a.lastBackupGauge.With(labels).Set(float64(time.Now().Unix()))
	}

	return err
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

// waitForPBSHost tests if PBS is reachable from the remote host before starting backup
func (a *BackupDirs) waitForPBSHost(ctx context.Context) error {
	// Extract hostname from target (format: user@host!datastore@hostname:port)
	target := a.Files.Target
	pbsHost := extractPBSHostFromTarget(target)
	if pbsHost == "" {
		return fmt.Errorf("could not extract PBS host from target: %s", target)
	}

	a.Logger.Debug("Testing PBS connectivity", "pbs_host", pbsHost, "max_retries", pbsConnectivityMaxRetries)

	for attempt := 1; attempt <= pbsConnectivityMaxRetries; attempt++ {
		// Test connectivity using a simple nc (netcat) command
		cmd := fmt.Sprintf("nc -z -w5 %s 8007 2>/dev/null", pbsHost)
		_, _, err := a.sshClient.Run(cmd)
		if err == nil {
			a.Logger.Debug("PBS connectivity test successful", "pbs_host", pbsHost, "attempts", attempt)
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
