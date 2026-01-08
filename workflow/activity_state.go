package workflow

// ActivityState represents the execution state of an activity
type ActivityState int

const (
	// NotStarted indicates the activity has been registered but not yet processed
	// If validation fails, all activities remain in NotStarted state
	NotStarted ActivityState = iota

	// Pending indicates the activity is waiting for its dependencies to complete
	// (execution phase has started, but this activity hasn't run yet)
	Pending

	// Running indicates the activity is currently executing
	Running

	// Skipped indicates the activity was prevented from running during execution
	// (dependency failed, context cancelled, etc.)
	Skipped

	// Completed indicates the activity has finished execution
	// The activity may have succeeded or failed - check the Error field
	Completed
)

// String returns a human-readable representation of the ActivityState
func (s ActivityState) String() string {
	switch s {
	case NotStarted:
		return "not_started"
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Skipped:
		return "skipped"
	case Completed:
		return "completed"
	default:
		return "unknown"
	}
}
