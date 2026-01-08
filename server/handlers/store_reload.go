package handlers

import (
	"log/slog"
	"net/http"
)

// ReloadableStore is a store that can be manually reloaded.
type ReloadableStore interface {
	Reload() error
}

// StoreReloadHandler handles requests to reload the run history store.
type StoreReloadHandler struct {
	logger *slog.Logger
	store  ReloadableStore
}

// NewStoreReloadHandler creates a new StoreReloadHandler.
func NewStoreReloadHandler(logger *slog.Logger, store ReloadableStore) *StoreReloadHandler {
	return &StoreReloadHandler{
		logger: logger,
		store:  store,
	}
}

// ServeHTTP implements http.Handler.
func (h *StoreReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("reloading run history store")

	if err := h.store.Reload(); err != nil {
		h.logger.Error("failed to reload store", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to reload store: " + err.Error(),
		})
		return
	}

	h.logger.Info("store reloaded successfully")
	w.WriteHeader(http.StatusNoContent)
}
