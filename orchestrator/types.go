package orchestrator

import (
	"context"
)

// Activity represents a single step in the orchestration process
type Activity interface {
	Init() error
	Run(ctx context.Context) (Result, error)
}

// Result contains the outcome of an activity execution
type Result interface {
	IsSuccess() bool
}

// ActivityResult is the standard implementation of Result
type ActivityResult struct {
	Success bool
}

func (r *ActivityResult) IsSuccess() bool {
	return r.Success
}

// NewSuccessResult creates a successful result
func NewSuccessResult() Result {
	return &ActivityResult{
		Success: true,
	}
}

// NewFailureResult creates a failed result
func NewFailureResult() Result {
	return &ActivityResult{
		Success: false,
	}
}
