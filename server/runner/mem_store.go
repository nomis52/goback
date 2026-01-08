package runner

import "sync"

// MemoryStore keeps run history in memory only (no persistence).
type MemoryStore struct {
	runs []RunStatus
	mu   sync.Mutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs: make([]RunStatus, 0),
	}
}

// Runs returns a copy of all runs.
func (s *MemoryStore) Runs() []RunStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]RunStatus, len(s.runs))
	copy(result, s.runs)
	return result
}

// Save stores a run in memory.
func (s *MemoryStore) Save(run RunStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prepend to keep most recent first
	s.runs = append([]RunStatus{run}, s.runs...)
	return nil
}
