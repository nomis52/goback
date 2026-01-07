// Package server provides an HTTP server for the goback backup automation system.
//
// The server exposes a REST API to monitor and control PBS backup operations,
// including checking power state via IPMI, triggering backup runs, and viewing
// run history.
//
// # Endpoints
//
//   - GET / - Web UI dashboard
//   - GET /health - Simple health check, returns "ok"
//   - GET /api/status - Consolidated status endpoint (PBS state, run status, next run, results)
//   - GET /config - Returns current configuration as YAML
//   - POST /reload - Reloads configuration from disk
//   - POST /run - Triggers a backup run
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
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/server/cron"
	"github.com/nomis52/goback/server/handlers"
	"github.com/nomis52/goback/server/runner"
)

//go:embed static
var staticFiles embed.FS

const (
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	defaultListenAddr      = ":8080"
)

// serverDeps holds config-derived dependencies that are swapped atomically on reload.
type serverDeps struct {
	config         *config.Config
	ipmiController *ipmi.IPMIController
}

// Server is the HTTP server for the goback web interface.
type Server struct {
	addr        string
	configPath  string
	logger      *slog.Logger
	logLevel    *slog.LevelVar
	deps        atomic.Pointer[serverDeps]
	httpServer  *http.Server
	runner      *runner.Runner
	cronTrigger *cron.CronTrigger
}

// Option configures a Server.
type Option func(*Server) error

// WithCron configures the server to run backups on a cron schedule.
// The spec follows standard cron format (5 fields: minute, hour, day, month, weekday).
func WithCron(spec string) Option {
	return func(s *Server) error {
		trigger, err := cron.NewCronTrigger(spec, s.runner, s.logger)
		if err != nil {
			return fmt.Errorf("creating cron trigger: %w", err)
		}
		s.cronTrigger = trigger
		return nil
	}
}

// WithListenAddr configures the address the server listens on.
// Default is ":8080".
func WithListenAddr(addr string) Option {
	return func(s *Server) error {
		s.addr = addr
		return nil
	}
}

// New creates a new Server with the given config path and options.
// It loads the configuration and initializes all dependencies.
func New(configPath string, opts ...Option) (*Server, error) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(handler)

	s := &Server{
		addr:       defaultListenAddr,
		configPath: configPath,
		logger:     logger,
		logLevel:   logLevel,
	}

	if err := s.Reload(); err != nil {
		return nil, err
	}

	s.runner = runner.New(logger, s)

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

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

// NextRun returns the next scheduled run time, or nil if no cron is configured.
func (s *Server) NextRun() *time.Time {
	if s.cronTrigger == nil {
		return nil
	}
	next := s.cronTrigger.NextRun()
	return &next
}

// Status returns the current run status by delegating to the runner.
func (s *Server) Status() runner.RunStatus {
	return s.runner.Status()
}

// GetResults returns the activity results by delegating to the runner.
func (s *Server) GetResults() map[orchestrator.ActivityID]*orchestrator.Result {
	return s.runner.GetResults()
}

// Run starts the HTTP server and blocks until the context is cancelled.
// It performs a graceful shutdown when the context is done.
// If a cron trigger is configured, it will be started automatically.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	// Start cron trigger if configured
	if s.cronTrigger != nil {
		s.logger.Info("starting cron trigger",
			"next_run", s.cronTrigger.NextRun(),
		)
		s.cronTrigger.Start(ctx)
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
	configHandler := handlers.NewConfigHandler(s)
	reloadHandler := handlers.NewReloadHandler(s.logger, s)
	runHandler := handlers.NewRunHandler(s.runner)
	historyHandler := handlers.NewHistoryHandler(s.runner)
	apiStatusHandler := handlers.NewAPIStatusHandler(s.logger, s)

	// API endpoints
	mux.HandleFunc("GET /health", handlers.HandleHealth)
	mux.Handle("GET /api/status", apiStatusHandler)
	mux.Handle("GET /config", configHandler)
	mux.Handle("POST /reload", reloadHandler)
	mux.Handle("POST /run", runHandler)
	mux.Handle("GET /history", historyHandler)

	// Static files (web UI)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Error("failed to create static file system", "error", err)
		return
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
}
