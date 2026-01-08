package statusreporter

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/nomis52/goback/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test activities for generating ActivityIDs
type TestActivityA struct{}

func (a *TestActivityA) Init() error                  { return nil }
func (a *TestActivityA) Execute(ctx context.Context) error { return nil }

type TestActivityB struct{}

func (a *TestActivityB) Init() error                  { return nil }
func (a *TestActivityB) Execute(ctx context.Context) error { return nil }

type TestActivityC struct{}

func (a *TestActivityC) Init() error                  { return nil }
func (a *TestActivityC) Execute(ctx context.Context) error { return nil }

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)
	require.NotNil(t, reporter)
	assert.Empty(t, reporter.CurrentStatuses())
}

func TestStatusReporter_SetStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)
	activity := &TestActivityA{}

	reporter.SetStatus(activity, "waiting for server")

	statuses := reporter.CurrentStatuses()
	require.Len(t, statuses, 1)
	activityID := orchestrator.GetActivityID(activity)
	assert.Equal(t, "waiting for server", statuses[activityID.String()])
}

func TestStatusReporter_SetStatus_UpdatesExisting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)
	activity := &TestActivityA{}

	reporter.SetStatus(activity, "waiting for server")
	reporter.SetStatus(activity, "server is online")

	statuses := reporter.CurrentStatuses()
	require.Len(t, statuses, 1)
	activityID := orchestrator.GetActivityID(activity)
	assert.Equal(t, "server is online", statuses[activityID.String()])
}

func TestStatusReporter_SetStatus_MultipleActivities(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)
	activityA := &TestActivityA{}
	activityB := &TestActivityB{}
	activityC := &TestActivityC{}

	reporter.SetStatus(activityA, "waiting for server")
	reporter.SetStatus(activityB, "backing up VM 1/10")
	reporter.SetStatus(activityC, "backing up /data")

	idA := orchestrator.GetActivityID(activityA)
	idB := orchestrator.GetActivityID(activityB)
	idC := orchestrator.GetActivityID(activityC)

	statuses := reporter.CurrentStatuses()
	require.Len(t, statuses, 3)
	assert.Equal(t, "waiting for server", statuses[idA.String()])
	assert.Equal(t, "backing up VM 1/10", statuses[idB.String()])
	assert.Equal(t, "backing up /data", statuses[idC.String()])
}

func TestStatusReporter_CurrentStatuses_ReturnsCopy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)
	activity := &TestActivityA{}

	reporter.SetStatus(activity, "waiting for server")

	statuses1 := reporter.CurrentStatuses()
	statuses2 := reporter.CurrentStatuses()

	require.Len(t, statuses1, 1)
	require.Len(t, statuses2, 1)

	activityID := orchestrator.GetActivityID(activity)
	// Modify one copy - should not affect the other
	statuses1[activityID.String()] = "modified"
	assert.Equal(t, "waiting for server", statuses2[activityID.String()])

	// And should not affect the reporter's internal state
	statuses3 := reporter.CurrentStatuses()
	assert.Equal(t, "waiting for server", statuses3[activityID.String()])
}

func TestStatusReporter_Concurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := New(logger)

	activityA := &TestActivityA{}
	activityB := &TestActivityB{}
	activityC := &TestActivityC{}

	var wg sync.WaitGroup

	// Test concurrent status updates for different activities
	numUpdates := 100
	for i := 0; i < numUpdates; i++ {
		wg.Add(3)
		go func(iteration int) {
			defer wg.Done()
			reporter.SetStatus(activityA, "running A")
		}(i)
		go func(iteration int) {
			defer wg.Done()
			reporter.SetStatus(activityB, "running B")
		}(i)
		go func(iteration int) {
			defer wg.Done()
			reporter.SetStatus(activityC, "running C")
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	idA := orchestrator.GetActivityID(activityA)
	idB := orchestrator.GetActivityID(activityB)
	idC := orchestrator.GetActivityID(activityC)

	statuses := reporter.CurrentStatuses()
	require.Len(t, statuses, 3)
	assert.Equal(t, "running A", statuses[idA.String()])
	assert.Equal(t, "running B", statuses[idB.String()])
	assert.Equal(t, "running C", statuses[idC.String()])
}
