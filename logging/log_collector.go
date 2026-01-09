package logging

import (
	"sync"
	"time"
)

// LogEntry represents a single log record with structured data.
type LogEntry struct {
	Time       time.Time              `json:"time"`
	Level      string                 `json:"level"` // "debug", "info", "warn", "error"
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes"` // Structured fields
}

// LogCollector provides thread-safe storage for activity logs.
type LogCollector struct {
	mu   sync.RWMutex
	logs map[string][]LogEntry // activityID -> log entries
}

// NewLogCollector creates a new LogCollector.
func NewLogCollector() *LogCollector {
	return &LogCollector{
		logs: make(map[string][]LogEntry),
	}
}

// AddLog adds a log entry for the specified activity (thread-safe).
func (c *LogCollector) AddLog(activityID string, entry LogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logs[activityID] = append(c.logs[activityID], entry)
}

// GetLogs retrieves all log entries for a specific activity (thread-safe).
func (c *LogCollector) GetLogs(activityID string) []LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	logs, exists := c.logs[activityID]
	if !exists {
		return nil
	}

	result := make([]LogEntry, len(logs))
	copy(result, logs)
	return result
}

// GetAllLogs returns all logs grouped by activity ID (thread-safe).
// Returns a copy of the internal map to prevent external modification.
func (c *LogCollector) GetAllLogs() map[string][]LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a deep copy to prevent external modification
	result := make(map[string][]LogEntry, len(c.logs))
	for activityID, logs := range c.logs {
		logsCopy := make([]LogEntry, len(logs))
		copy(logsCopy, logs)
		result[activityID] = logsCopy
	}

	return result
}

// Clear resets the log collector, removing all stored logs (thread-safe).
func (c *LogCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logs = make(map[string][]LogEntry)
}
