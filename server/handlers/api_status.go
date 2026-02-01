package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/server/cron"
	"github.com/nomis52/goback/server/runner"
	"github.com/nomis52/goback/server/types"
)

// PBSStatus contains the current power state of the PBS server.
type PBSStatus struct {
	PowerState string `json:"power_state"`
}

// NextRunResponse is the JSON response for the next run information.
type NextRunResponse struct {
	Scheduled bool       `json:"scheduled"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	Workflows []string   `json:"workflows,omitempty"`
}

// APIStatusResponse is the consolidated response for /api/status.
type APIStatusResponse struct {
	PBS PBSStatus `json:"pbs"`
	ActiveWorkflow struct {
		Status       runner.RunSummary           `json:"status"`
		ActivityLogs []runner.ActivityExecution `json:"activity_logs,omitempty"`
	} `json:"active_workflow"`
	NextRun NextRunResponse        `json:"next_run"`
	Server  types.ServerProperties `json:"server"`
}

// APIStatusProvider aggregates all the providers needed for the status endpoint.
type APIStatusProvider interface {
	IPMIController() *ipmiclient.IPMIController
	Status() (runner.RunSummary, []runner.ActivityExecution)
	NextTrigger() *cron.NextTriggerInfo
	Properties() types.ServerProperties
}

// APIStatusHandler handles requests for the consolidated status endpoint.
type APIStatusHandler struct {
	logger   *slog.Logger
	provider APIStatusProvider
}

// NewAPIStatusHandler creates a new APIStatusHandler.
func NewAPIStatusHandler(logger *slog.Logger, provider APIStatusProvider) *APIStatusHandler {
	return &APIStatusHandler{
		logger:   logger,
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *APIStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get PBS power state
	var powerStateStr string
	ctrl := h.provider.IPMIController()
	if ctrl != nil {
		state, err := ctrl.Status()
		if err != nil {
			h.logger.Error("failed to get IPMI status", "error", err)
			powerStateStr = "unknown"
		} else {
			powerStateStr = state.String()
		}
	} else {
		powerStateStr = "unknown"
	}

	// Get run status (includes live activity executions with logs and status messages)
	runSummary, activityExecutions := h.provider.Status()

	// Get next run with workflows
	nextTrigger := h.provider.NextTrigger()
	nextRunResp := NextRunResponse{}
	if nextTrigger != nil {
		nextRunResp.Scheduled = true
		nextRunResp.NextRun = &nextTrigger.Time
		nextRunResp.Workflows = nextTrigger.Workflows
	} else {
		nextRunResp.Scheduled = false
	}

	resp := APIStatusResponse{
		PBS: PBSStatus{
			PowerState: powerStateStr,
		},
		ActiveWorkflow: struct {
			Status       runner.RunSummary           `json:"status"`
			ActivityLogs []runner.ActivityExecution `json:"activity_logs,omitempty"`
		}{
			Status:       runSummary,
			ActivityLogs: activityExecutions,
		},
		NextRun: nextRunResp,
		Server:  h.provider.Properties(),
	}

	writeJSON(w, http.StatusOK, resp)
}
