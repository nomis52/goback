package runner

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	require.NotNil(t, store)

	// Should start with empty runs
	assert.Empty(t, store.Runs())
}

func TestMemoryStore_Save(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	run := RunStatus{
		State:     RunStateIdle,
		StartedAt: &now,
		EndedAt:   &now,
		Error:     "",
	}

	err := store.Save(run)
	require.NoError(t, err)

	runs := store.Runs()
	require.Len(t, runs, 1)

	// ID should have been populated
	run.ID = run.CalculateID()
	assert.Equal(t, run, runs[0])
}

func TestMemoryStore_SaveMultiple(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	for i := 0; i < 5; i++ {
		runTime := now.Add(time.Duration(i) * time.Hour)
		run := RunStatus{
			State:     RunStateIdle,
			StartedAt: &runTime,
			EndedAt:   &runTime,
		}
		err := store.Save(run)
		require.NoError(t, err)
	}

	runs := store.Runs()
	assert.Len(t, runs, 5)

	// Should be in reverse order (most recent first)
	for i := 0; i < len(runs)-1; i++ {
		assert.True(t, runs[i].StartedAt.After(*runs[i+1].StartedAt))
	}
}

func TestMemoryStore_Runs_ReturnsCopy(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	run := RunStatus{
		State:     RunStateIdle,
		StartedAt: &now,
		EndedAt:   &now,
		Error:     "",
	}
	err := store.Save(run)
	require.NoError(t, err)

	// Get runs twice
	runs1 := store.Runs()
	runs2 := store.Runs()

	require.Len(t, runs1, 1)
	require.Len(t, runs2, 1)

	// Modifying one shouldn't affect the other
	runs1[0].Error = "modified"

	// Expected run should have its ID populated
	expectedRun := run
	expectedRun.ID = expectedRun.CalculateID()
	assert.Equal(t, expectedRun, runs2[0], "modifying one slice should not affect the other")
}

func TestMemoryStore_Concurrent(t *testing.T) {
	store := NewMemoryStore()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent saves
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			now := time.Now()
			run := RunStatus{
				State:     RunStateIdle,
				StartedAt: &now,
				EndedAt:   &now,
			}
			err := store.Save(run)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	runs := store.Runs()
	assert.Len(t, runs, numGoroutines)
}

func TestMemoryStore_NoLimit(t *testing.T) {
	store := NewMemoryStore()

	// Memory store doesn't enforce a limit
	now := time.Now()
	for i := 0; i < 100; i++ {
		runTime := now.Add(time.Duration(i) * time.Second)
		run := RunStatus{
			State:     RunStateIdle,
			StartedAt: &runTime,
			EndedAt:   &runTime,
		}
		err := store.Save(run)
		require.NoError(t, err)
	}

	runs := store.Runs()
	assert.Len(t, runs, 100, "memory store should not limit runs")
}
