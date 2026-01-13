package statusreporter

import (
	"sync"

	"github.com/nomis52/goback/workflow"
)

// StatusCollection stores activity status messages by activity ID.
// This is the shared storage that all status lines write to.
type StatusCollection struct {
	statuses map[workflow.ActivityID]string
	mu       sync.RWMutex
}

// NewStatusCollection creates a new status collection.
func NewStatusCollection() *StatusCollection {
	return &StatusCollection{
		statuses: make(map[workflow.ActivityID]string),
	}
}

// Set updates the status for a specific activity ID.
// This is called by StatusLine instances.
func (sc *StatusCollection) Set(activityID workflow.ActivityID, status string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.statuses[activityID] = status
}

// Get returns the status for a specific activity ID.
func (sc *StatusCollection) Get(activityID workflow.ActivityID) string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.statuses[activityID]
}

// All returns a copy of all activity statuses.
// Used by the server to display current status in the web UI.
func (sc *StatusCollection) All() map[workflow.ActivityID]string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Return a copy to avoid concurrent map access
	copy := make(map[workflow.ActivityID]string, len(sc.statuses))
	for k, v := range sc.statuses {
		copy[k] = v
	}
	return copy
}
