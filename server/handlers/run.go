package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/nomis52/goback/server/runner"
)

// RunRequest defines the request body for POST /run.
type RunRequest struct {
	Workflows []string `json:"workflows"`
}

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
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("invalid JSON: %v", err),
		})
		return
	}

	if len(req.Workflows) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "workflows array cannot be empty",
		})
		return
	}

	// Check for duplicate workflows
	seen := make(map[string]bool, len(req.Workflows))
	for _, wf := range req.Workflows {
		if seen[wf] {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("duplicate workflow %q in request", wf),
			})
			return
		}
		seen[wf] = true
	}

	err := h.runner.Run(req.Workflows)
	if err != nil {
		if errors.Is(err, runner.ErrRunInProgress) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: err.Error(),
			})
			return
		}
		// Unknown workflow or validation error
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
