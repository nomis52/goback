package logging

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogCollector(t *testing.T) {
	collector := NewLogCollector()
	require.NotNil(t, collector)
	assert.NotNil(t, collector.logs)
}

func TestLogCollector_AddLog(t *testing.T) {
	collector := NewLogCollector()

	entry := LogEntry{
		Time:       time.Now(),
		Level:      "info",
		Message:    "test message",
		Attributes: map[string]interface{}{"key": "value"},
	}

	collector.AddLog("activity1", entry)

	logs := collector.GetLogs("activity1")
	require.Len(t, logs, 1)
	assert.Equal(t, entry.Level, logs[0].Level)
	assert.Equal(t, entry.Message, logs[0].Message)
	assert.Equal(t, entry.Attributes["key"], logs[0].Attributes["key"])
}

func TestLogCollector_AddLog_Concurrent(t *testing.T) {
	collector := NewLogCollector()
	const numGoroutines = 100
	const logsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent goroutines adding logs
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				entry := LogEntry{
					Time:       time.Now(),
					Level:      "info",
					Message:    "concurrent test",
					Attributes: map[string]interface{}{"goroutine": goroutineID, "log": j},
				}
				collector.AddLog("activity1", entry)
			}
		}(i)
	}

	wg.Wait()

	// Verify all logs were captured
	logs := collector.GetLogs("activity1")
	assert.Len(t, logs, numGoroutines*logsPerGoroutine)
}

func TestLogCollector_GetLogs(t *testing.T) {
	collector := NewLogCollector()

	entry1 := LogEntry{Time: time.Now(), Level: "info", Message: "first", Attributes: map[string]interface{}{}}
	entry2 := LogEntry{Time: time.Now(), Level: "error", Message: "second", Attributes: map[string]interface{}{}}

	collector.AddLog("activity1", entry1)
	collector.AddLog("activity1", entry2)

	logs := collector.GetLogs("activity1")
	require.Len(t, logs, 2)
	assert.Equal(t, "first", logs[0].Message)
	assert.Equal(t, "second", logs[1].Message)
}

func TestLogCollector_GetLogs_NonExistent(t *testing.T) {
	collector := NewLogCollector()

	logs := collector.GetLogs("nonexistent")
	assert.Nil(t, logs)
}

func TestLogCollector_GetLogs_ReturnsCopy(t *testing.T) {
	collector := NewLogCollector()

	entry := LogEntry{Time: time.Now(), Level: "info", Message: "test", Attributes: map[string]interface{}{}}
	collector.AddLog("activity1", entry)

	// Get logs and modify the returned slice
	logs := collector.GetLogs("activity1")
	require.Len(t, logs, 1)

	logs[0].Message = "modified"

	// Get logs again and verify original is unchanged
	logsAgain := collector.GetLogs("activity1")
	assert.Equal(t, "test", logsAgain[0].Message, "GetLogs should return a copy, not the original")
}

func TestLogCollector_GetAllLogs(t *testing.T) {
	collector := NewLogCollector()

	entry1 := LogEntry{Time: time.Now(), Level: "info", Message: "activity1 log", Attributes: map[string]interface{}{}}
	entry2 := LogEntry{Time: time.Now(), Level: "warn", Message: "activity2 log", Attributes: map[string]interface{}{}}

	collector.AddLog("activity1", entry1)
	collector.AddLog("activity2", entry2)

	allLogs := collector.GetAllLogs()
	require.Len(t, allLogs, 2)
	assert.Contains(t, allLogs, "activity1")
	assert.Contains(t, allLogs, "activity2")
	assert.Len(t, allLogs["activity1"], 1)
	assert.Len(t, allLogs["activity2"], 1)
}

func TestLogCollector_GetAllLogs_ReturnsCopy(t *testing.T) {
	collector := NewLogCollector()

	entry := LogEntry{Time: time.Now(), Level: "info", Message: "test", Attributes: map[string]interface{}{}}
	collector.AddLog("activity1", entry)

	// Get all logs and modify the returned map
	allLogs := collector.GetAllLogs()
	require.Len(t, allLogs, 1)

	allLogs["activity1"][0].Message = "modified"

	// Get all logs again and verify original is unchanged
	allLogsAgain := collector.GetAllLogs()
	assert.Equal(t, "test", allLogsAgain["activity1"][0].Message, "GetAllLogs should return a deep copy")
}

func TestLogCollector_Clear(t *testing.T) {
	collector := NewLogCollector()

	entry1 := LogEntry{Time: time.Now(), Level: "info", Message: "log1", Attributes: map[string]interface{}{}}
	entry2 := LogEntry{Time: time.Now(), Level: "info", Message: "log2", Attributes: map[string]interface{}{}}

	collector.AddLog("activity1", entry1)
	collector.AddLog("activity2", entry2)

	// Verify logs exist
	allLogs := collector.GetAllLogs()
	assert.Len(t, allLogs, 2)

	// Clear and verify empty
	collector.Clear()

	allLogsAfterClear := collector.GetAllLogs()
	assert.Len(t, allLogsAfterClear, 0)
}

func TestLogCollector_MultipleActivitiesConcurrent(t *testing.T) {
	collector := NewLogCollector()
	const numActivities = 10
	const logsPerActivity = 50

	var wg sync.WaitGroup
	wg.Add(numActivities)

	// Launch concurrent goroutines, each logging to a different activity
	for i := 0; i < numActivities; i++ {
		go func(activityNum int) {
			defer wg.Done()
			activityID := "activity" + string(rune('0'+activityNum))
			for j := 0; j < logsPerActivity; j++ {
				entry := LogEntry{
					Time:       time.Now(),
					Level:      "debug",
					Message:    "concurrent multi-activity test",
					Attributes: map[string]interface{}{"activity": activityNum, "log": j},
				}
				collector.AddLog(activityID, entry)
			}
		}(i)
	}

	wg.Wait()

	// Verify all activities have correct number of logs
	allLogs := collector.GetAllLogs()
	assert.Len(t, allLogs, numActivities)

	for activityID, logs := range allLogs {
		assert.Len(t, logs, logsPerActivity, "Activity %s should have %d logs", activityID, logsPerActivity)
	}
}
