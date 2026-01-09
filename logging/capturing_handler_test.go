package logging

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCapturingHandler(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)

	handler := NewCapturingHandler(underlying, collector, "test-activity")
	require.NotNil(t, handler)
	assert.Equal(t, "test-activity", handler.activityID)
}

func TestCapturingHandler_Enabled(t *testing.T) {
	collector := NewLogCollector()

	// Create underlying handler with Info level
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), opts)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	ctx := context.Background()

	// CapturingHandler always returns true to capture all log levels
	// regardless of the underlying handler's level setting
	assert.True(t, handler.Enabled(ctx, slog.LevelDebug))
	assert.True(t, handler.Enabled(ctx, slog.LevelInfo))
	assert.True(t, handler.Enabled(ctx, slog.LevelWarn))
	assert.True(t, handler.Enabled(ctx, slog.LevelError))
}

func TestCapturingHandler_Handle_CapturesLogs(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Info("test message", "key1", "value1", "key2", 42)

	// Verify log was captured
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "INFO", log.Level)
	assert.Equal(t, "test message", log.Message)
	assert.Equal(t, "value1", log.Attributes["key1"])
	assert.Equal(t, int64(42), log.Attributes["key2"]) // Integers are int64
}

func TestCapturingHandler_Handle_PassesThrough(t *testing.T) {
	collector := NewLogCollector()
	var buf bytes.Buffer
	underlying := slog.NewJSONHandler(&buf, nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Info("test message", "key", "value")

	// Verify log was written to underlying handler
	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
}

func TestCapturingHandler_WithAttrs_PreservesCapturing(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	// Create logger with additional attributes
	logger := slog.New(handler).With("component", "test-component")
	logger.Info("test message", "extra", "data")

	// Verify logs were captured with both base and added attributes
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "INFO", log.Level)
	assert.Equal(t, "test message", log.Message)
	assert.Equal(t, "test-component", log.Attributes["component"])
	assert.Equal(t, "data", log.Attributes["extra"])
}

func TestCapturingHandler_WithAttrs_ReturnsCapturingHandler(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	// Call WithAttrs
	newHandler := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})

	// Verify it returns a CapturingHandler (not the underlying handler)
	capturingHandler, ok := newHandler.(*CapturingHandler)
	require.True(t, ok, "WithAttrs should return a *CapturingHandler")
	assert.Equal(t, "test-activity", capturingHandler.activityID)
	assert.Equal(t, collector, capturingHandler.collector)
}

func TestCapturingHandler_WithGroup_PreservesCapturing(t *testing.T) {
	collector := NewLogCollector()
	var buf bytes.Buffer
	underlying := slog.NewJSONHandler(&buf, nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	// Create logger with a group
	logger := slog.New(handler).WithGroup("mygroup")
	logger.Info("test message", "key", "value")

	// Verify log was captured
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "INFO", log.Level)
	assert.Equal(t, "test message", log.Message)

	// Grouped attributes should be nested
	// Note: slog groups create nested structures in attributes
	assert.Contains(t, buf.String(), "mygroup")
}

func TestCapturingHandler_WithGroup_ReturnsCapturingHandler(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	// Call WithGroup
	newHandler := handler.WithGroup("testgroup")

	// Verify it returns a CapturingHandler (not the underlying handler)
	capturingHandler, ok := newHandler.(*CapturingHandler)
	require.True(t, ok, "WithGroup should return a *CapturingHandler")
	assert.Equal(t, "test-activity", capturingHandler.activityID)
	assert.Equal(t, collector, capturingHandler.collector)
}

func TestCapturingHandler_MultipleLogLevels(t *testing.T) {
	collector := NewLogCollector()
	// Create underlying handler with Debug level to capture all logs
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), opts)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Verify all levels were captured
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 4)

	assert.Equal(t, "DEBUG", logs[0].Level)
	assert.Equal(t, "INFO", logs[1].Level)
	assert.Equal(t, "WARN", logs[2].Level)
	assert.Equal(t, "ERROR", logs[3].Level)
}

func TestCapturingHandler_ConcurrentLogging(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	const numGoroutines = 50
	const logsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent goroutines logging
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				logger.Info("concurrent message", "goroutine", goroutineID, "log", j)
			}
		}(i)
	}

	wg.Wait()

	// Verify all logs were captured
	logs := collector.GetLogs("test-activity")
	assert.Len(t, logs, numGoroutines*logsPerGoroutine)
}

func TestCapturingHandler_ChainedWithCalls(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	// Chain multiple .With() calls (mimics real usage in codebase)
	logger := slog.New(handler).
		With("component", "pbsclient").
		With("host", "localhost")

	logger.Info("chained message", "extra", "field")

	// Verify log was captured with all attributes
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "INFO", log.Level)
	assert.Equal(t, "chained message", log.Message)
	assert.Equal(t, "pbsclient", log.Attributes["component"])
	assert.Equal(t, "localhost", log.Attributes["host"])
	assert.Equal(t, "field", log.Attributes["extra"])
}

func TestCapturingHandler_StructuredAttributes(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Info("structured test",
		"string", "value",
		"int", 42,
		"bool", true,
		"float", 3.14,
		"time", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	// Verify structured attributes are preserved
	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	attrs := logs[0].Attributes
	assert.Equal(t, "value", attrs["string"])
	assert.Equal(t, int64(42), attrs["int"])
	assert.Equal(t, true, attrs["bool"])
	assert.InDelta(t, 3.14, attrs["float"], 0.01)
	assert.NotNil(t, attrs["time"])
}

func TestCapturingHandler_EmptyMessage(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Info("", "key", "value")

	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)
	assert.Equal(t, "", logs[0].Message)
	assert.Equal(t, "value", logs[0].Attributes["key"])
}

func TestCapturingHandler_NoAttributes(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	logger.Info("message with no attributes")

	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)
	assert.Equal(t, "message with no attributes", logs[0].Message)
	assert.Empty(t, logs[0].Attributes)
}

func TestCapturingHandler_ErrorAttribute(t *testing.T) {
	collector := NewLogCollector()
	underlying := slog.NewJSONHandler(bytes.NewBuffer(nil), nil)
	handler := NewCapturingHandler(underlying, collector, "test-activity")

	logger := slog.New(handler)
	testErr := fmt.Errorf("test error message")

	logger.Info("test message", "error", testErr, "attempt", 5)

	logs := collector.GetLogs("test-activity")
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, "test message", log.Message)
	assert.Equal(t, "test error message", log.Attributes["error"])
	assert.Equal(t, int64(5), log.Attributes["attempt"])
}
