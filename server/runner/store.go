package runner

// StateStore manages persistence of run history.
type StateStore interface {
	// History returns all loaded runs as summaries.
	History() []RunSummary
	// Logs returns the activity executions for a specific run.
	Logs(string) []ActivityExecution
	// Save persists a run.
	Save(RunSummary, []ActivityExecution) error
}
