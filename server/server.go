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
//   - GET /api/history - Returns history of completed runs
//   - GET /config - Returns current configuration as YAML
//   - POST /reload - Reloads configuration from disk
//   - POST /run - Triggers a backup run
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
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nomis52/goback/buildinfo"
	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/metrics"
	serverconfig "github.com/nomis52/goback/server/config"
	"github.com/nomis52/goback/server/cron"
	"github.com/nomis52/goback/server/handlers"
	"github.com/nomis52/goback/server/runner"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows/backup"
	"github.com/nomis52/goback/workflows/demo"
	"github.com/nomis52/goback/workflows/poweroff"
)

//go:embed static
var staticFiles embed.FS

const (
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	defaultListenAddr      = ":8080"
)

// defaultWorkflowFactories returns the standard workflow factories for backup, poweroff, and demo workflows.
func defaultWorkflowFactories() map[string]runner.WorkflowFactory {
	return map[string]runner.WorkflowFactory{
		"backup":   backup.NewWorkflow,
		"poweroff": poweroff.NewWorkflow,
		"demo":     demo.NewWorkflow,
	}
}

// serverDeps holds config-derived dependencies that are swapped atomically on reload.
type serverDeps struct {
	config         *config.Config
	ipmiController *ipmiclient.IPMIController
}

// Server is the HTTP server for the goback web interface.
type Server struct {
	addr            string
	configPath      string
	stateDir        string
	logger          *slog.Logger
	logLevel        *slog.LevelVar
	deps            atomic.Pointer[serverDeps]
	httpServer      *http.Server
	runner          *runner.Runner
	store           *runner.DiskStore
	cronTrigger     *cron.CronTriggerManager
	cronConfig      []serverconfig.CronTrigger
	tlsCert    string
	tlsKey     string
	properties ServerProperties

	// Metrics
	metricsRegistry *metrics.ScrapeRegistry

	// Static files
	staticFS fs.FS
}

// Option is a functional option for configuring the Server.
type Option func(*Server)

// WithBuildProperties sets the build properties for the server.
func WithBuildProperties(props buildinfo.Properties) Option {
	return func(s *Server) {
		s.properties.Build = props
	}
}

