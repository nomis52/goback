package activity

// CaptureError wraps an Execute function and captures any errors to the status line.
// If the function returns an error, it's prefixed with ❌ and set as the status.
//
// Usage in activities:
//
//	func (a *MyActivity) Execute(ctx context.Context) error {
//	    return activity.CaptureError(a.StatusLine, func() error {
//	        a.StatusLine.Set("doing work")
//	        // ... do actual work
//	        return nil
//	    })
//	}
func CaptureError(statusLine *StatusLine, f func() error) error {
	err := f()
	if err != nil && statusLine != nil {
		statusLine.Set("❌ " + err.Error())
	}
	return err
}
