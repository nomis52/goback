// Package test provides a simple test workflow with 3 sequential activities
// for testing the orchestrator's status reporting and execution flow.
package test

import (
	"log/slog"

	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
)

// NewWorkflow creates a test workflow with 3 sequential activities.
// Each activity sets status messages and sleeps to simulate work.
func NewWorkflow(logger *slog.Logger, statusReporter *statusreporter.StatusReporter) (workflow.Workflow, error) {
	o := workflow.NewOrchestrator(workflow.WithLogger(logger))

	if err := o.Inject(logger, statusReporter); err != nil {
		return nil, err
	}

	step1 := &Step1{}
	step2 := &Step2{}
	step3 := &Step3{}

	if err := o.AddActivity(step1, step2, step3); err != nil {
		return nil, err
	}

	return o, nil
}
