package logging

import (
	"context"
	"log/slog"
	"strings"
)

// redactedMarker replaces any sensitive value found in a captured log entry.
const redactedMarker = "***REDACTED***"

// CapturingHandler wraps an slog.Handler to capture log records while passing them through.
type CapturingHandler struct {
	underlying slog.Handler      // Pass-through to actual handler
	collector  *LogCollector     // Stores captured logs
	activityID string            // Auto-tagged on all logs
	attrs      []slog.Attr       // Attributes added via WithAttrs
	groups     []string          // Groups added via WithGroup
	redactor   *strings.Replacer // Masks sensitive values in captured entries; nil disables
}

// CapturingHandlerOption configures a CapturingHandler.
type CapturingHandlerOption func(*CapturingHandler)

// WithRedactedValues masks every occurrence of the given values in captured log
// messages and string attributes. Empty values are ignored. Only the captured copy
// is redacted; records passed through to the underlying handler are unchanged.
func WithRedactedValues(values []string) CapturingHandlerOption {
	return func(h *CapturingHandler) {
		pairs := make([]string, 0, 2*len(values))
		for _, v := range values {
			if v == "" {
				continue
			}
			pairs = append(pairs, v, redactedMarker)
		}
		if len(pairs) == 0 {
			h.redactor = nil
			return
		}
		h.redactor = strings.NewReplacer(pairs...)
	}
}

// NewCapturingHandler creates a new CapturingHandler that captures logs to the collector
// while passing them through to the underlying handler.
func NewCapturingHandler(underlying slog.Handler, collector *LogCollector, activityID string, opts ...CapturingHandlerOption) *CapturingHandler {
	h := &CapturingHandler{
		underlying: underlying,
		collector:  collector,
		activityID: activityID,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
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
		Message:    h.redact(r.Message),
		Attributes: make(map[string]interface{}, r.NumAttrs()+len(h.attrs)),
	}

	// 2. Add attributes from WithAttrs calls (in order)
	for _, attr := range h.attrs {
		entry.Attributes[attr.Key] = h.redactValue(resolveValue(attr.Value))
	}

	// 3. Add attributes from this specific log call
	r.Attrs(func(a slog.Attr) bool {
		entry.Attributes[a.Key] = h.redactValue(resolveValue(a.Value))
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
		redactor:   h.redactor,                    // Same redaction
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
		redactor:   h.redactor,
	}
}

// redact masks sensitive values in s. It is a no-op when no values are configured.
func (h *CapturingHandler) redact(s string) string {
	if h.redactor == nil {
		return s
	}
	return h.redactor.Replace(s)
}

// redactValue masks sensitive values inside a resolved attribute value. Strings are
// redacted directly and groups (maps) are redacted recursively; other kinds are
// returned unchanged.
func (h *CapturingHandler) redactValue(v interface{}) interface{} {
	if h.redactor == nil {
		return v
	}
	switch val := v.(type) {
	case string:
		return h.redactor.Replace(val)
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(val))
		for k, inner := range val {
			redacted[k] = h.redactValue(inner)
		}
		return redacted
	default:
		return v
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
