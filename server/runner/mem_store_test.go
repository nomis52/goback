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
	assert.Empty(t, store.History())
}

func TestMemoryStore_Save(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	run := runStatus{
		RunSummary: RunSummary{
			State:     RunStateIdle,
			StartedAt: &now,
			EndedAt:   &now,
			Error:     "",
		},
	}

	err := store.Save(run.RunSummary, run.ActivityExecutions)
	require.NoError(t, err)

	history := store.History()
	require.Len(t, history, 1)

	// ID should have been populated
	run.ID = run.CalculateID()
	assert.Equal(t, run.RunSummary, history[0])
}

func TestMemoryStore_SaveMultiple(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	for i := 0; i < 5; i++ {
		runTime := now.Add(time.Duration(i) * time.Hour)
		run := runStatus{
			RunSummary: RunSummary{
				State:     RunStateIdle,
				StartedAt: &runTime,
				EndedAt:   &runTime,
			},
		}
		err := store.Save(run.RunSummary, run.ActivityExecutions)
		require.NoError(t, err)
	}

	history := store.History()
	assert.Len(t, history, 5)

	// Should be in reverse order (most recent first)
	for i := 0; i < len(history)-1; i++ {
		assert.True(t, history[i].StartedAt.After(*history[i+1].StartedAt))
	}
}

func TestMemoryStore_History_ReturnsCopy(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	run := runStatus{
		RunSummary: RunSummary{
			State:     RunStateIdle,
			StartedAt: &now,
			EndedAt:   &now,
			Error:     "",
		},
	}
	err := store.Save(run.RunSummary, run.ActivityExecutions)
	require.NoError(t, err)

	// Get history twice
	history1 := store.History()
	history2 := store.History()

	require.Len(t, history1, 1)
	require.Len(t, history2, 1)

	// Modifying one shouldn't affect the other
	history1[0].Error = "modified"

	// Expected summary should have its ID populated
	expectedSummary := run.RunSummary
	expectedSummary.ID = expectedSummary.CalculateID()
	assert.Equal(t, expectedSummary, history2[0], "modifying one slice should not affect the other")
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
			run := runStatus{
				RunSummary: RunSummary{
					State:     RunStateIdle,
					StartedAt: &now,
					EndedAt:   &now,
				},
			}
			err := store.Save(run.RunSummary, run.ActivityExecutions)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	history := store.History()
	assert.Len(t, history, numGoroutines)
}

func TestMemoryStore_NoLimit(t *testing.T) {
	store := NewMemoryStore()

	// Memory store doesn't enforce a limit
	now := time.Now()
	for i := 0; i < 100; i++ {
		runTime := now.Add(time.Duration(i) * time.Second)
		run := runStatus{
			RunSummary: RunSummary{
				State:     RunStateIdle,
				StartedAt: &runTime,
				EndedAt:   &runTime,
			},
		}
		err := store.Save(run.RunSummary, run.ActivityExecutions)
		require.NoError(t, err)
	}

	history := store.History()
	assert.Len(t, history, 100, "memory store should not limit runs")
}
