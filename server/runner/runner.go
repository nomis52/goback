// Package runner manages backup run execution for the goback server.
//
// The runner handles:
//   - Starting backup runs in the background
//   - Preventing concurrent runs
//   - Tracking current run status
//   - Maintaining history of completed runs
//
// Each run creates fresh dependencies from the current configuration,
// ensuring config changes take effect on the next run.
//
// # Example
//
//	r := runner.New(logger, configProvider)
//
//	// Start a run
//	if err := r.Run(); err != nil {
//	    if errors.Is(err, runner.ErrRunInProgress) {
//	        // Handle concurrent run attempt
//	    }
//	}
//
//	// Check status
//	status := r.Status()
//	if status.State == runner.RunStateRunning {
//	    // Run in progress
//	}
//
//	// Get history
//	history := r.History() // Most recent first
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/nomis52/goback/activities"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/pbsclient"
	"github.com/nomis52/goback/proxmoxclient"
)

const defaultMaxHistorySize = 100

// ErrRunInProgress is returned when attempting to start a run while one is already running.
var ErrRunInProgress = errors.New("backup run already in progress")

// Runner manages backup run execution.
type Runner struct {
	logger         *slog.Logger
	configProvider ConfigProvider

	mu             sync.Mutex
	maxHistorySize int
	runStatus      RunStatus
	history        []RunStatus
	orchestrator   *orchestrator.Orchestrator // Current or last run's orchestrator
}

// ConfigProvider provides access to the current configuration.
type ConfigProvider interface {
	Config() *config.Config
}

// New creates a new Runner.
func New(logger *slog.Logger, provider ConfigProvider) *Runner {
	return &Runner{
		logger:         logger,
		configProvider: provider,
		maxHistorySize: defaultMaxHistorySize,
		runStatus:      RunStatus{State: RunStateIdle},
		history:        make([]RunStatus, 0),
	}
}

// Run starts a backup run in the background.
// Returns ErrRunInProgress if a run is already in progress.
func (r *Runner) Run() error {
	if !r.tryStart() {
		return ErrRunInProgress
	}

	r.logger.Info("starting backup run")

	go func() {
		err := r.executeRun(context.Background())
		r.finish(err)
	}()

	return nil
}

// Status returns the current run status.
func (r *Runner) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runStatus
}

// IsRunning returns true if a backup run is in progress.
func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runStatus.State == RunStateRunning
}

// History returns the history of completed runs, most recent first.
func (r *Runner) History() []RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]RunStatus, len(r.history))
	copy(result, r.history)
	return result
}

// GetResults returns the activity results from the current or last run.
// Returns nil if no run has been executed yet.
func (r *Runner) GetResults() map[orchestrator.ActivityID]*orchestrator.Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.orchestrator == nil {
		return nil
	}
	return r.orchestrator.GetAllResults()
}

// tryStart attempts to transition from idle to running.
// Returns true if successful, false if already running.
func (r *Runner) tryStart() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.runStatus.State == RunStateRunning {
		return false
	}

	now := time.Now()
	r.runStatus = RunStatus{
		State:     RunStateRunning,
		StartedAt: &now,
	}
	return true
}

// finish transitions from running to idle and records the result.
func (r *Runner) finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	endTime := time.Now()
	duration := endTime.Sub(*r.runStatus.StartedAt)

	r.runStatus.State = RunStateIdle
	r.runStatus.EndedAt = &endTime

	if err != nil {
		r.runStatus.Error = err.Error()
		r.logger.Error("backup run failed", "error", err, "duration", duration)
	} else {
		r.runStatus.Error = ""
		r.logger.Info("backup run completed", "duration", duration)
	}

	r.history = append([]RunStatus{r.runStatus}, r.history...)

	if len(r.history) > r.maxHistorySize {
		r.history = r.history[:r.maxHistorySize]
	}
}

func (r *Runner) executeRun(ctx context.Context) error {
	cfg := r.configProvider.Config()
	if cfg == nil {
		return errors.New("no configuration available")
	}

	deps, err := r.buildRunDeps(cfg)
	if err != nil {
		return fmt.Errorf("failed to build run dependencies: %w", err)
	}

	o := orchestrator.NewOrchestrator(
		orchestrator.WithConfig(cfg),
		orchestrator.WithLogger(r.logger),
	)

	// Store orchestrator reference for result access
	r.mu.Lock()
	r.orchestrator = o
	r.mu.Unlock()

	if err := o.Inject(r.logger, deps.metricsClient, deps.ipmiController, deps.pbsClient, deps.proxmoxClient); err != nil {
		return fmt.Errorf("failed to inject dependencies: %w", err)
	}

	powerOnPBS := &activities.PowerOnPBS{}
	backupDirs := &activities.BackupDirs{}
	backupVMs := &activities.BackupVMs{}
	powerOffPBS := &activities.PowerOffPBS{}

	if err := o.AddActivity(powerOnPBS, backupDirs, backupVMs, powerOffPBS); err != nil {
		return fmt.Errorf("failed to add activities: %w", err)
	}

	if err := o.Execute(ctx); err != nil {
		return fmt.Errorf("orchestrator execution failed: %w", err)
	}

	return nil
}

// runDeps holds dependencies created for a single run.
type runDeps struct {
	ipmiController *ipmi.IPMIController
	pbsClient      *pbsclient.Client
	proxmoxClient  *proxmoxclient.Client
	metricsClient  *metrics.Client
}

func (r *Runner) buildRunDeps(cfg *config.Config) (*runDeps, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	ctrl := ipmi.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmi.WithUsername(cfg.PBS.IPMI.Username),
		ipmi.WithPassword(cfg.PBS.IPMI.Password),
		ipmi.WithLogger(r.logger),
	)

	pbsClient, err := pbsclient.New(cfg.PBS.Host, r.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PBS client: %w", err)
	}

	proxmoxClient, err := proxmoxclient.New(cfg.Proxmox.Host, proxmoxclient.WithToken(cfg.Proxmox.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	metricsClient := metrics.NewClient(
		cfg.Monitoring.VictoriaMetricsURL,
		metrics.WithPrefix(cfg.Monitoring.MetricsPrefix),
		metrics.WithJob(cfg.Monitoring.JobName),
		metrics.WithInstance(hostname),
	)

	return &runDeps{
		ipmiController: ctrl,
		pbsClient:      pbsClient,
		proxmoxClient:  proxmoxClient,
		metricsClient:  metricsClient,
	}, nil
}
