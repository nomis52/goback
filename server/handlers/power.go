package handlers

import (
	"log/slog"
	"net/http"

	"github.com/nomis52/goback/clients/ipmiclient"
)

// PowerProvider provides access to the IPMI controller for power operations.
type PowerProvider interface {
	IPMIController() *ipmiclient.IPMIController
}

// PowerOnHandler handles requests to power on the PBS server.
type PowerOnHandler struct {
	logger   *slog.Logger
	provider PowerProvider
}

// NewPowerOnHandler creates a new PowerOnHandler.
func NewPowerOnHandler(logger *slog.Logger, provider PowerProvider) *PowerOnHandler {
	return &PowerOnHandler{
		logger:   logger,
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *PowerOnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctrl := h.provider.IPMIController()
	if ctrl == nil {
		h.logger.Error("IPMI controller not available")
		http.Error(w, "IPMI controller not available", http.StatusInternalServerError)
		return
	}

	// Check current power status
	status, err := ctrl.Status()
	if err != nil {
		h.logger.Error("failed to get power status", "error", err)
		http.Error(w, "Failed to get power status", http.StatusInternalServerError)
		return
	}

	if status == ipmiclient.PowerStateOn {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "success",
			"message": "PBS server is already powered on",
		})
		return
	}

	// Power on the server
	if err := ctrl.PowerOn(); err != nil {
		h.logger.Error("failed to power on PBS", "error", err)
		http.Error(w, "Failed to power on PBS", http.StatusInternalServerError)
		return
	}

	h.logger.Info("PBS server powered on via web UI")
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "PBS server power on command sent",
	})
}
