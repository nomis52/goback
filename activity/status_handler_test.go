package activity

import (
	"testing"

	"github.com/nomis52/goback/workflow"
	"github.com/stretchr/testify/assert"
)

func TestStatusHandler(t *testing.T) {
	activityID := workflow.ActivityID{Module: "test/module", Type: "TestActivity"}

	t.Run("set and get", func(t *testing.T) {
		handler := NewStatusHandler()
		handler.Set(activityID, "running")
		assert.Equal(t, "running", handler.Get(activityID))
	})

	t.Run("get returns empty for unknown activity", func(t *testing.T) {
		handler := NewStatusHandler()
		assert.Equal(t, "", handler.Get(activityID))
	})

	t.Run("all returns copy of statuses", func(t *testing.T) {
		handler := NewStatusHandler()
		handler.Set(activityID, "done")

		all := handler.All()
		assert.Equal(t, "done", all[activityID])

		// Modifying returned map doesn't affect handler
		all[activityID] = "modified"
		assert.Equal(t, "done", handler.Get(activityID))
	})
}
