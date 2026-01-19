package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomis52/goback/server/config"
)

// Runnable is implemented by anything that can be triggered by the cron scheduler.
type Runnable interface {
	Run(workflows []string) error
}

// CronTriggerManager manages multiple CronTrigger instances with different workflows and schedules.
type CronTriggerManager struct {
	triggers  []*CronTrigger
	workflows [][]string // workflows[i] corresponds to triggers[i]
	logger    *slog.Logger
}

// NewCronTriggerManager creates a new CronTriggerManager from a list of trigger configurations.
func NewCronTriggerManager(triggers []config.CronTrigger, runnable Runnable, logger *slog.Logger) (*CronTriggerManager, error) {
	// Create a CronTrigger for each config
	managedTriggers := make([]*CronTrigger, 0, len(triggers))
	workflows := make([][]string, 0, len(triggers))

	for i, cfg := range triggers {
		// Validate workflows
		if len(cfg.Workflows) == 0 {
			return nil, fmt.Errorf("trigger %d: no workflows specified", i)
		}

		// Create a closure that captures the workflows and runnable
		workflowsCopy := make([]string, len(cfg.Workflows))
		copy(workflowsCopy, cfg.Workflows)

		callback := func() error {
			return runnable.Run(workflowsCopy)
		}

		trigger, err := NewCronTrigger(cfg.Schedule, callback, logger)
		if err != nil {
			return nil, fmt.Errorf("creating trigger %d for '%s': %w",
				i, cfg.Schedule, err)
		}
		managedTriggers = append(managedTriggers, trigger)
		workflows = append(workflows, workflowsCopy)
	}

	// Log details for each trigger
	for i, trigger := range managedTriggers {
		logger.Info("trigger registered",
			"index", i,
			"workflows", workflows[i],
			"schedule", triggers[i].Schedule,
			"next_run", trigger.NextRun(),
		)
	}

	return &CronTriggerManager{
		triggers:  managedTriggers,
		workflows: workflows,
		logger:    logger,
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

// NextTriggerInfo contains information about the next scheduled trigger.
type NextTriggerInfo struct {
	Time      time.Time
	Workflows []string
}

// NextTrigger returns information about the next scheduled trigger.
// Returns the earliest scheduled run time and its associated workflows.
// Returns zero time and nil workflows if there are no triggers.
func (m *CronTriggerManager) NextTrigger() NextTriggerInfo {
	if len(m.triggers) == 0 {
		return NextTriggerInfo{}
	}

	earliest := m.triggers[0].NextRun()
	earliestIdx := 0

	for i := 1; i < len(m.triggers); i++ {
		next := m.triggers[i].NextRun()
		if next.Before(earliest) {
			earliest = next
			earliestIdx = i
		}
	}

	return NextTriggerInfo{
		Time:      earliest,
		Workflows: m.workflows[earliestIdx],
	}
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
