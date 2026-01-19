package cron

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunnable is a test implementation of Runnable.
type mockRunnable struct {
	runCount  atomic.Int32
	runErr    error
	workflows []string
}

func (m *mockRunnable) Run(workflows []string) error {
	m.runCount.Add(1)
	m.workflows = workflows
	return m.runErr
}

func TestNewCronTrigger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}

	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{
			name:    "valid spec - daily at 2am",
			spec:    "0 2 * * *",
			wantErr: false,
		},
		{
			name:    "valid spec - every hour",
			spec:    "0 * * * *",
			wantErr: false,
		},
		{
			name:    "valid spec - every minute",
			spec:    "* * * * *",
			wantErr: false,
		},
		{
			name:    "invalid spec - empty",
			spec:    "",
			wantErr: true,
		},
		{
			name:    "invalid spec - wrong format",
			spec:    "not a cron spec",
			wantErr: true,
		},
		{
			name:    "invalid spec - too few fields",
			spec:    "0 2 *",
			wantErr: true,
		},
		{
			name:    "invalid spec - invalid value",
			spec:    "60 2 * * *",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := func() error {
				return runnable.Run([]string{"backup", "poweroff"})
			}
			trigger, err := NewCronTrigger(tt.spec, callback, logger)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidCronSpec)
				assert.Nil(t, trigger)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, trigger)
				assert.Equal(t, tt.spec, trigger.spec)
			}
		})
	}
}

func TestCronTrigger_NextRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}
	callback := func() error {
		return runnable.Run([]string{"backup", "poweroff"})
	}

	trigger, err := NewCronTrigger("0 2 * * *", callback, logger)
	require.NoError(t, err)

	nextRun := trigger.NextRun()
	assert.True(t, nextRun.After(time.Now()), "next run should be in the future")
	assert.Equal(t, 2, nextRun.Hour(), "next run should be at 2am")
	assert.Equal(t, 0, nextRun.Minute(), "next run should be at minute 0")
}

func TestCronTrigger_Start_CancellationStopsLoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runnable := &mockRunnable{}
	callback := func() error {
		return runnable.Run([]string{"test"})
	}

	// Use a spec that would run every minute
	trigger, err := NewCronTrigger("* * * * *", callback, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	trigger.Start(ctx)

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel should cause the goroutine to exit
	cancel()

	// Give goroutine time to exit
	time.Sleep(10 * time.Millisecond)

	// Run count should be 0 since we cancelled before the first scheduled run
	assert.Equal(t, int32(0), runnable.runCount.Load())
}
