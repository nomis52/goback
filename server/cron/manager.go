package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Runnable is implemented by anything that can be triggered by the cron scheduler.
type Runnable interface {
	Run(workflows []string) error
}

// CronTriggerManager manages multiple CronTrigger instances with different workflows and schedules.
type CronTriggerManager struct {
	triggers []*CronTrigger
	logger   *slog.Logger
}

// NewCronTriggerManager creates a new CronTriggerManager from a multi-trigger specification.
// The spec format is: workflow1,workflow2:cron_expression;workflow3:cron_expression2
//
// Example:
//
//	"backup,poweroff:0 2 * * *;test:0 3 * * *"
//
// Returns an error if:
//   - The spec is invalid or cannot be parsed
//   - Any workflow name is not in availableWorkflows
//   - Any cron expression is invalid
func NewCronTriggerManager(spec string, runnable Runnable, logger *slog.Logger, availableWorkflows map[string]bool) (*CronTriggerManager, error) {
	// Parse the trigger specifications
	triggerSpecs, err := ParseTriggerSpecs(spec, availableWorkflows)
	if err != nil {
		return nil, err
	}

	// Create a CronTrigger for each spec
	triggers := make([]*CronTrigger, 0, len(triggerSpecs))
	for _, spec := range triggerSpecs {
		// Create a closure that captures the workflows and runnable
		workflows := spec.Workflows // Capture for closure
		callback := func() error {
			return runnable.Run(workflows)
		}

		trigger, err := NewCronTrigger(spec.CronSpec, callback, logger)
		if err != nil {
			return nil, fmt.Errorf("creating trigger for '%s:%s': %w",
				formatWorkflowList(spec.Workflows), spec.CronSpec, err)
		}
		triggers = append(triggers, trigger)
	}

	logger.Info("cron trigger manager created", "trigger_count", len(triggers))

	// Log details for each trigger
	for i, trigger := range triggers {
		logger.Info("trigger registered",
			"index", i,
			"workflows", triggerSpecs[i].Workflows,
			"schedule", triggerSpecs[i].CronSpec,
			"next_run", trigger.NextRun(),
		)
	}

	return &CronTriggerManager{
		triggers: triggers,
		logger:   logger,
	}, nil
}

// Start launches all triggers. Each trigger runs in its own goroutine.
// Returns immediately. All goroutines exit when ctx is cancelled.
func (m *CronTriggerManager) Start(ctx context.Context) {
	for _, trigger := range m.triggers {
		trigger.Start(ctx)
	}
}

// NextRun returns the earliest scheduled run time across all triggers.
// Returns zero time if there are no triggers.
func (m *CronTriggerManager) NextRun() time.Time {
	if len(m.triggers) == 0 {
		return time.Time{}
	}

	earliest := m.triggers[0].NextRun()
	for i := 1; i < len(m.triggers); i++ {
		next := m.triggers[i].NextRun()
		if next.Before(earliest) {
			earliest = next
		}
	}

	return earliest
}

// formatWorkflowList formats a workflow list for error messages.
func formatWorkflowList(workflows []string) string {
	result := ""
	for i, w := range workflows {
		if i > 0 {
			result += ","
		}
		result += w
	}
	return result
}
