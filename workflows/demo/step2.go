package demo

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/workflow"
)

// Step2 is the second demo activity that runs after Step1 completes.
type Step2 struct {
	Logger         *slog.Logger
	StatusLine *activity.StatusLine
	_              *Step1 // Unnamed dependency ensures Step1 runs first
}

// Init performs structural validation.
func (a *Step2) Init() error {
	return nil
}

// Execute performs the activity work.
func (a *Step2) Execute(ctx context.Context) error {
	return activity.CaptureError(a.StatusLine, func() error {
		a.Logger.Info("starting demo step 2")

		a.StatusLine.Set("starting demo step 2")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("halfway through demo step 2")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("completed demo step 2")
		return nil
	})
}

var _ workflow.Activity = (*Step2)(nil)
