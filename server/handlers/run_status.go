package handlers

import (
	"net/http"
)

// RunStatusHandler handles requests for the current run status.
type RunStatusHandler struct {
	provider RunStatusProvider
}

// NewRunStatusHandler creates a new RunStatusHandler.
func NewRunStatusHandler(provider RunStatusProvider) *RunStatusHandler {
	return &RunStatusHandler{
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *RunStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.provider.Status()
	writeJSON(w, http.StatusOK, status)
}
