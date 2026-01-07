package handlers

import (
	"net/http"
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

// ServeHTTP implements http.Handler.
func (h *ResultsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	results := h.provider.GetResults()
	if results == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, results)
}
