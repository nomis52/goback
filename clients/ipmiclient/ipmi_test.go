package ipmiclient

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIPMIController(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		opts     []Option
		expected *IPMIController
	}{
		{
			name: "default options",
			host: "test-host",
			expected: &IPMIController{
				host:   "test-host",
				logger: slog.Default(),
			},
		},
		{
			name: "with username and password",
			host: "test-host",
			opts: []Option{
				WithUsername("test-user"),
				WithPassword("test-pass"),
			},
			expected: &IPMIController{
				host:     "test-host",
				username: "test-user",
				password: "test-pass",
				logger:   slog.Default(),
			},
		},
		{
			name: "with custom logger",
			host: "test-host",
			opts: []Option{
				WithLogger(slog.New(slog.NewTextHandler(nil, nil))),
			},
			expected: &IPMIController{
				host:   "test-host",
				logger: slog.New(slog.NewTextHandler(nil, nil)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewIPMIController(tt.host, tt.opts...)
			assert.Equal(t, tt.expected.host, got.host)
			assert.Equal(t, tt.expected.username, got.username)
			assert.Equal(t, tt.expected.password, got.password)
			if (got.logger == nil) != (tt.expected.logger == nil) {
				assert.Fail(t, "logger = %v, want %v", got.logger, tt.expected.logger)
			}
		})
	}
}
