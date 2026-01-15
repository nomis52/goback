package cron

import (
	"errors"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

const (
	triggerSeparator      = ";"
	workflowSeparator     = ":"
	workflowListSeparator = ","
)

// TriggerSpec represents a parsed trigger specification with workflows and cron schedule.
type TriggerSpec struct {
	Workflows []string
	CronSpec  string
}

// ParseTriggerSpecs parses a multi-trigger specification string into individual trigger specs.
// The format is: workflow1,workflow2:cron_expression;workflow3:cron_expression2
//
// Example:
//
//	"backup,poweroff:0 2 * * *;test:0 3 * * *"
//
// Returns an error if:
//   - Any trigger is missing workflows or cron expression
//   - Any workflow name is not in availableWorkflows
//   - Any cron expression is invalid
//   - Any trigger has duplicate workflows
func ParseTriggerSpecs(spec string, availableWorkflows map[string]bool) ([]TriggerSpec, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, errors.New("cron spec cannot be empty")
	}

	// Split by semicolon for multiple triggers
	triggerStrs := strings.Split(spec, triggerSeparator)
	specs := make([]TriggerSpec, 0, len(triggerStrs))

	for _, triggerStr := range triggerStrs {
		triggerStr = strings.TrimSpace(triggerStr)
		if triggerStr == "" {
			continue // Skip empty triggers (e.g., trailing semicolon)
		}

		triggerSpec, err := parseSingleTrigger(triggerStr, availableWorkflows)
		if err != nil {
			return nil, err
		}
		specs = append(specs, triggerSpec)
	}

	if len(specs) == 0 {
		return nil, errors.New("no valid triggers found in cron spec")
	}

	return specs, nil
}

// parseSingleTrigger parses a single trigger specification.
func parseSingleTrigger(triggerStr string, availableWorkflows map[string]bool) (TriggerSpec, error) {
	// Split by colon to separate workflows from cron expression
	parts := strings.Split(triggerStr, workflowSeparator)
	if len(parts) != 2 {
		return TriggerSpec{}, fmt.Errorf("invalid trigger spec: expected format 'workflows:cron', got '%s'", triggerStr)
	}

	workflowsStr := strings.TrimSpace(parts[0])
	cronSpec := strings.TrimSpace(parts[1])

	// Validate workflows part is not empty
	if workflowsStr == "" {
		return TriggerSpec{}, fmt.Errorf("invalid trigger spec: missing workflows in '%s'", triggerStr)
	}

	// Validate cron spec part is not empty
	if cronSpec == "" {
		return TriggerSpec{}, fmt.Errorf("invalid trigger spec: missing cron schedule in '%s'", triggerStr)
	}

	// Parse workflows
	workflowStrs := strings.Split(workflowsStr, workflowListSeparator)
	workflows := make([]string, 0, len(workflowStrs))
	seen := make(map[string]bool, len(workflowStrs))

	for _, w := range workflowStrs {
		w = strings.TrimSpace(w)
		if w == "" {
			continue // Skip empty workflow names
		}

		// Check for duplicates within this trigger
		if seen[w] {
			return TriggerSpec{}, fmt.Errorf("invalid trigger spec: duplicate workflow '%s' in '%s'", w, triggerStr)
		}
		seen[w] = true

		// Validate workflow exists
		if !availableWorkflows[w] {
			return TriggerSpec{}, fmt.Errorf("invalid trigger spec: unknown workflow '%s' in '%s' (available: %s)",
				w, triggerStr, formatAvailableWorkflows(availableWorkflows))
		}

		workflows = append(workflows, w)
	}

	if len(workflows) == 0 {
		return TriggerSpec{}, fmt.Errorf("invalid trigger spec: no valid workflows in '%s'", triggerStr)
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronSpec); err != nil {
		return TriggerSpec{}, fmt.Errorf("invalid trigger spec: invalid cron expression in '%s': %w", triggerStr, err)
	}

	return TriggerSpec{
		Workflows: workflows,
		CronSpec:  cronSpec,
	}, nil
}

// formatAvailableWorkflows formats the available workflows for error messages.
func formatAvailableWorkflows(availableWorkflows map[string]bool) string {
	workflows := make([]string, 0, len(availableWorkflows))
	for w := range availableWorkflows {
		workflows = append(workflows, w)
	}
	return strings.Join(workflows, ", ")
}
