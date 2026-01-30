package activity

import (
	"log/slog"
	"testing"

	"github.com/nomis52/goback/workflow"
	"github.com/stretchr/testify/assert"
)

func TestStatusLine(t *testing.T) {
	activityID := workflow.ActivityID{Module: "test/module", Type: "TestActivity"}

	t.Run("set updates handler", func(t *testing.T) {
		handler := NewStatusHandler()
		sl := NewStatusLine(activityID, slog.Default(), handler)

		sl.Set("working")
		assert.Equal(t, "working", handler.Get(activityID))
	})

	t.Run("set with nil handler does not panic", func(t *testing.T) {
		sl := NewStatusLine(activityID, slog.Default(), nil)
		sl.Set("working") // should not panic
	})
}
