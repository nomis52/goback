package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockReloader struct {
	err error
}

func (m *mockReloader) Reload() error {
	return m.err
}

func TestReloadHandler_Success(t *testing.T) {
	reloader := &mockReloader{err: nil}
	handler := NewReloadHandler(slog.Default(), reloader)

	req := httptest.NewRequest(http.MethodPost, "/reload", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestReloadHandler_Error(t *testing.T) {
	reloader := &mockReloader{err: errors.New("config file not found")}
	handler := NewReloadHandler(slog.Default(), reloader)

	req := httptest.NewRequest(http.MethodPost, "/reload", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "config file not found")
}
