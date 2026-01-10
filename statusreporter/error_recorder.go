package statusreporter

import (
	"fmt"

	"github.com/nomis52/goback/workflow"
)

// RecordError executes the provided function and records any error as a status.
// This helper ensures that when an activity fails, the error message is visible
// in the status reporter for monitoring and debugging.
//
// Usage in activities:
//
//	func (a *MyActivity) Execute(ctx context.Context) error {
//	    return statusreporter.RecordError(a, a.StatusReporter, func() error {
//	        a.StatusReporter.SetStatus(a, "doing work")
//	        // ... do actual work
//	        return nil
//	    })
//	}
func RecordError(activity workflow.Activity, sr *StatusReporter, f func() error) error {
	if err := f(); err != nil {
		sr.SetStatus(activity, fmt.Sprintf("❌ %v", err))
		return err
	}
	return nil
}
