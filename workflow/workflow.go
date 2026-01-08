package workflow

import (
	"context"
	"fmt"
	"strings"
)

// Workflow represents an executable workflow that manages a set of activities.
// Workflows provide a clean interface for composing and executing activities
// while maintaining access to their results.
type Workflow interface {
	// Execute runs the workflow to completion.
	// Returns an error if the workflow execution fails.
	Execute(ctx context.Context) error

	// GetAllResults returns all activity results from the workflow.
	// The returned map is a copy and safe for concurrent access.
	GetAllResults() map[ActivityID]*Result
}

// Compose creates a composite workflow that executes multiple workflows in sequence.
// Each workflow is executed in order, and execution continues even if a workflow fails.
// If multiple workflows fail, their errors are combined into a single error.
// Results from all workflows are aggregated and accessible via GetAllResults().
func Compose(workflows ...Workflow) Workflow {
	return &compositeWorkflow{
		workflows: workflows,
	}
}

// compositeWorkflow executes multiple workflows in sequence.
type compositeWorkflow struct {
	workflows []Workflow
}

// Execute runs all workflows in sequence, continuing even if one fails.
// Returns a combined error if any workflow fails.
func (c *compositeWorkflow) Execute(ctx context.Context) error {
	var errors []error

	for i, w := range c.workflows {
		if err := w.Execute(ctx); err != nil {
			errors = append(errors, fmt.Errorf("workflow %d failed: %w", i, err))
		}
	}

	// Return combined error if any workflows failed
	if len(errors) > 0 {
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("%d workflow(s) failed:\n  - %s", len(errors), strings.Join(errMsgs, "\n  - "))
	}

	return nil
}

// GetAllResults returns all activity results from all composed workflows in real-time.
// This queries each workflow directly and merges the results, so it reflects
// the current state even during execution.
func (c *compositeWorkflow) GetAllResults() map[ActivityID]*Result {
	// Merge results from all workflows
	results := make(map[ActivityID]*Result)
	for _, w := range c.workflows {
		for id, result := range w.GetAllResults() {
			results[id] = result
		}
	}
	return results
}
