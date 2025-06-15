package orchestrator

import (
	"context"
)

// Activity represents a single step in the orchestration process.
//
// IMPLEMENTATION CONTRACT:
// - Init() is called after all dependency/config injection but before Execute()
// - Use Init() to validate configuration and dependencies are properly set
// - Execute() performs the actual work - return nil for success, error for failure
// - Activities should handle context cancellation gracefully
// - Dependencies are automatically injected into struct pointer fields
type Activity interface {
	// Init can be used to validate dependencies.
	// Init() is called after dependency injection but before Execute().
	// Return an error if configuration or dependencies are invalid.
	Init() error

	// Execute performs the activity's work.
	// Return nil for success, or an error describing the failure.
	// Should handle context cancellation appropriately.
	Execute(ctx context.Context) error
}

// Result contains the outcome of an activity execution.
//
// LIFECYCLE:
// - Created in NotStarted state when activity is added via AddActivity()
// - Progresses through states during Execute(): NotStarted -> Pending -> Running -> (Completed|Skipped)
// - Final state persists after Execute() completes
// - Thread-safe access via orchestrator result methods
type Result struct {
	// State indicates the current execution state
	// See ActivityState constants and package documentation for state progression details
	State ActivityState

	// Error contains only errors returned by the activity's Execute() method
	// nil indicates Execute() returned nil (success) or Execute() was never called
	// Validation errors, dependency failures, and cancellations are reflected in State only
	Error error
}

// IsSuccess returns true if the activity completed successfully.
//
// SUCCESS CRITERIA:
// - State must be Completed (activity actually ran)
// - Error must be nil (no execution errors occurred)
//
// NOTE: Skipped activities are not considered successful even if Error is nil.
func (r *Result) IsSuccess() bool {
	return r.State == Completed && r.Error == nil
}
