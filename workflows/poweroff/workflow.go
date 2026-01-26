// Package poweroff provides workflow factories for power-off operations.
// It orchestrates the graceful shutdown of the PBS server via IPMI.
package poweroff

import (
	"fmt"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// NewWorkflow creates a workflow that gracefully powers off PBS.
// The workflow executes: PowerOffPBS
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	cfg := params.Config
	logger := params.Logger

	// Create orchestrator with config and logger options
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	// Create IPMI controller directly (no buildDeps needed)
	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	// Register factories for dependencies
	workflow.Provide(o, workflow.Shared(ctrl))

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

	// Add power off activity
	powerOffPBS := &PowerOffPBS{}

	if err := o.AddActivity(powerOffPBS); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}
