package statusreporter

import (
	"log/slog"
	"sync"

	"github.com/nomis52/goback/workflow"
)

// StatusReporter allows activities to report their current status during execution.
//
// StatusReporter is created by the runner and injected into activities via orchestrator.Inject().
// Activities call SetStatus() passing themselves and their current status message.
//
// USAGE IN ACTIVITIES:
//
//	type PowerOnPBS struct {
//	    StatusReporter *statusreporter.StatusReporter
//	    // ... other fields
//	}
//
//	func (a *PowerOnPBS) Execute(ctx context.Context) error {
//	    a.StatusReporter.SetStatus(a, "waiting for PBS server to power on")
//	    // ... do work
//	    a.StatusReporter.SetStatus(a, "PBS server is online")
//	    return nil
//	}
//
// THREAD SAFETY:
// All methods are thread-safe and can be called from concurrent goroutines.
type StatusReporter struct {
	statuses map[string]string
	logger   *slog.Logger
	mu       sync.RWMutex
}

// New creates a new StatusReporter.
// Each status change is automatically logged at Info level.
func New(logger *slog.Logger) *StatusReporter {
	return &StatusReporter{
		statuses: make(map[string]string),
		logger:   logger,
	}
}

// SetStatus updates the current status for a specific activity.
//
// The activity parameter is the activity reporting its status.
// The status string should describe what the activity is currently doing,
// for example: "waiting for PBS server to power on" or "backing up VM 3/10".
//
// Each status change is automatically logged at Info level with the activity name and status.
func (r *StatusReporter) SetStatus(activity workflow.Activity, status string) {
	activityID := workflow.GetActivityID(activity)

	// Log status change (outside the lock to avoid holding it during I/O)
	r.logger.Info(status, "activity", activityID.ShortString())

	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses[activityID.String()] = status
}

// CurrentStatuses returns a copy of all current activity statuses.
// The returned map uses activity ID strings as keys.
func (r *StatusReporter) CurrentStatuses() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string, len(r.statuses))
	for name, status := range r.statuses {
		result[name] = status
	}
	return result
}
