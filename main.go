package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/nomis52/goback/activities"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/pbsclient"
	"github.com/nomis52/goback/proxmoxclient"
)

type Args struct {
	ConfigPath string
}

const jobName = "goback"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := parseArgs()
	if args.ConfigPath == "" {
		return fmt.Errorf("config flag (-c or --config) is required")
	}

	cfg, err := config.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	loggerConfig := logging.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		Output:    cfg.Logging.Output,
		AddSource: cfg.Logging.AddSource,
	}
	logger, err := logging.New(loggerConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("goback started", "config_path", args.ConfigPath)

	// Create orchestrator with config
	o := orchestrator.NewOrchestrator(
		orchestrator.WithConfig(&cfg),
		orchestrator.WithLogger(logger),
	)

	// Inject dependencies
	if err := injectClients(o, cfg, logger); err != nil {
		return fmt.Errorf("failed to inject clients: %w", err)
	}

	// Add activities
	powerOnPBS := &activities.PowerOnPBS{}
	runProxmoxBackup := &activities.RunProxmoxBackup{}
	o.AddActivity(powerOnPBS, runProxmoxBackup)

	// Execute orchestrator
	ctx := context.Background()
	if err := o.Execute(ctx); err != nil {
		return fmt.Errorf("orchestrator execution failed: %w", err)
	}

	return nil
}

func injectClients(o *orchestrator.Orchestrator, cfg config.Config, logger *slog.Logger) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	ctrl := ipmi.NewIPMIController(
		cfg.IPMI.Host,
		ipmi.WithUsername(cfg.IPMI.Username),
		ipmi.WithPassword(cfg.IPMI.Password),
		ipmi.WithLogger(logger),
	)

	pbsClient, err := pbsclient.New(cfg.PBS.Host, logger)
	if err != nil {
		return fmt.Errorf("failed to create PBS client: %w", err)
	}

	proxmoxClient, err := proxmoxclient.New(cfg.Proxmox.Host, proxmoxclient.WithToken(cfg.Proxmox.Token))
	if err != nil {
		return fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	metricsClient := metrics.NewClient(
		cfg.Monitoring.VictoriaMetricsURL,
		metrics.WithPrefix(cfg.Monitoring.MetricsPrefix),
		metrics.WithJob(jobName),
		metrics.WithInstance(hostname),
	)

	o.Inject(logger, metricsClient, ctrl, pbsClient, proxmoxClient)
	return nil
}

func parseArgs() Args {
	configPath := flag.String("config", "", "Path to config file")
	configPathShort := flag.String("c", "", "Path to config file (shorthand)")
	flag.Parse()

	path := *configPath
	if path == "" && *configPathShort != "" {
		path = *configPathShort
	}
	return Args{ConfigPath: path}
}
