package runner

import "sync"

// MemoryStore keeps run history in memory only (no persistence).
type MemoryStore struct {
	runs []runStatus
	mu   sync.Mutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs: make([]runStatus, 0),
	}
}

// History returns all runs as summaries.
func (s *MemoryStore) History() []RunSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]RunSummary, len(s.runs))
	for i, run := range s.runs {
		result[i] = run.RunSummary
	}
	return result
}

// Logs returns the activity executions for a specific run.
func (s *MemoryStore) Logs(id string) []ActivityExecution {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, run := range s.runs {
		if run.ID == id {
			result := make([]ActivityExecution, len(run.ActivityExecutions))
			copy(result, run.ActivityExecutions)
			return result
		}
	}
	return nil
}

// Save stores a run in memory.
func (s *MemoryStore) Save(summary RunSummary, logs []ActivityExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure ID is populated
	if summary.ID == "" {
		summary.ID = summary.CalculateID()
	}

	run := runStatus{
		RunSummary:         summary,
		ActivityExecutions: logs,
	}

	// Prepend to keep most recent first
	s.runs = append([]runStatus{run}, s.runs...)
	return nil
}
