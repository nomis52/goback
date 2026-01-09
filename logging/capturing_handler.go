package logging

import (
	"context"
	"log/slog"
)

// CapturingHandler wraps an slog.Handler to capture log records while passing them through.
type CapturingHandler struct {
	underlying slog.Handler  // Pass-through to actual handler
	collector  *LogCollector // Stores captured logs
	activityID string        // Auto-tagged on all logs
	attrs      []slog.Attr   // Attributes added via WithAttrs
	groups     []string      // Groups added via WithGroup
}

// NewCapturingHandler creates a new CapturingHandler that captures logs to the collector
// while passing them through to the underlying handler.
func NewCapturingHandler(underlying slog.Handler, collector *LogCollector, activityID string) *CapturingHandler {
	return &CapturingHandler{
		underlying: underlying,
		collector:  collector,
		activityID: activityID,
	}
}

// Enabled always returns true to capture all log levels regardless of the underlying handler's level.
// This ensures we capture DEBUG, INFO, WARN, and ERROR logs even if the base logger filters some levels.
// The underlying handler will still filter for output in Handle().
func (h *CapturingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle captures the log record and then passes it to the underlying handler.
func (h *CapturingHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1. Extract structured data from slog.Record
	entry := LogEntry{
		Time:       r.Time,
		Level:      r.Level.String(),
		Message:    r.Message,
		Attributes: make(map[string]interface{}, r.NumAttrs()+len(h.attrs)),
	}

	// 2. Add attributes from WithAttrs calls (in order)
	for _, attr := range h.attrs {
		entry.Attributes[attr.Key] = resolveValue(attr.Value)
	}

	// 3. Add attributes from this specific log call
	r.Attrs(func(a slog.Attr) bool {
		entry.Attributes[a.Key] = resolveValue(a.Value)
		return true // Continue iteration
	})

	// 4. Store in collector (thread-safe)
	h.collector.AddLog(h.activityID, entry)

	// 5. Pass through to underlying handler
	return h.underlying.Handle(ctx, r)
}

// WithAttrs returns a new CapturingHandler with additional attributes.
// CRITICAL: Must return a new CapturingHandler (not the underlying handler)
// to preserve log capturing through .With() chains.
func (h *CapturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Accumulate attributes from parent handler
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &CapturingHandler{
		underlying: h.underlying.WithAttrs(attrs), // Chain the attrs
		collector:  h.collector,                   // Same collector
		activityID: h.activityID,                  // Same activity ID
		attrs:      newAttrs,                      // Accumulated attributes
		groups:     h.groups,                      // Same groups
	}
}

// WithGroup returns a new CapturingHandler with a group name.
// CRITICAL: Must return a new CapturingHandler (not the underlying handler)
// to preserve log capturing through .With() chains.
func (h *CapturingHandler) WithGroup(name string) slog.Handler {
	// Accumulate groups from parent handler
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &CapturingHandler{
		underlying: h.underlying.WithGroup(name),
		collector:  h.collector,
		activityID: h.activityID,
		attrs:      h.attrs,   // Keep same attributes
		groups:     newGroups, // Accumulated groups
	}
}

// resolveValue converts a slog.Value to a JSON-serializable value.
// This handles special cases like errors which need to be converted to strings.
func resolveValue(v slog.Value) interface{} {
	// Resolve any LogValuer implementations
	v = v.Resolve()

	// Handle different kinds of values
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time()
	case slog.KindAny:
		// For Any kind, check if it's an error and convert to string
		any := v.Any()
		if err, ok := any.(error); ok {
			return err.Error()
		}
		return any
	case slog.KindGroup:
		// Handle groups by recursively resolving attributes
		attrs := v.Group()
		group := make(map[string]interface{}, len(attrs))
		for _, attr := range attrs {
			group[attr.Key] = resolveValue(attr.Value)
		}
		return group
	default:
		// Fallback to Any() for unknown kinds
		return v.Any()
	}
}
