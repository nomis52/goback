package handlers

import (
	"net/http"

	"github.com/nomis52/goback/server/runner"
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
	noLogs := r.URL.Query().Get("no_logs") == "true"
	history := h.provider.History()

	if noLogs {
		// Create a copy of the history without logs to avoid modifying the original data
		noLogsHistory := make([]runner.RunStatus, len(history))
		for i, run := range history {
			noLogsHistory[i] = run
			if len(run.ActivityExecutions) > 0 {
				noLogsHistory[i].ActivityExecutions = make([]runner.ActivityExecution, len(run.ActivityExecutions))
				for j, exec := range run.ActivityExecutions {
					exec.Logs = nil // Strip logs
					noLogsHistory[i].ActivityExecutions[j] = exec
				}
			}
		}
		writeJSON(w, http.StatusOK, noLogsHistory)
		return
	}

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

	run, err := h.provider.GetRun(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, run)
}
