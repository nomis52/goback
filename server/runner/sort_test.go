package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func timePtr(t time.Time) *time.Time { return &t }

func typesOf(execs []ActivityExecution) []string {
	types := make([]string, len(execs))
	for i, e := range execs {
		types[i] = e.Type
	}
	return types
}

func TestSortByExecutionOrder_ByStartTime(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	// Provided out of execution order (and not alphabetical either).
	execs := []ActivityExecution{
		{Type: "PowerOnPBS", StartTime: timePtr(base.Add(1 * time.Second))},
		{Type: "BackupVMs", StartTime: timePtr(base.Add(3 * time.Second))},
		{Type: "BackupDirs", StartTime: timePtr(base.Add(2 * time.Second))},
	}

	sortByExecutionOrder(execs)

	assert.Equal(t, []string{"PowerOnPBS", "BackupDirs", "BackupVMs"}, typesOf(execs))
}

func TestSortByExecutionOrder_NotStartedGoLast(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	zero := time.Time{}

	execs := []ActivityExecution{
		// Skipped/pending: zero start time.
		{Type: "Zebra", StartTime: timePtr(zero)},
		{Type: "Started", StartTime: timePtr(base.Add(1 * time.Second))},
		// Never even assigned a start time (nil pointer).
		{Type: "Apple", StartTime: nil},
	}

	sortByExecutionOrder(execs)

	// Started activity first; not-started ones after, alphabetical by type.
	assert.Equal(t, []string{"Started", "Apple", "Zebra"}, typesOf(execs))
}

func TestSortByExecutionOrder_EqualStartTimesFallBackToType(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	// Parallel activities with identical start times sort by type for stability.
	execs := []ActivityExecution{
		{Type: "Charlie", StartTime: timePtr(base)},
		{Type: "Alpha", StartTime: timePtr(base)},
		{Type: "Bravo", StartTime: timePtr(base)},
	}

	sortByExecutionOrder(execs)

	assert.Equal(t, []string{"Alpha", "Bravo", "Charlie"}, typesOf(execs))
}
