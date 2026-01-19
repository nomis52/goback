package cron

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	serverconfig "github.com/nomis52/goback/server/config"
)

func TestNewCronTriggerManager_ValidSingleTrigger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	triggers := []serverconfig.CronTrigger{
		{
			Workflows: []string{"backup"},
			Schedule:  "0 2 * * *",
		},
	}

	manager, err := NewCronTriggerManager(triggers, runnable, logger)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Len(t, manager.triggers, 1)
}

func TestNewCronTriggerManager_ValidMultipleTriggers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	triggers := []serverconfig.CronTrigger{
		{
			Workflows: []string{"backup", "poweroff"},
			Schedule:  "0 2 * * *",
		},
		{
			Workflows: []string{"test"},
			Schedule:  "0 3 * * *",
		},
	}

	manager, err := NewCronTriggerManager(triggers, runnable, logger)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Len(t, manager.triggers, 2)
}

func TestNewCronTriggerManager_InvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	tests := []struct {
		name     string
		triggers []serverconfig.CronTrigger
		wantErr  string
	}{
		{
			name: "empty workflows",
			triggers: []serverconfig.CronTrigger{
				{
					Workflows: []string{},
					Schedule:  "0 2 * * *",
				},
			},
			wantErr: "no workflows specified",
		},
		{
			name: "invalid cron schedule",
			triggers: []serverconfig.CronTrigger{
				{
					Workflows: []string{"backup"},
					Schedule:  "invalid-cron",
				},
			},
			wantErr: "creating trigger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewCronTriggerManager(tt.triggers, runnable, logger)
			require.Error(t, err)
			assert.Nil(t, manager)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCronTriggerManager_NextRun_SingleTrigger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	triggers := []serverconfig.CronTrigger{
		{
			Workflows: []string{"backup"},
			Schedule:  "0 2 * * *",
		},
	}

	manager, err := NewCronTriggerManager(triggers, runnable, logger)
	require.NoError(t, err)

	nextRun := manager.NextRun()
	assert.True(t, nextRun.After(time.Now()), "next run should be in the future")
	assert.Equal(t, 2, nextRun.Hour(), "next run should be at 2am")
}

func TestCronTriggerManager_NextRun_MultipleTriggers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	triggers := []serverconfig.CronTrigger{
		{
			Workflows: []string{"backup"},
			Schedule:  "0 2 * * *",
		},
		{
			Workflows: []string{"test"},
			Schedule:  "0 14 * * *",
		},
		{
			Workflows: []string{"poweroff"},
			Schedule:  "0 20 * * *",
		},
	}

	manager, err := NewCronTriggerManager(triggers, runnable, logger)
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

	// Create manager with no triggers (valid case)
	manager, err := NewCronTriggerManager([]serverconfig.CronTrigger{}, nil, logger)
	require.NoError(t, err)

	nextRun := manager.NextRun()
	assert.True(t, nextRun.IsZero(), "should return zero time with no triggers")
}

func TestCronTriggerManager_Start(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	triggers := []serverconfig.CronTrigger{
		{
			Workflows: []string{"backup"},
			Schedule:  "* * * * *",
		},
		{
			Workflows: []string{"test"},
			Schedule:  "* * * * *",
		},
	}

	manager, err := NewCronTriggerManager(triggers, runnable, logger)
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
