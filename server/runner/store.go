package runner

// StateStore manages persistence of run history.
type StateStore interface {
	// Runs returns all loaded runs.
	Runs() []runStatus
	// Save persists a run.
	Save(runStatus) error
}
