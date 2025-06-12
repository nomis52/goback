package logging

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid json config",
			config: Config{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			wantErr: false,
		},
		{
			name: "valid text config",
			config: Config{
				Level:  "debug",
				Format: "text",
				Output: "stderr",
			},
			wantErr: false,
		},
		{
			name: "invalid level",
			config: Config{
				Level:  "invalid",
				Format: "json",
				Output: "stdout",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: Config{
				Level:  "info",
				Format: "invalid",
				Output: "stdout",
			},
			wantErr: true,
		},
		{
			name:   "defaults applied",
			config: Config{
				// No fields set, should use defaults
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Level:  "info",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "invalid level",
			config: Config{
				Level:  "trace", // not supported
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: Config{
				Level:  "info",
				Format: "xml", // not supported
			},
			wantErr: true,
		},
		{
			name:    "empty config (should pass with defaults)",
			config:  Config{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected slog.Level
		wantErr  bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"DEBUG", slog.LevelDebug, false}, // case insensitive
		{"invalid", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			level, err := parseLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && level != tt.expected {
				t.Errorf("parseLevel() = %v, want %v", level, tt.expected)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	config := Config{}
	config.setDefaults()

	if config.Level != "info" {
		t.Errorf("default level = %v, want info", config.Level)
	}
	if config.Format != "json" {
		t.Errorf("default format = %v, want json", config.Format)
	}
	if config.Output != "stdout" {
		t.Errorf("default output = %v, want stdout", config.Output)
	}
}
