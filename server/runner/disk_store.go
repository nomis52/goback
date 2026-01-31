package runner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// DiskStore persists run history to disk as JSON files.
type DiskStore struct {
	dir      string
	logger   *slog.Logger
	maxCount int
	runs     []RunStatus // protected by mu
	mu       sync.Mutex
}

// NewDiskStore creates a new disk-backed store.
// The directory is created if it doesn't exist, and existing runs are loaded.
func NewDiskStore(dir string, maxCount int, logger *slog.Logger) (*DiskStore, error) {
	s := &DiskStore{
		dir:      dir,
		logger:   logger,
		maxCount: maxCount,
		runs:     make([]RunStatus, 0),
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing runs
	runs, err := s.load()
	if err != nil {
		logger.Warn("failed to load existing runs", "error", err)
		// Continue without existing data
	} else {
		s.runs = runs
	}

	return s, nil
}

// Runs returns a copy of all loaded runs.
func (s *DiskStore) Runs() []RunStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]RunStatus, len(s.runs))
	copy(result, s.runs)
	return result
}

// Save persists a run to disk and updates the in-memory representation.
func (s *DiskStore) Save(run RunStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.StartedAt == nil {
		return fmt.Errorf("cannot save run without start time")
	}

	// Use timestamp as filename: 2006-01-02T15-04-05.json
	filename := run.StartedAt.Format("2006-01-02T15-04-05") + ".json"
	path := filepath.Join(s.dir, filename)

	// Ensure ID is populated
	if run.ID == "" {
		run.ID = run.CalculateID()
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write run file: %w", err)
	}

	// Add to in-memory representation (prepend to keep most recent first)
	s.runs = append([]RunStatus{run}, s.runs...)

	// Enforce max count limit
	if len(s.runs) > s.maxCount {
		s.runs = s.runs[:s.maxCount]
	}

	s.logger.Debug("saved run to disk", "path", path)
	return nil
}

// Reload re-loads all runs from disk.
func (s *DiskStore) Reload() error {
	runs, err := s.load()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = runs

	return nil
}

// load loads all runs from disk.
func (s *DiskStore) load() ([]RunStatus, error) {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	// Count JSON files to pre-size the slice
	jsonFileCount := 0
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			jsonFileCount++
		}
	}

	// Pre-allocate slice to avoid reallocations
	capacity := jsonFileCount
	if capacity > s.maxCount {
		capacity = s.maxCount
	}
	runs := make([]RunStatus, 0, capacity)

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			s.logger.Warn("failed to read run file", "file", path, "error", err)
			continue
		}

		var run RunStatus
		if err := json.Unmarshal(data, &run); err != nil {
			s.logger.Warn("failed to parse run file", "file", path, "error", err)
			continue
		}

		// Ensure ID is populated
		if run.ID == "" {
			run.ID = run.CalculateID()
		}

		runs = append(runs, run)
	}

	// Sort by start time descending (most recent first)
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].StartedAt == nil {
			return false
		}
		if runs[j].StartedAt == nil {
			return true
		}
		return runs[i].StartedAt.After(*runs[j].StartedAt)
	})

	// Limit to max count
	if len(runs) > s.maxCount {
		runs = runs[:s.maxCount]
	}

	s.logger.Info("loaded run history from disk", "count", len(runs))

	return runs, nil
}
