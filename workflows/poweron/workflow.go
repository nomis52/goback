package poweron

import (
	"fmt"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// NewWorkflow creates the "power-on" workflow: power on PBS via IPMI and wait for it
// to become reachable.
// The workflow executes: PowerOnPBS.
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	cfg := params.Config
	logger := params.Logger

	// Create orchestrator with config and logger options
	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	// PowerOnPBS needs the IPMI controller (to power on) and the PBS client (to
	// wait for PBS to become reachable).
	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	pbsClient, err := pbsclient.New(cfg.PBS.Host, pbsclient.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("failed to create PBS client: %w", err)
	}

	// Register factories for dependencies
	workflow.Provide(o, workflow.Shared(ctrl))
	workflow.Provide(o, workflow.Shared(pbsClient))

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

	// Add power on activity
	if err := o.AddActivity(&PowerOnPBS{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}
