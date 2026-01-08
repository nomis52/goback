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
	"sync"
	"time"

	"github.com/nomis52/goback/backup"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

const defaultMaxHistorySize = 100

// ErrRunInProgress is returned when attempting to start a run while one is already running.
var ErrRunInProgress = errors.New("backup run already in progress")

// Runner manages backup run execution.
type Runner struct {
	logger         *slog.Logger
	configProvider ConfigProvider
	store          StateStore

	mu             sync.Mutex
	runStatus      RunStatus
	workflow       workflow.Workflow // Current or last run's workflow
	statusReporter *statusreporter.StatusReporter // Current run's status reporter
}

// ConfigProvider provides access to the current configuration.
type ConfigProvider interface {
	Config() *config.Config
}

// Option configures a Runner.
type Option func(*Runner)

// WithStateStore configures the runner to use the provided store for persistence.
func WithStateStore(store StateStore) Option {
	return func(r *Runner) {
		r.store = store
	}
}

// New creates a new Runner.
func New(logger *slog.Logger, provider ConfigProvider, opts ...Option) *Runner {
	r := &Runner{
		logger:         logger,
		configProvider: provider,
		store:          NewMemoryStore(),
		runStatus:      RunStatus{State: RunStateIdle},
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	return r
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
	return r.store.Runs()
}

// GetResults returns the activity results from the current or last run.
// Returns nil if no run has been executed yet.
func (r *Runner) GetResults() map[workflow.ActivityID]*workflow.Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.workflow == nil {
		return nil
	}
	return r.workflow.GetAllResults()
}

// CurrentStatuses returns the current activity statuses during a run.
// Returns nil if no run is currently in progress.
func (r *Runner) CurrentStatuses() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.statusReporter == nil {
		return nil
	}
	return r.statusReporter.CurrentStatuses()
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

	// Save to store
	if err := r.store.Save(r.runStatus); err != nil {
		r.logger.Error("failed to save run to store", "error", err)
	}
}

func (r *Runner) executeRun(ctx context.Context) error {
	cfg := r.configProvider.Config()
	if cfg == nil {
		return errors.New("no configuration available")
	}

	// Create status reporter
	sr, err := r.createStatusReporter()
	if err != nil {
		return fmt.Errorf("failed to create status reporter: %w", err)
	}

	// Create backup workflow (PowerOnPBS → BackupDirs → BackupVMs)
	backupWorkflow, err := backup.NewBackupWorkflow(cfg, r.logger, sr)
	if err != nil {
		return fmt.Errorf("failed to create backup workflow: %w", err)
	}

	// Create power off workflow (PowerOffPBS)
	powerOffWorkflow, err := backup.NewPowerOffWorkflow(cfg, r.logger, sr)
	if err != nil {
		return fmt.Errorf("failed to create power off workflow: %w", err)
	}

	// Compose workflows to run backup then power off
	composedWorkflow := workflow.Compose(backupWorkflow, powerOffWorkflow)

	// Store workflow and status reporter references for result/status access
	r.mu.Lock()
	r.workflow = composedWorkflow
	r.statusReporter = sr
	r.mu.Unlock()

	// Execute composed workflow
	if err := composedWorkflow.Execute(ctx); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}

func (r *Runner) createStatusReporter() (*statusreporter.StatusReporter, error) {
	return statusreporter.New(r.logger), nil
}
