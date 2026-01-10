package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflows/poweroff"
)

// PowerProvider provides access to the IPMI controller and config for power operations.
type PowerProvider interface {
	IPMIController() *ipmiclient.IPMIController
	Config() *config.Config
	Logger() *slog.Logger
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

// PowerOffHandler handles requests to power off the PBS server.
type PowerOffHandler struct {
	logger   *slog.Logger
	provider PowerProvider
}

// NewPowerOffHandler creates a new PowerOffHandler.
func NewPowerOffHandler(logger *slog.Logger, provider PowerProvider) *PowerOffHandler {
	return &PowerOffHandler{
		logger:   logger,
		provider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *PowerOffHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctrl := h.provider.IPMIController()
	if ctrl == nil {
		h.logger.Error("IPMI controller not available")
		http.Error(w, "IPMI controller not available", http.StatusInternalServerError)
		return
	}

	cfg := h.provider.Config()
	if cfg == nil {
		h.logger.Error("config not available")
		http.Error(w, "Config not available", http.StatusInternalServerError)
		return
	}

	// Check current power status
	status, err := ctrl.Status()
	if err != nil {
		h.logger.Error("failed to get power status", "error", err)
		http.Error(w, "Failed to get power status", http.StatusInternalServerError)
		return
	}

	if status == ipmiclient.PowerStateOff {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "success",
			"message": "PBS server is already powered off",
		})
		return
	}

	// Create status reporter
	sr := statusreporter.New(h.provider.Logger())

	// Create power off workflow
	powerOffWorkflow, err := poweroff.NewWorkflow(cfg, h.provider.Logger(), sr)
	if err != nil {
		h.logger.Error("failed to create power off workflow", "error", err)
		http.Error(w, "Failed to create power off workflow", http.StatusInternalServerError)
		return
	}

	// Execute workflow in background
	go func() {
		ctx := context.Background()
		if err := powerOffWorkflow.Execute(ctx); err != nil {
			h.logger.Error("power off workflow failed", "error", err)
		} else {
			h.logger.Info("PBS server powered off successfully via web UI")
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "PBS server shutdown initiated",
	})
}
