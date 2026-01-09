package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/server/runner"
)

// PBSStatus contains the current power state of the PBS server.
type PBSStatus struct {
	PowerState string `json:"power_state"`
}

// NextRunResponse is the JSON response for the next run information.
type NextRunResponse struct {
	Scheduled bool       `json:"scheduled"`
	NextRun   *time.Time `json:"next_run,omitempty"`
}

// APIStatusResponse is the consolidated response for /api/status.
type APIStatusResponse struct {
	PBS     PBSStatus        `json:"pbs"`
	Run     runner.RunStatus `json:"run"`     // Includes ActivityExecutions with Status field
	NextRun NextRunResponse  `json:"next_run"`
}

// APIStatusProvider aggregates all the providers needed for the status endpoint.
type APIStatusProvider interface {
	IPMIController() *ipmiclient.IPMIController
	Status() runner.RunStatus
	NextRun() *time.Time
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
	runStatus := h.provider.Status()

	// Get next run
	nextRun := h.provider.NextRun()
	nextRunResp := NextRunResponse{
		Scheduled: nextRun != nil,
		NextRun:   nextRun,
	}

	resp := APIStatusResponse{
		PBS: PBSStatus{
			PowerState: powerStateStr,
		},
		Run:     runStatus,
		NextRun: nextRunResp,
	}

	writeJSON(w, http.StatusOK, resp)
}
