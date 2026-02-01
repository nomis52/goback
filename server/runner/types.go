package runner

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

// WorkflowFactory creates a workflow with the given parameters.
type WorkflowFactory func(params workflows.Params) (workflow.Workflow, error)

// RunState represents the current state of a backup run.
type RunState int

const (
	// RunStateIdle indicates no backup is running.
	RunStateIdle RunState = iota
	// RunStateRunning indicates a backup is in progress.
	RunStateRunning
)

// String returns the string representation of the run state.
func (s RunState) String() string {
	switch s {
	case RunStateIdle:
		return "idle"
	case RunStateRunning:
		return "running"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler.
func (s RunState) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *RunState) UnmarshalJSON(data []byte) error {
	str := string(data)
	// Remove quotes
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	switch str {
	case "idle":
		*s = RunStateIdle
	case "running":
		*s = RunStateRunning
	default:
		*s = RunStateIdle
	}
	return nil
}

// RunSummary contains summary information about a backup run.
type RunSummary struct {
	// ID is a stable identifier for the run.
	ID string `json:"id,omitempty"`
	// State is the current state of the run.
	State RunState `json:"state"`
	// Workflows is the list of workflows that were/are being executed.
	Workflows []string `json:"workflows,omitempty"`
	// StartedAt is when the run started. Nil if no run has occurred.
	StartedAt *time.Time `json:"started_at,omitempty"`
	// EndedAt is when the run ended. Nil if run is in progress or no run has occurred.
	EndedAt *time.Time `json:"ended_at,omitempty"`
	// Error contains the error message if the run failed. Empty on success.
	Error string `json:"error,omitempty"`
}

// runStatus is an internal type that manages the state of the current or last run.
type runStatus struct {
	RunSummary
}

// runRecord is an internal type used for JSON serialization of a run to/from disk.
type runRecord struct {
	RunSummary
	ActivityExecutions []ActivityExecution `json:"activity_executions,omitempty"`
}

// CalculateID generates a stable ID for the run based on its start time and workflows.
func (s *RunSummary) CalculateID() string {
	if s.StartedAt == nil {
		return ""
	}

	// Use Unix timestamp (seconds) for stability
	data := fmt.Sprintf("%d:%s", s.StartedAt.Unix(), strings.Join(s.Workflows, ","))
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// ActivityExecution captures all details for a single activity execution.
type ActivityExecution struct {
	// Module is the activity's module path.
	Module string `json:"module"`
	// Type is the activity's type name.
	Type string `json:"type"`
	// State is the activity's state (NotStarted, Pending, Running, Completed, Skipped).
	State string `json:"state"`
	// Status is the human-readable progress message (e.g., "backing up VM 3/10", "waiting for server").
	Status string `json:"status,omitempty"`
	// Error contains the error message if the activity failed. Empty on success.
	Error string `json:"error,omitempty"`
	// StartTime is when the activity started execution.
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime is when the activity completed execution.
	EndTime *time.Time `json:"end_time,omitempty"`
	// Logs contains all log entries captured from this activity.
	Logs []logging.LogEntry `json:"logs,omitempty"`
}
