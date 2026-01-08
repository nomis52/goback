package handlers

import (
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/workflow"
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

// activityResult is the JSON-serializable representation of an activity result.
type activityResult struct {
	Module    string     `json:"module"`
	Type      string     `json:"type"`
	State     string     `json:"state"`
	Error     string     `json:"error,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// APIStatusResponse is the consolidated response for /api/status.
type APIStatusResponse struct {
	PBS      PBSStatus          `json:"pbs"`
	Run      runner.RunStatus   `json:"run"`
	NextRun  NextRunResponse    `json:"next_run"`
	Results  []activityResult   `json:"results,omitempty"`
	Statuses map[string]string  `json:"statuses,omitempty"`
}

// APIStatusProvider aggregates all the providers needed for the status endpoint.
type APIStatusProvider interface {
	IPMIController() *ipmi.IPMIController
	Status() runner.RunStatus
	NextRun() *time.Time
	GetResults() map[workflow.ActivityID]*workflow.Result
	CurrentStatuses() map[string]string
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

	// Get run status
	runStatus := h.provider.Status()

	// Get next run
	nextRun := h.provider.NextRun()
	nextRunResp := NextRunResponse{
		Scheduled: nextRun != nil,
		NextRun:   nextRun,
	}

	// Get results and statuses (only if running)
	var results []activityResult
	var statuses map[string]string
	if runStatus.State == runner.RunStateRunning {
		rawResults := h.provider.GetResults()
		if rawResults != nil && len(rawResults) > 0 {
			results = make([]activityResult, 0, len(rawResults))
			for id, result := range rawResults {
				ar := activityResult{
					Module: id.Module,
					Type:   id.Type,
					State:  result.State.String(),
				}
				if result.Error != nil {
					ar.Error = result.Error.Error()
				}
				if !result.StartTime.IsZero() {
					ar.StartTime = &result.StartTime
				}
				if !result.EndTime.IsZero() {
					ar.EndTime = &result.EndTime
				}
				results = append(results, ar)
			}
			// Sort by Type alphabetically for stable order
			sort.Slice(results, func(i, j int) bool {
				return results[i].Type < results[j].Type
			})
		}

		// Get current activity statuses
		statuses = h.provider.CurrentStatuses()
	}

	resp := APIStatusResponse{
		PBS: PBSStatus{
			PowerState: powerStateStr,
		},
		Run:      runStatus,
		NextRun:  nextRunResp,
		Results:  results,
		Statuses: statuses,
	}

	writeJSON(w, http.StatusOK, resp)
}
