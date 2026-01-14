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
//	// Check status with live activity executions and logs
//	status := r.Status()
//	if status.State == runner.RunStateRunning {
//	    // Run in progress - status includes real-time activity executions with logs
//	    for _, exec := range status.ActivityExecutions {
//	        fmt.Printf("%s [%s]: %s\n", exec.Type, exec.State, exec.Status)
//	        for _, log := range exec.Logs {
//	            fmt.Printf("  [%s] %s\n", log.Level, log.Message)
//	        }
//	    }
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
	"sort"
	"sync"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows/backup"
	"github.com/nomis52/goback/workflows/poweroff"
)

const defaultMaxHistorySize = 100

// ErrRunInProgress is returned when attempting to start a run while one is already running.
var ErrRunInProgress = errors.New("backup run already in progress")

// Runner manages backup run execution.
type Runner struct {
	logger         *slog.Logger
	configProvider ConfigProvider
	store          StateStore

	mu               sync.Mutex
	runStatus        RunStatus
	workflow         workflow.Workflow                 // Current or last run's workflow
	statusCollection *activity.StatusHandler // Current run's status collection
	logCollector     *logging.LogCollector            // Captures logs during workflow execution
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

// Status returns the current run status with live activity executions and logs.
// If a run is in progress, includes real-time activity executions with captured logs and status messages.
// If idle, returns the last completed run status (already includes activity executions).
func (r *Runner) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Make a copy of the base status
	status := r.runStatus

	// If running, build live activity executions with current logs and status messages
	if r.runStatus.State == RunStateRunning && r.workflow != nil && r.logCollector != nil {
		status.ActivityExecutions = r.buildActivityExecutions()
	}

	return status
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
func (r *Runner) CurrentStatuses() map[workflow.ActivityID]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.statusCollection == nil {
		return nil
	}
	return r.statusCollection.All()
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

	// Build activity executions with logs and status messages
	if r.workflow != nil && r.logCollector != nil {
		r.runStatus.ActivityExecutions = r.buildActivityExecutions()
	}

	// Save to store
	if err := r.store.Save(r.runStatus); err != nil {
		r.logger.Error("failed to save run to store", "error", err)
	}
}

// buildActivityExecutions combines workflow results, logs, and status messages into ActivityExecution structs.
func (r *Runner) buildActivityExecutions() []ActivityExecution {
	results := r.workflow.GetAllResults()
	logs := r.logCollector.GetAllLogs()

	// Get current status messages if collection is available
	var statuses map[workflow.ActivityID]string
	if r.statusCollection != nil {
		statuses = r.statusCollection.All()
	}

	executions := make([]ActivityExecution, 0, len(results))

	for id, result := range results {
		exec := ActivityExecution{
			Module:    id.Module,
			Type:      id.Type,
			State:     result.State.String(),
			StartTime: &result.StartTime,
			EndTime:   &result.EndTime,
		}

		if result.Error != nil {
			exec.Error = result.Error.Error()
		}

		// Add status message for this activity
		if statuses != nil {
			if statusMsg, exists := statuses[id]; exists {
				exec.Status = statusMsg
			}
		}

		// Add logs for this activity
		if activityLogs, exists := logs[id.String()]; exists {
			exec.Logs = activityLogs
		}

		executions = append(executions, exec)
	}

	// Sort by type for stable ordering
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].Type < executions[j].Type
	})

	return executions
}

func (r *Runner) executeRun(ctx context.Context) error {
	cfg := r.configProvider.Config()
	if cfg == nil {
		return errors.New("no configuration available")
	}

	// Create status collection for this run
	statusCollection := activity.NewStatusHandler()

	// Create log collector for this run
	logCollector := logging.NewLogCollector()

	// Create logger factory that captures logs per activity
	loggerFactory := func(id workflow.ActivityID) *slog.Logger {
		handler := logging.NewCapturingHandler(r.logger.Handler(), logCollector, id.String())
		return slog.New(handler)
	}

	// Create backup workflow (PowerOnPBS → BackupDirs → BackupVMs) with log capturing
	backupWorkflow, err := backup.NewWorkflow(cfg, r.logger,
		backup.WithStatusCollection(statusCollection),
		backup.WithLoggerFactory(loggerFactory))
	if err != nil {
		return fmt.Errorf("failed to create backup workflow: %w", err)
	}

	// Create power off workflow (PowerOffPBS) with log capturing
	powerOffWorkflow, err := poweroff.NewWorkflow(cfg, r.logger,
		poweroff.WithStatusCollection(statusCollection),
		poweroff.WithLoggerFactory(loggerFactory))
	if err != nil {
		return fmt.Errorf("failed to create power off workflow: %w", err)
	}

	// Compose workflows to run backup then power off
	composedWorkflow := workflow.Compose(backupWorkflow, powerOffWorkflow)

	// Store workflow, status collection, and log collector references for result/status/log access
	r.mu.Lock()
	r.workflow = composedWorkflow
	r.statusCollection = statusCollection
	r.logCollector = logCollector
	r.mu.Unlock()

	// Execute composed workflow
	if err := composedWorkflow.Execute(ctx); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}
