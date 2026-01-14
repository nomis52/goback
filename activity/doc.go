// Package activity provides activity-scoped status reporting for workflow activities.
//
// The package implements a status line mechanism that allows activities to communicate
// their current progress via unstructured text messages. Status messages are both logged
// and collected for real-time UI display.
//
// # Architecture
//
// The package follows the handler/writer pattern similar to the standard library's
// log/slog package:
//
//   - StatusLine: Writes status messages (analogous to slog.Logger)
//   - StatusHandler: Receives and stores status updates (analogous to slog.Handler)
//
// # Usage
//
// Activities receive a StatusLine via dependency injection and use it to report
// their current state:
//
//	type PowerOnPBS struct {
//	    StatusLine *activity.StatusLine
//	}
//
//	func (a *PowerOnPBS) Execute(ctx context.Context) error {
//	    a.StatusLine.Set("checking PBS power status")
//	    // ... perform work
//	    a.StatusLine.Set("waiting for PBS to boot")
//	    // ... more work
//	    return nil
//	}
//
// Workflow constructors create a StatusHandler and provide a factory to the orchestrator
// that creates StatusLines for each activity:
//
//	handler := activity.NewStatusHandler()
//	workflow.Provide(o, func(id workflow.ActivityID) *activity.StatusLine {
//	    return activity.NewStatusLine(id, logger, handler)
//	})
//
// The StatusHandler can be queried for current status of all activities:
//
//	statuses := handler.All() // Returns map[workflow.ActivityID]string
//
// # Error Capturing
//
// The CaptureError helper function automatically updates status on error:
//
//	return activity.CaptureError(a.StatusLine, func() error {
//	    // ... work that might fail
//	    return someOperation()
//	})
//
// If the function returns an error, the status line is automatically updated
// with the error message.
package activity
