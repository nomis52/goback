package handlers

import (
	"log/slog"
	"net/http"
)

// ReloadHandler handles requests to reload configuration from disk.
type ReloadHandler struct {
	logger   *slog.Logger
	reloader Reloader
}

// NewReloadHandler creates a new ReloadHandler.
func NewReloadHandler(logger *slog.Logger, reloader Reloader) *ReloadHandler {
	return &ReloadHandler{
		logger:   logger,
		reloader: reloader,
	}
}

// ServeHTTP implements http.Handler.
func (h *ReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("reloading configuration")

	if err := h.reloader.Reload(); err != nil {
		h.logger.Error("failed to reload configuration", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to reload configuration: " + err.Error(),
		})
		return
	}

	h.logger.Info("configuration reloaded successfully")
	w.WriteHeader(http.StatusNoContent)
}
