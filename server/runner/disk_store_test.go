package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertRunSummaryEqual compares RunSummary structs handling time.Time properly.
func assertRunSummaryEqual(t *testing.T, expected, actual RunSummary, msgAndArgs ...interface{}) {
	t.Helper()

	// Ensure IDs match if expected ID is set, otherwise check if actual ID is populated
	if expected.ID != "" {
		assert.Equal(t, expected.ID, actual.ID, msgAndArgs...)
	} else if expected.StartedAt != nil {
		assert.NotEmpty(t, actual.ID, msgAndArgs...)
	}

	assert.Equal(t, expected.State, actual.State, msgAndArgs...)
	assert.Equal(t, expected.Error, actual.Error, msgAndArgs...)

	if expected.StartedAt == nil {
		assert.Nil(t, actual.StartedAt, msgAndArgs...)
	} else {
		require.NotNil(t, actual.StartedAt, msgAndArgs...)
		assert.True(t, expected.StartedAt.Equal(*actual.StartedAt), msgAndArgs...)
	}

	if expected.EndedAt == nil {
		assert.Nil(t, actual.EndedAt, msgAndArgs...)
	} else {
		require.NotNil(t, actual.EndedAt, msgAndArgs...)
		assert.True(t, expected.EndedAt.Equal(*actual.EndedAt), msgAndArgs...)
	}
}

func TestNewDiskStore(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Should start with empty runs
	assert.Empty(t, store.History())
}

func TestDiskStore_Save(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	now := time.Now()
	summary := RunSummary{
		State:     RunStateIdle,
		StartedAt: &now,
		EndedAt:   &now,
		Error:     "",
	}

	err = store.Save(summary, nil)
	require.NoError(t, err)

	// Check file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	// Check filename format
	expectedName := now.Format("2006-01-02T15-04-05") + ".json"
	assert.Equal(t, expectedName, files[0].Name())
}

func TestDiskStore_SaveWithoutStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	summary := RunSummary{
		State:     RunStateIdle,
		StartedAt: nil, // No start time
		Error:     "",
	}

	err = store.Save(summary, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot save run without start time")
}

func TestDiskStore_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	// Save multiple runs
	now := time.Now()
	for i := 0; i < 3; i++ {
		runTime := now.Add(time.Duration(i) * time.Hour)
		summary := RunSummary{
			State:     RunStateIdle,
			StartedAt: &runTime,
			EndedAt:   &runTime,
		}
		err = store.Save(summary, nil)
		require.NoError(t, err)
	}

	// All runs should be visible
	history := store.History()
	assert.Len(t, history, 3)

	// Should be sorted by start time descending (most recent first)
	for i := 0; i < len(history)-1; i++ {
		assert.True(t, history[i].StartedAt.After(*history[i+1].StartedAt))
	}
}

func TestDiskStore_MaxCount(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	maxCount := 5
	store, err := NewDiskStore(tmpDir, maxCount, logger)
	require.NoError(t, err)

	// Save more than maxCount runs
	now := time.Now()
	for i := 0; i < 10; i++ {
		runTime := now.Add(time.Duration(i) * time.Hour)
		summary := RunSummary{
			State:     RunStateIdle,
			StartedAt: &runTime,
			EndedAt:   &runTime,
		}
		err = store.Save(summary, nil)
		require.NoError(t, err)
	}

	// Reload and verify only maxCount runs are loaded
	err = store.Reload()
	require.NoError(t, err)

	history := store.History()
	assert.Len(t, history, maxCount)

	// Should keep the most recent ones
	for i := 0; i < len(history)-1; i++ {
		assert.True(t, history[i].StartedAt.After(*history[i+1].StartedAt))
	}
}

func TestDiskStore_LoadsExistingRuns(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create a run file manually
	now := time.Now()
	summary := RunSummary{
		State:     RunStateIdle,
		StartedAt: &now,
		EndedAt:   &now,
		Error:     "",
	}

	// Save using first store
	store1, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)
	err = store1.Save(summary, nil)
	require.NoError(t, err)

	// Create new store - should load existing run
	store2, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	history := store2.History()
	assert.Len(t, history, 1)
	assertRunSummaryEqual(t, summary, history[0])
}

func TestDiskStore_IgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create some non-JSON files
	err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)

	// Create store
	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	// Should ignore non-JSON files
	assert.Empty(t, store.History())
}

func TestDiskStore_History_ReturnsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := NewDiskStore(tmpDir, 10, logger)
	require.NoError(t, err)

	now := time.Now()
	summary := RunSummary{
		State:     RunStateIdle,
		StartedAt: &now,
		EndedAt:   &now,
	}
	err = store.Save(summary, nil)
	require.NoError(t, err)

	err = store.Reload()
	require.NoError(t, err)

	// Get history twice
	history1 := store.History()
	history2 := store.History()

	// Should be copies, not the same slice
	// Check by comparing pointers using reflection or modify one and verify other unchanged
	require.Len(t, history1, 1)
	require.Len(t, history2, 1)

	// Modifying one shouldn't affect the other
	history1[0].Error = "modified"
	assertRunSummaryEqual(t, summary, history2[0], "modifying one slice should not affect the other")
}
