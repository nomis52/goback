// Package server provides an HTTP server for the goback backup automation system.
//
// The server exposes a REST API to monitor and control PBS backup operations,
// including checking power state via IPMI, triggering backup runs, and viewing
// run history.
//
// # Endpoints
//
//   - GET /health - Simple health check, returns "ok"
//   - GET /ipmi - Returns PBS power state via IPMI
//   - GET /config - Returns current configuration as YAML
//   - POST /reload - Reloads configuration from disk
//   - POST /run - Triggers a backup run
//   - GET /status - Returns status of current/last run
//   - GET /history - Returns history of completed runs
//
// # Architecture
//
// The server maintains two sets of dependencies:
//
// Server-level deps are swapped atomically on reload and include the config
// and IPMI controller used by the /ipmi endpoint.
//
// Run-level deps are created fresh for each backup run from the current config,
// ensuring configuration changes take effect on the next run without interrupting
// any in-progress backup.
//
// # Example
//
//	srv, err := server.New(":8080", "/etc/goback/config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := srv.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/server/handlers"
	"github.com/nomis52/goback/server/runner"
)

const (
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultShutdownTimeout = 5 * time.Second
)

// serverDeps holds config-derived dependencies that are swapped atomically on reload.
type serverDeps struct {
	config         *config.Config
	ipmiController *ipmi.IPMIController
}

// Server is the HTTP server for the goback web interface.
type Server struct {
	addr       string
	configPath string
	logger     *slog.Logger
	logLevel   *slog.LevelVar
	deps       atomic.Pointer[serverDeps]
	httpServer *http.Server
	runner     *runner.Runner
}

// New creates a new Server with the given address and config path.
// It loads the configuration and initializes all dependencies.
func New(addr, configPath string) (*Server, error) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(handler)

	s := &Server{
		addr:       addr,
		configPath: configPath,
		logger:     logger,
		logLevel:   logLevel,
	}

	if err := s.Reload(); err != nil {
		return nil, err
	}

	s.runner = runner.New(logger, s)
	return s, nil
}

// Logger returns the server's logger.
func (s *Server) Logger() *slog.Logger {
	return s.logger
}

// SetLogLevel changes the server's log level at runtime.
func (s *Server) SetLogLevel(level slog.Level) {
	s.logLevel.Set(level)
}

// Reload reads the config from disk and rebuilds server dependencies.
func (s *Server) Reload() error {
	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return err
	}

	ctrl := ipmi.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmi.WithUsername(cfg.PBS.IPMI.Username),
		ipmi.WithPassword(cfg.PBS.IPMI.Password),
		ipmi.WithLogger(s.logger),
	)

	s.deps.Store(&serverDeps{
		config:         &cfg,
		ipmiController: ctrl,
	})

	s.logger.Info("configuration loaded", "config_path", s.configPath)

	return nil
}

// Config returns the current configuration.
func (s *Server) Config() *config.Config {
	return s.deps.Load().config
}

// IPMIController returns the current IPMI controller.
func (s *Server) IPMIController() *ipmi.IPMIController {
	return s.deps.Load().ipmiController
}

// Run starts the HTTP server and blocks until the context is cancelled.
// It performs a graceful shutdown when the context is done.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting server",
			"addr", s.addr,
			"config_path", s.configPath,
		)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	ipmiHandler := handlers.NewIPMIHandler(s.logger, s)
	configHandler := handlers.NewConfigHandler(s)
	reloadHandler := handlers.NewReloadHandler(s.logger, s)
	runHandler := handlers.NewRunHandler(s.runner)
	statusHandler := handlers.NewRunStatusHandler(s.runner)
	historyHandler := handlers.NewHistoryHandler(s.runner)

	mux.HandleFunc("GET /health", handlers.HandleHealth)
	mux.Handle("GET /ipmi", ipmiHandler)
	mux.Handle("GET /config", configHandler)
	mux.Handle("POST /reload", reloadHandler)
	mux.Handle("POST /run", runHandler)
	mux.Handle("GET /status", statusHandler)
	mux.Handle("GET /history", historyHandler)
}
