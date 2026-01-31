package pbsclient

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name    string
		host    string
		opts    []Option
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid https host",
			host:    "https://pbs.example.com",
			opts:    []Option{WithLogger(logger)},
			wantErr: false,
		},
		{
			name:    "valid http host",
			host:    "http://192.168.1.100:8007",
			wantErr: false, // Should use default logger
		},
		{
			name:    "missing scheme",
			host:    "pbs.example.com",
			wantErr: true,
			errMsg:  "host URL must include scheme",
		},
		{
			name:    "invalid url",
			host:    "http://:invalid",
			wantErr: true,
			errMsg:  "invalid host URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.host, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.host, client.Host)
				assert.NotNil(t, client.Logger)
			}
		})
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
func (e *errorReader) Close() error {
	return nil
}

func TestPing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		modifyClient func(*Client)
		wantResp   string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/ping", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":"pong"}`))
			},
			wantResp: `{"data":"pong"}`,
			wantErr:  false,
		},
		{
			name: "non-200 status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api2/json/ping", r.URL.Path)
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr: true,
			errMsg:  "PBS server returned status 401",
		},
		{
			name: "connection error",
			handler: func(w http.ResponseWriter, r *http.Request) {},
			modifyClient: func(c *Client) {
				c.Host = "http://localhost:1" // Invalid port should trigger connection error
			},
			wantErr: true,
			errMsg:  "failed to ping PBS server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client, err := New(ts.URL, WithLogger(logger))
			require.NoError(t, err)

			if tt.modifyClient != nil {
				tt.modifyClient(client)
			}

			resp, err := client.Ping()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Empty(t, resp)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

// TestPingReadError specifically tests the body read failure
func TestPingReadError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, _ := New("http://example.com", WithLogger(logger))

	// Mocking the http.Client to return a response with a faulty body
	client.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &errorReader{},
			}, nil
		}),
	}

	resp, err := client.Ping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read response body")
	assert.Empty(t, resp)
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
