package ipmi

import (
	"log/slog"
	"testing"
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
			if got.host != tt.expected.host {
				t.Errorf("host = %v, want %v", got.host, tt.expected.host)
			}
			if got.username != tt.expected.username {
				t.Errorf("username = %v, want %v", got.username, tt.expected.username)
			}
			if got.password != tt.expected.password {
				t.Errorf("password = %v, want %v", got.password, tt.expected.password)
			}
			// Can't directly compare loggers, but we can check if they're both nil or both non-nil
			if (got.logger == nil) != (tt.expected.logger == nil) {
				t.Errorf("logger = %v, want %v", got.logger, tt.expected.logger)
			}
		})
	}
}
