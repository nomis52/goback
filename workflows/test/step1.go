package test

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/workflow"
)

// Step1 is the first test activity that simulates work with status updates and sleeps.
type Step1 struct {
	Logger         *slog.Logger
	StatusLine *activity.StatusLine
}

// Init performs structural validation.
func (a *Step1) Init() error {
	return nil
}

// Execute performs the activity work.
func (a *Step1) Execute(ctx context.Context) error {
	return activity.CaptureError(a.StatusLine, func() error {
		a.Logger.Info("starting step 1")

		a.StatusLine.Set("starting step 1")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("halfway through step 1")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusLine.Set("completed step 1")
		return nil
	})
}

var _ workflow.Activity = (*Step1)(nil)
