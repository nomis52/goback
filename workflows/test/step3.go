package test

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

// Step3 is the third test activity that runs after Step2 completes.
type Step3 struct {
	Logger         *slog.Logger
	StatusReporter *statusreporter.StatusReporter
	_              *Step2 // Unnamed dependency ensures Step2 runs first
}

// Init performs structural validation.
func (a *Step3) Init() error {
	return nil
}

// Execute performs the activity work.
func (a *Step3) Execute(ctx context.Context) error {
	return statusreporter.RecordError(a, a.StatusReporter, func() error {
		a.Logger.Info("starting step 3")

		a.StatusReporter.SetStatus(a, "starting step 3")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusReporter.SetStatus(a, "halfway through step 3")
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		a.StatusReporter.SetStatus(a, "completed step 3")
		return nil
	})
}

var _ workflow.Activity = (*Step3)(nil)
