package runner

// StateStore manages persistence of run history.
type StateStore interface {
	// Runs returns all loaded runs.
	Runs() []RunStatus
	// Save persists a run.
	Save(RunStatus) error
}
