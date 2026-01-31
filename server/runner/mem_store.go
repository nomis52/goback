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

// Runs returns a copy of all runs.
func (s *MemoryStore) Runs() []runStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]runStatus, len(s.runs))
	copy(result, s.runs)
	return result
}

// Save stores a run in memory.
func (s *MemoryStore) Save(run runStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure ID is populated
	if run.ID == "" {
		run.ID = run.CalculateID()
	}

	// Prepend to keep most recent first
	s.runs = append([]runStatus{run}, s.runs...)
	return nil
}
