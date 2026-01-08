package runner

import "time"

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

// RunStatus contains information about the current or last run.
type RunStatus struct {
	// State is the current state of the run.
	State RunState `json:"state"`
	// StartedAt is when the run started. Nil if no run has occurred.
	StartedAt *time.Time `json:"started_at,omitempty"`
	// EndedAt is when the run ended. Nil if run is in progress or no run has occurred.
	EndedAt *time.Time `json:"ended_at,omitempty"`
	// Error contains the error message if the run failed. Empty on success.
	Error string `json:"error,omitempty"`
}
