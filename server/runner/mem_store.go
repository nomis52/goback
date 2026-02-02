package runner

import "sync"

// MemoryStore keeps run history in memory only (no persistence).
type MemoryStore struct {
	summaries []RunSummary
	logs      map[string][]ActivityExecution
	mu        sync.Mutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		summaries: make([]RunSummary, 0),
		logs:      make(map[string][]ActivityExecution),
	}
}

// History returns all runs as summaries.
func (s *MemoryStore) History() []RunSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]RunSummary, len(s.summaries))
	copy(result, s.summaries)
	return result
}

// Logs returns the activity executions for a specific run.
func (s *MemoryStore) Logs(id string) []ActivityExecution {
	s.mu.Lock()
	defer s.mu.Unlock()

	if logs, ok := s.logs[id]; ok {
		result := make([]ActivityExecution, len(logs))
		copy(result, logs)
		return result
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

	// Prepend to keep most recent first
	s.summaries = append([]RunSummary{summary}, s.summaries...)
	s.logs[summary.ID] = logs
	return nil
}
