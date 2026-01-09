package logging

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapturingLoggerHook_LoggerForActivity_ReturnsLogger(t *testing.T) {
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)
	require.NotNil(t, hook)

	logger := hook.LoggerForActivity(baseLogger, "test-activity")
	require.NotNil(t, logger)
}

func TestCapturingLoggerHook_LoggerForActivity_Unique(t *testing.T) {
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)

	logger1 := hook.LoggerForActivity(baseLogger, "activity1")
	logger2 := hook.LoggerForActivity(baseLogger, "activity2")

	// Verify different logger instances
	assert.NotSame(t, logger1, logger2, "Each activity should get a unique logger instance")

	// Log with each logger
	logger1.Info("log from activity1")
	logger2.Info("log from activity2")

	// Verify logs are tagged correctly
	logs1 := collector.GetLogs("activity1")
	logs2 := collector.GetLogs("activity2")

	require.Len(t, logs1, 1)
	require.Len(t, logs2, 1)

	assert.Equal(t, "log from activity1", logs1[0].Message)
	assert.Equal(t, "log from activity2", logs2[0].Message)

	// Verify all logs in shared collector
	allLogs := collector.GetAllLogs()
	require.Len(t, allLogs, 2)

	assert.Contains(t, allLogs, "activity1")
	assert.Contains(t, allLogs, "activity2")
}

func TestCapturingLoggerHook_ConcurrentLogging(t *testing.T) {
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)

	const numActivities = 10
	const logsPerActivity = 50

	var wg sync.WaitGroup
	wg.Add(numActivities)

	// Launch concurrent goroutines, each with its own activity logger
	for i := 0; i < numActivities; i++ {
		go func(activityNum int) {
			defer wg.Done()
			activityID := "activity-" + string(rune('0'+activityNum))
			logger := hook.LoggerForActivity(baseLogger, activityID)

			for j := 0; j < logsPerActivity; j++ {
				logger.Info("concurrent message", "activity", activityNum, "log", j)
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

func TestCapturingLoggerHook_WithAttributes(t *testing.T) {
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)

	logger := hook.LoggerForActivity(baseLogger, "test-activity")

	// Add attributes via .With() and log
	contextLogger := logger.With("component", "test-component", "version", "1.0")
	contextLogger.Info("test message", "extra", "data")

	// Verify attributes are captured
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "test message", log.Message)
	assert.Equal(t, "test-component", log.Attributes["component"])
	assert.Equal(t, "1.0", log.Attributes["version"])
	assert.Equal(t, "data", log.Attributes["extra"])
}

func TestCapturingLoggerHook_MultipleLogLevels(t *testing.T) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Enable all levels
	}
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), opts))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)

	logger := hook.LoggerForActivity(baseLogger, "test-activity")

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Verify all levels captured
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 4)

	assert.Equal(t, "DEBUG", logs[0].Level)
	assert.Equal(t, "INFO", logs[1].Level)
	assert.Equal(t, "WARN", logs[2].Level)
	assert.Equal(t, "ERROR", logs[3].Level)
}

func TestCapturingLoggerHook_ReuseActivityID(t *testing.T) {
	baseLogger := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
	collector := NewLogCollector()
	hook := NewCapturingLoggerHook(collector)

	// Create two loggers with the same activity ID
	logger1 := hook.LoggerForActivity(baseLogger, "same-activity")
	logger2 := hook.LoggerForActivity(baseLogger, "same-activity")

	logger1.Info("first message")
	logger2.Info("second message")

	// Both logs should be under the same activity ID
	logs := collector.GetLogs("same-activity")
	require.Len(t, logs, 2)
	assert.Equal(t, "first message", logs[0].Message)
	assert.Equal(t, "second message", logs[1].Message)
}
