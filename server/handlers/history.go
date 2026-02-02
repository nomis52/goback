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

// HistoryLogsHandler handles requests for logs of a specific run.
type HistoryLogsHandler struct {
	provider HistoryProvider
}

// NewHistoryLogsHandler creates a new HistoryLogsHandler.
func NewHistoryLogsHandler(provider HistoryProvider) *HistoryLogsHandler {
	return &HistoryLogsHandler{
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *HistoryLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing run id"})
		return
	}

	logs, err := h.provider.GetLogs(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, logs)
}
