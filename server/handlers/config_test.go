package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type mockConfigProvider struct {
	config *config.Config
}

func (m *mockConfigProvider) Config() *config.Config {
	return m.config
}

func TestConfigHandler(t *testing.T) {
	cfg := &config.Config{
		PBS: config.PBSConfig{
			Host: "pbs.example.com",
			IPMI: config.IPMIConfig{
				Host:     "pbs-ipmi.example.com",
				Username: "admin",
				Password: "secret",
			},
			BootTimeout:     10 * time.Minute,
			ServiceWaitTime: 30 * time.Second,
			ShutdownTimeout: 2 * time.Minute,
		},
		Proxmox: config.ProxmoxConfig{
			Host:          "pve.example.com",
			Token:         "secret-token",
			Storage:       "local-zfs",
			BackupTimeout: 2 * time.Hour,
		},
	}

	provider := &mockConfigProvider{config: cfg}
	handler := NewConfigHandler(provider)

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/yaml", w.Header().Get("Content-Type"))

	var resp config.Config
	err := yaml.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "pbs.example.com", resp.PBS.Host)
	assert.Equal(t, "pve.example.com", resp.Proxmox.Host)
	assert.Equal(t, "local-zfs", resp.Proxmox.Storage)
}
