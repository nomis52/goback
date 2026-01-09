package logging

import (
	"log/slog"
)

// LoggerHook creates activity-specific loggers by wrapping a base logger.
// This interface allows the orchestrator to remain generic while supporting
// log capturing through custom implementations.
type LoggerHook interface {
	// LoggerForActivity wraps the base logger to create an activity-specific logger.
	// The base logger comes from the orchestrator's WithLogger() option.
	LoggerForActivity(baseLogger *slog.Logger, activityID string) *slog.Logger
}

// CapturingLoggerHook creates loggers that capture logs via CapturingHandler.
type CapturingLoggerHook struct {
	collector *LogCollector
}

// NewCapturingLoggerHook creates a provider that captures all activity logs.
func NewCapturingLoggerHook(collector *LogCollector) LoggerHook {
	return &CapturingLoggerHook{
		collector: collector,
	}
}

// LoggerForActivity creates an activity-specific logger with capturing enabled.
// Each call wraps the base logger with a CapturingHandler that tags logs with the activity ID.
func (p *CapturingLoggerHook) LoggerForActivity(baseLogger *slog.Logger, activityID string) *slog.Logger {
	capturingHandler := NewCapturingHandler(
		baseLogger.Handler(),
		p.collector,
		activityID,
	)
	return slog.New(capturingHandler)
}
