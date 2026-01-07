package handlers

import (
	"errors"
	"net/http"

	"github.com/nomis52/goback/server/runner"
)

// RunHandler handles requests to trigger a backup run.
type RunHandler struct {
	runner BackupRunner
}

// NewRunHandler creates a new RunHandler.
func NewRunHandler(r BackupRunner) *RunHandler {
	return &RunHandler{
		runner: r,
	}
}

// ServeHTTP implements http.Handler.
func (h *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.runner.Run()
	if err != nil {
		if errors.Is(err, runner.ErrRunInProgress) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
