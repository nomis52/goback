// Package maintenance provides workflow factories for PBS datastore maintenance:
// pruning old snapshots and garbage-collecting the chunks they referenced.
//
// PBS can schedule both itself, but on a server that is only powered on for the
// daily backup run every schedule fires as a missed-job catch-up right after boot,
// on top of the backup. These workflows run the same PBS jobs through the PBS API
// at a point the caller chooses instead: stack them between the backup and
// power-off in the cron config (e.g. combined-backup → pbs-maintenance → power-off).
//
// The prune side runs a prune job that is configured in PBS, so the retention
// policy stays visible and editable in the PBS UI. Keep that job disabled in PBS:
// the disable flag only stops the PBS scheduler (and the boot-time catch-up); a
// manual run through the API still works.
//
// Like the backup workflows, each maintenance workflow powers PBS on first (a
// no-op when it is already on) and none of them power it off.
package maintenance

import (
	"fmt"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
	"github.com/nomis52/goback/workflows/poweron"
)

// NewWorkflow creates the "pbs-maintenance" workflow: prune, then garbage-collect.
// It composes the prune and GC workflows, so GC still runs when the prune fails
// (per Compose's continue-on-failure semantics): GC only frees chunks that are
// already unreferenced, and a prune failure is usually a lock on one group rather
// than a datastore problem.
// The workflow executes: (PowerOnPBS → PrunePBS) then (PowerOnPBS → GarbageCollectPBS).
func NewWorkflow(params workflows.Params) (workflow.Workflow, error) {
	prune, err := NewPruneWorkflow(params)
	if err != nil {
		return nil, err
	}

	gc, err := NewGCWorkflow(params)
	if err != nil {
		return nil, err
	}

	return workflow.Compose(prune, gc), nil
}

// NewPruneWorkflow creates the "pbs-prune" workflow: power on PBS, then run the
// configured prune job and wait for it to finish.
// The workflow executes: PowerOnPBS → PrunePBS.
func NewPruneWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newMaintenanceOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&poweron.PowerOnPBS{}, &PrunePBS{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// NewGCWorkflow creates the "pbs-gc" workflow: power on PBS, then run garbage
// collection on the configured datastore and wait for it to finish.
// The workflow executes: PowerOnPBS → GarbageCollectPBS.
func NewGCWorkflow(params workflows.Params) (workflow.Workflow, error) {
	o, err := newMaintenanceOrchestrator(params)
	if err != nil {
		return nil, err
	}

	if err := o.AddActivity(&poweron.PowerOnPBS{}, &GarbageCollectPBS{}); err != nil {
		return nil, fmt.Errorf("failed to add activities: %w", err)
	}

	return o, nil
}

// newMaintenanceOrchestrator builds an orchestrator with the IPMI controller (for
// PowerOnPBS) and a token-authenticated PBS client (shared by PowerOnPBS, which
// only pings, and the maintenance activities) registered, but no activities added.
func newMaintenanceOrchestrator(params workflows.Params) (*workflow.Orchestrator, error) {
	cfg := params.Config
	logger := params.Logger

	o := workflow.NewOrchestrator(
		workflow.WithConfig(cfg),
		workflow.WithLogger(logger),
	)

	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(logger),
	)

	pbsClient, err := pbsclient.New(cfg.PBS.Host,
		pbsclient.WithToken(cfg.Maintenance.Token),
		pbsclient.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create PBS client: %w", err)
	}

	workflow.Provide(o, workflow.Shared(ctrl))
	workflow.Provide(o, workflow.Shared(pbsClient))

	// Inject common factories (logger, metrics registry, status line)
	params.InjectInto(o)

	return o, nil
}
