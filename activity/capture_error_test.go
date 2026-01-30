package activity

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/nomis52/goback/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureError(t *testing.T) {
	activityID := workflow.ActivityID{Module: "test/module", Type: "TestActivity"}

	t.Run("sets error status on failure", func(t *testing.T) {
		handler := NewStatusHandler()
		statusLine := NewStatusLine(activityID, slog.Default(), handler)

		err := CaptureError(statusLine, func() error {
			return errors.New("operation failed")
		})

		require.Error(t, err)
		assert.Equal(t, "❌ operation failed", handler.Get(activityID))
	})

	t.Run("returns nil on success", func(t *testing.T) {
		handler := NewStatusHandler()
		statusLine := NewStatusLine(activityID, slog.Default(), handler)

		err := CaptureError(statusLine, func() error {
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, "", handler.Get(activityID))
	})

	t.Run("handles nil statusLine", func(t *testing.T) {
		err := CaptureError(nil, func() error {
			return errors.New("some error")
		})

		require.Error(t, err)
	})
}
