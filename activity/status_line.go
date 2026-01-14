package activity

import (
	"log/slog"

	"github.com/nomis52/goback/workflow"
)

// StatusLine logs status with activity context AND updates the shared handler.
// This struct is created by factories during dependency injection and bound to a specific activity.
// Activities use this to report their status with a clean API: statusLine.Set("message")
type StatusLine struct {
	logger     *slog.Logger
	handler    *StatusHandler
	activityID workflow.ActivityID
}

// NewStatusLine creates a status line bound to an activity ID.
// Used by factory functions registered with orchestrator.
// The handler parameter is optional - if nil, status updates are only logged.
func NewStatusLine(activityID workflow.ActivityID, logger *slog.Logger, handler *StatusHandler) *StatusLine {
	return &StatusLine{
		logger:     logger,
		handler:    handler,
		activityID: activityID,
	}
}

// Set logs the status with activity context and updates the handler if present.
// Called by activities to report their current status.
func (sl *StatusLine) Set(status string) {
	sl.logger.Info(status, "activity", sl.activityID.String())
	if sl.handler != nil {
		sl.handler.Set(sl.activityID, status)
	}
}
