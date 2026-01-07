package handlers

import (
	"net/http"
)

// HistoryHandler handles requests for the run history.
type HistoryHandler struct {
	provider HistoryProvider
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(provider HistoryProvider) *HistoryHandler {
	return &HistoryHandler{
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	history := h.provider.History()
	writeJSON(w, http.StatusOK, history)
}
