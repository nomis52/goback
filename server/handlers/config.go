package handlers

import (
	"log/slog"
	"net/http"

	"gopkg.in/yaml.v3"
)

// ConfigHandler handles requests for the current configuration.
type ConfigHandler struct {
	configProvider ConfigProvider
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(provider ConfigProvider) *ConfigHandler {
	return &ConfigHandler{
		configProvider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *ConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := h.configProvider.Config()

	// Redact sensitive fields before returning
	redacted := cfg.Redacted()

	w.Header().Set("Content-Type", "text/yaml")
	w.WriteHeader(http.StatusOK)
	if err := yaml.NewEncoder(w).Encode(redacted); err != nil {
		slog.Error("failed to encode YAML response", "error", err)
	}
}