// New creates a new Server with the given configuration.
// It loads the configuration and initializes all dependencies.
func New(cfg *serverconfig.ServerConfig, opts ...Option) (*Server, error) {
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	// Configure log level if specified
	if cfg.LogLevel != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
			return nil, fmt.Errorf("invalid log level '%s': %w", cfg.LogLevel, err)
		}
		logLevel.Set(level)
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(handler)

	addr := defaultListenAddr
	if cfg.Listener.Addr != "" {
		addr = cfg.Listener.Addr
	}

	// Create metrics registry and server-level metrics
	metricsRegistry, err := metrics.NewScrapeRegistry()
	if err != nil {
		return nil, fmt.Errorf("creating metrics registry: %w", err)
	}
	startTime := time.Now()

	// Get hostname for server properties
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Register server uptime as a GaugeFunc for dynamic calculation on scrape
	uptimeGauge := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "server_uptime_seconds",
			Help: "Number of seconds since the server started",
		},
		func() float64 {
			return time.Since(startTime).Seconds()
		},
	)
	if err := metricsRegistry.PrometheusRegistry().Register(uptimeGauge); err != nil {
		return nil, fmt.Errorf("registering uptime gauge: %w", err)
	}

	// Initialize static file system
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("creating static file system: %w", err)
	}

	s := &Server{
		addr:       addr,
		configPath: cfg.WorkflowConfig,
		stateDir:   cfg.StateDir,
		logger:     logger,
		logLevel:   logLevel,
		cronConfig: cfg.Cron,
		tlsCert:    cfg.Listener.TLSCert,
		tlsKey:     cfg.Listener.TLSKey,
		properties: ServerProperties{
			StartedAt: startTime,
			Hostname:  hostname,
		},
		metricsRegistry: metricsRegistry,
		staticFS:        staticFS,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	if err := s.Reload(); err != nil {
		return nil, err
	}

	// Create runner with optional disk store and metrics
	runnerOpts := []runner.Option{
		runner.WithMetricsRegistry(metricsRegistry),
	}
	if s.stateDir != "" {
		store, err := runner.NewDiskStore(s.stateDir, 100, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create disk store: %w", err)
		}
		s.store = store
		runnerOpts = append(runnerOpts, runner.WithStateStore(store))
	}
	s.runner = runner.New(logger, s, defaultWorkflowFactories(), runnerOpts...)

	// Initialize cron triggers if configured
	if len(s.cronConfig) > 0 {
		if err := validateCronWorkflows(s.cronConfig, s.runner.AvailableWorkflows()); err != nil {
			return nil, fmt.Errorf("validating workflows: %w", err)
		}

		manager, err := cron.NewCronTriggerManager(s.cronConfig, s.runner, s.logger)
		if err != nil {
			return nil, fmt.Errorf("creating cron trigger manager: %w", err)
		}
		s.cronTrigger = manager
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

	ctrl := ipmiclient.NewIPMIController(
		cfg.PBS.IPMI.Host,
		ipmiclient.WithUsername(cfg.PBS.IPMI.Username),
		ipmiclient.WithPassword(cfg.PBS.IPMI.Password),
		ipmiclient.WithLogger(s.logger),
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
func (s *Server) IPMIController() *ipmiclient.IPMIController {
	return s.deps.Load().ipmiController
}

// Properties returns the server properties (build info, start time, hostname).
func (s *Server) Properties() ServerProperties {
	return s.properties
}

// MetricsRegistry returns the metrics registry.
func (s *Server) MetricsRegistry() *metrics.ScrapeRegistry {
	return s.metricsRegistry
}

// NextRun returns the next scheduled run time, or nil if no cron is configured.
func (s *Server) NextRun() *time.Time {
	if s.cronTrigger == nil {
		return nil
	}
	next := s.cronTrigger.NextRun()
	return &next
}

// NextTrigger returns information about the next scheduled trigger, or nil if no cron is configured.
func (s *Server) NextTrigger() *cron.NextTriggerInfo {
	if s.cronTrigger == nil {
		return nil
	}

	info := s.cronTrigger.NextTrigger()
	if info.Time.IsZero() {
		return nil
	}

	return &info
}

// Status returns the current run status by delegating to the runner.
func (s *Server) Status() runner.RunStatus {
	return s.runner.Status()
}

// GetResults returns the activity results by delegating to the runner.
func (s *Server) GetResults() map[workflow.ActivityID]*workflow.Result {
	return s.runner.GetResults()
}

// CurrentStatuses returns the current activity statuses by delegating to the runner.
func (s *Server) CurrentStatuses() map[workflow.ActivityID]string {
	return s.runner.CurrentStatuses()
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
		defer close(errCh)

		s.logger.Info("starting server",
			"addr", s.addr,
			"config_path", s.configPath,
			"tls_enabled", s.tlsCert != "" && s.tlsKey != "",
		)

		var err error
		if s.tlsCert != "" && s.tlsKey != "" {
			loader, loadErr := NewCertLoader(s.tlsCert, s.tlsKey, s.logger)
			if loadErr != nil {
				errCh <- fmt.Errorf("failed to initialize cert loader: %w", loadErr)
				return
			}

			s.httpServer.TLSConfig = &tls.Config{
				GetCertificate: loader.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			}
			// Use empty strings to signal to ListenAndServeTLS to use the config
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
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
	availableWorkflowsHandler := handlers.NewAvailableWorkflowsHandler(s.runner)

	// API endpoints
	mux.HandleFunc("GET /health", handlers.HandleHealth)
	mux.Handle("GET /api/status", apiStatusHandler)
	mux.Handle("GET /api/history", historyHandler)
	mux.Handle("GET /api/workflows", availableWorkflowsHandler)
	if s.store != nil {
		storeReloadHandler := handlers.NewStoreReloadHandler(s.logger, s.store)
		mux.Handle("POST /api/store_reload", storeReloadHandler)
	}
	mux.Handle("GET /config", configHandler)
	mux.Handle("POST /reload", reloadHandler)
	mux.Handle("POST /run", runHandler)

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", s.metricsRegistry.Handler())

	// Static files (web UI)
	mux.Handle("GET /", http.FileServer(http.FS(s.staticFS)))
}

func validateCronWorkflows(triggers []serverconfig.CronTrigger, availableWorkflows map[string]bool) error {
	availableList := make([]string, 0, len(availableWorkflows))
	for k := range availableWorkflows {
		availableList = append(availableList, k)
	}

	for i, cfg := range triggers {
		for _, w := range cfg.Workflows {
			if !availableWorkflows[w] {
				return fmt.Errorf("trigger %d: unknown workflow '%s' (available: %v)", i, w, availableList)
			}
		}
	}
	return nil
}
