package activities

import (
	"fmt"

	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/statusreporter"
)

// RecordError executes the provided function and records any error as a status.
// This helper ensures that when an activity fails, the error message is visible
// in the status reporter for monitoring and debugging.
func RecordError(activity orchestrator.Activity, sr *statusreporter.StatusReporter, f func() error) error {
	if err := f(); err != nil {
		sr.SetStatus(activity, fmt.Sprintf("❌ %v", err))
		return err
	}
	return nil
}
