package activity

import (
	"sync"

	"github.com/nomis52/goback/workflow"
)

// StatusHandler stores activity status messages by activity ID.
// This is the shared storage that all status lines write to.
// Similar to slog.Handler, it receives and stores status updates.
type StatusHandler struct {
	statuses map[workflow.ActivityID]string
	mu       sync.RWMutex
}

// NewStatusHandler creates a new status handler.
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{
		statuses: make(map[workflow.ActivityID]string),
	}
}

// Set updates the status for a specific activity ID.
// This is called by StatusLine instances.
func (sh *StatusHandler) Set(activityID workflow.ActivityID, status string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.statuses[activityID] = status
}

// Get returns the status for a specific activity ID.
func (sh *StatusHandler) Get(activityID workflow.ActivityID) string {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.statuses[activityID]
}

// All returns a copy of all activity statuses.
// Used by the server to display current status in the web UI.
func (sh *StatusHandler) All() map[workflow.ActivityID]string {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	// Return a copy to avoid concurrent map access
	copy := make(map[workflow.ActivityID]string, len(sh.statuses))
	for k, v := range sh.statuses {
		copy[k] = v
	}
	return copy
}
