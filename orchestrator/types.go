package orchestrator

import (
	"context"
)

// Activity represents a single step in the orchestration process
type Activity interface {
	Init() error
	// Execute runs the activity and returns an error if it fails
	// Success is indicated by returning nil
	Execute(ctx context.Context) error
}

// Result contains the outcome of an activity execution
type Result struct {
	// State indicates the current execution state
	State ActivityState
	
	// Error contains any error that occurred during execution
	// nil indicates successful execution
	Error error
}

// IsSuccess returns true if the activity completed successfully
func (r *Result) IsSuccess() bool {
	return r.State == Completed && r.Error == nil
}
