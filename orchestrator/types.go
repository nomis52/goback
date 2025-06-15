package orchestrator

import (
	"context"
)

// Activity represents a single step in the orchestration process.
//
// IMPLEMENTATION CONTRACT:
// - Init() validates structure and configuration (called after injection, before execution)
// - Execute() performs the actual work (called after dependencies complete successfully)
// - Activities should handle context cancellation gracefully
// - Dependencies are automatically injected into struct pointer fields
//
// INIT() PURPOSE - STRUCTURAL VALIDATION ONLY:
// Init() is for "fail-fast" validation of static structure and configuration.
// At Init() time, all dependencies are injected but NO activities have executed yet.
// 
// Init() SHOULD validate:
// - Required configuration fields are set and valid
// - Required dependencies are injected (not nil)
// - Static relationships between configuration values
// - Configuration value ranges and formats
//
// Init() SHOULD NOT validate:
// - Whether dependencies have executed (they haven't yet)
// - Runtime state or dynamic conditions
// - Results from other activities
type Activity interface {
	// Init validates structure and configuration after injection is complete.
	// Called on ALL activities before ANY activity executes.
	// Use for "fail-fast" validation of configuration and dependency injection.
	// DO NOT check execution state of dependencies - they haven't run yet.
	// Return an error if the activity is misconfigured or dependencies are missing.
	Init() error
	
	// Execute performs the activity's actual work.
	// Called after all dependencies have completed successfully.
	// This is where runtime validation and dependency execution checks belong.
	// Return nil for success, or an error describing the failure.
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
