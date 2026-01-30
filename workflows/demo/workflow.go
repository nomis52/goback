// Package demo provides a simple demo workflow with 3 sequential activities
// for testing the orchestrator's status reporting and execution flow.
package demo

import (
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// NewWorkflow creates a demo workflow with 3 sequential activities.
// Each activity sets status messages and sleeps to simulate work.
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	cfg := params.Config
	logger := params.Logger

	// Create orchestrator with config and logger options
	var opts []workflow.OrchestratorOption
	opts = append(opts, workflow.WithLogger(logger))
	if cfg != nil {
		opts = append(opts, workflow.WithConfig(cfg))
	}
	o := workflow.NewOrchestrator(opts...)

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

	step1 := &Step1{}
	step2 := &Step2{}
	step3 := &Step3{}

	if err := o.AddActivity(step1, step2, step3); err != nil {
		return nil, err
	}

	return o, nil
}
