package cron

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCronTriggerManager_ValidSingleTrigger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	manager, err := NewCronTriggerManager("backup:0 2 * * *", runnable, logger, testAvailableWorkflows)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Len(t, manager.triggers, 1)
}

func TestNewCronTriggerManager_ValidMultipleTriggers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	manager, err := NewCronTriggerManager(
		"backup,poweroff:0 2 * * *;test:0 3 * * *",
		runnable,
		logger,
		testAvailableWorkflows,
	)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Len(t, manager.triggers, 2)
}

func TestNewCronTriggerManager_InvalidSpec(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	tests := []struct {
		name string
		spec string
	}{
		{
			name: "empty spec",
			spec: "",
		},
		{
			name: "missing colon",
			spec: "backup",
		},
		{
			name: "invalid cron",
			spec: "backup:invalid",
		},
		{
			name: "unknown workflow",
			spec: "unknown:0 2 * * *",
		},
		{
			name: "duplicate workflow in trigger",
			spec: "backup,backup:0 2 * * *",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewCronTriggerManager(tt.spec, runnable, logger, testAvailableWorkflows)
			require.Error(t, err)
			assert.Nil(t, manager)
		})
	}
}

func TestNewCronTriggerManager_WorkflowValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	// Unknown workflow should fail
	manager, err := NewCronTriggerManager("unknown:0 2 * * *", runnable, logger, testAvailableWorkflows)
	require.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "unknown workflow")
	assert.Contains(t, err.Error(), "available:")
}

func TestCronTriggerManager_NextRun_SingleTrigger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	manager, err := NewCronTriggerManager("backup:0 2 * * *", runnable, logger, testAvailableWorkflows)
	require.NoError(t, err)

	nextRun := manager.NextRun()
	assert.True(t, nextRun.After(time.Now()), "next run should be in the future")
	assert.Equal(t, 2, nextRun.Hour(), "next run should be at 2am")
}

func TestCronTriggerManager_NextRun_MultipleTriggers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	// Create triggers at different hours: 2am, 14pm (2pm), 20pm (8pm)
	manager, err := NewCronTriggerManager(
		"backup:0 2 * * *;test:0 14 * * *;poweroff:0 20 * * *",
		runnable,
		logger,
		testAvailableWorkflows,
	)
	require.NoError(t, err)

	nextRun := manager.NextRun()
	assert.True(t, nextRun.After(time.Now()), "next run should be in the future")

	// Get next run for each trigger
	trigger1Next := manager.triggers[0].NextRun()
	trigger2Next := manager.triggers[1].NextRun()
	trigger3Next := manager.triggers[2].NextRun()

	// Find the earliest
	earliest := trigger1Next
	if trigger2Next.Before(earliest) {
		earliest = trigger2Next
	}
	if trigger3Next.Before(earliest) {
		earliest = trigger3Next
	}

	// Manager should return the earliest
	assert.Equal(t, earliest, nextRun)
}

func TestCronTriggerManager_NextRun_NoTriggers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create manager with no triggers (edge case - shouldn't happen in practice)
	manager := &CronTriggerManager{
		triggers: []*CronTrigger{},
		logger:   logger,
	}

	nextRun := manager.NextRun()
	assert.True(t, nextRun.IsZero(), "should return zero time with no triggers")
}

func TestCronTriggerManager_Start(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	manager, err := NewCronTriggerManager(
		"backup:* * * * *;test:* * * * *",
		runnable,
		logger,
		testAvailableWorkflows,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should return immediately
	manager.Start(ctx)

	// Give goroutines time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel should stop all triggers
	cancel()

	// Give goroutines time to exit
	time.Sleep(10 * time.Millisecond)

	// Verify no runs completed (we cancelled before first scheduled run)
	assert.Equal(t, int32(0), runnable.runCount.Load())
}

func TestCronTriggerManager_ComplexSpec(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	// Complex spec with multiple workflows per trigger
	manager, err := NewCronTriggerManager(
		"backup,poweroff:0 2 * * *;test:0 3 * * *;backup:0 14 * * *",
		runnable,
		logger,
		testAvailableWorkflows,
	)
	require.NoError(t, err)
	assert.Len(t, manager.triggers, 3)

	// Verify all triggers are scheduled
	for _, trigger := range manager.triggers {
		nextRun := trigger.NextRun()
		assert.True(t, nextRun.After(time.Now()), "each trigger should have a future next run")
	}
}
