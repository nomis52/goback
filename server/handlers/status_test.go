package handlers

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nomis52/goback/ipmi"
	"github.com/stretchr/testify/assert"
)

type mockIPMIProvider struct {
	controller *ipmi.IPMIController
}

func (m *mockIPMIProvider) IPMIController() *ipmi.IPMIController {
	return m.controller
}

func TestStatusHandler_NoIPMIController(t *testing.T) {
	provider := &mockIPMIProvider{controller: nil}
	handler := NewStatusHandler(slog.Default(), provider)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"power_state":"unknown"`)
}
