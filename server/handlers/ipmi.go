package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nomis52/goback/ipmi"
)

// IPMIResponse is returned by the /ipmi endpoint.
type IPMIResponse struct {
	PBS       PBSStatus `json:"pbs"`
	Timestamp time.Time `json:"timestamp"`
}

// PBSStatus contains the current power state of the PBS server.
type PBSStatus struct {
	PowerState string `json:"power_state"`
}

// ErrorResponse is returned when an error occurs.
type ErrorResponse struct {
	Error string `json:"error"`
}

// IPMIProvider provides access to an IPMI controller.
type IPMIProvider interface {
	IPMIController() *ipmi.IPMIController
}

// IPMIHandler handles requests for PBS power state via IPMI.
type IPMIHandler struct {
	logger       *slog.Logger
	ipmiProvider IPMIProvider
}

// NewIPMIHandler creates a new IPMIHandler.
func NewIPMIHandler(logger *slog.Logger, provider IPMIProvider) *IPMIHandler {
	return &IPMIHandler{
		logger:       logger,
		ipmiProvider: provider,
	}
}

// ServeHTTP implements http.Handler.
func (h *IPMIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctrl := h.ipmiProvider.IPMIController()

	var powerStateStr string
	if ctrl != nil {
		state, err := ctrl.Status()
		if err != nil {
			h.logger.Error("failed to get IPMI status", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error: "failed to get PBS power state",
			})
			return
		}
		powerStateStr = state.String()
	} else {
		powerStateStr = "unknown"
	}

	resp := IPMIResponse{
		PBS: PBSStatus{
			PowerState: powerStateStr,
		},
		Timestamp: time.Now(),
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
