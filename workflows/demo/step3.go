package demo

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/workflow"
)

// Step3 is the third demo activity that runs after Step2 completes.
type Step3 struct {
	Logger         *slog.Logger
	StatusLine *activity.StatusLine
	_              *Step2 // Unnamed dependency ensures Step2 runs first
}

// Init performs structural validation.
func (a *Step3) Init() error {
	return nil
}

// Execute performs the activity work.
func (a *Step3) Execute(ctx context.Context) error {
	return activity.CaptureError(a.StatusLine, func() error {
		a.Logger.Info("starting demo step 3")

		a.StatusLine.Set("starting demo step 3")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("halfway through demo step 3")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("completed demo step 3")
		return nil
	})
}

var _ workflow.Activity = (*Step3)(nil)
