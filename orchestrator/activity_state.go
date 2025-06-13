package orchestrator

// ActivityState represents the execution state of an activity
type ActivityState int

const (
	// NotStarted indicates the activity has not begun execution
	NotStarted ActivityState = iota
	
	// Running indicates the activity is currently executing
	Running
	
	// Completed indicates the activity has finished execution
	// The activity may have succeeded or failed - check the Error field
	Completed
)

// String returns a human-readable representation of the ActivityState
func (s ActivityState) String() string {
	switch s {
	case NotStarted:
		return "not_started"
	case Running:
		return "running"
	case Completed:
		return "completed"
	default:
		return "unknown"
	}
}

// IsCompleted returns true if the activity has finished execution
func (s ActivityState) IsCompleted() bool {
	return s == Completed
}

// IsRunning returns true if the activity is currently executing
func (s ActivityState) IsRunning() bool {
	return s == Running
}
