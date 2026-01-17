package handlers

import (
	"net/http"
)

// AvailableWorkflowsResponse is the JSON response for /api/workflows.
type AvailableWorkflowsResponse struct {
	Workflows []string `json:"workflows"`
}

// WorkflowProvider provides access to available workflows.
type WorkflowProvider interface {
	AvailableWorkflows() map[string]bool
}

// AvailableWorkflowsHandler handles requests for the available workflows endpoint.
type AvailableWorkflowsHandler struct {
	runner WorkflowProvider
}

// NewAvailableWorkflowsHandler creates a new AvailableWorkflowsHandler.
func NewAvailableWorkflowsHandler(runner WorkflowProvider) *AvailableWorkflowsHandler {
	return &AvailableWorkflowsHandler{
		runner: runner,
	}
}

// ServeHTTP implements http.Handler.
func (h *AvailableWorkflowsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	available := h.runner.AvailableWorkflows()

	// Convert map to slice
	workflows := make([]string, 0, len(available))
	for name := range available {
		workflows = append(workflows, name)
	}

	resp := AvailableWorkflowsResponse{
		Workflows: workflows,
	}

	writeJSON(w, http.StatusOK, resp)
}
