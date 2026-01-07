package handlers

import (
	"net/http"
	"time"
)

// ResultsHandler handles requests for activity results.
type ResultsHandler struct {
	provider ResultsProvider
}

// NewResultsHandler creates a new ResultsHandler.
func NewResultsHandler(provider ResultsProvider) *ResultsHandler {
	return &ResultsHandler{
		provider: provider,
	}
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

// ServeHTTP implements http.Handler.
func (h *ResultsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	results := h.provider.GetResults()
	if results == nil {
		writeJSON(w, http.StatusOK, []activityResult{})
		return
	}

	response := make([]activityResult, 0, len(results))
	for id, result := range results {
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
		response = append(response, ar)
	}

	writeJSON(w, http.StatusOK, response)
}
