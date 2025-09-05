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

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

type Args struct {
	ConfigPath  string
	ShowVersion bool
	Validate    bool
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

	// Handle version request
	if args.ShowVersion {
		showVersion()
		return nil
	}

	// Validate required config path
	if args.ConfigPath == "" {
		return fmt.Errorf("config flag (-c or --config) is required")
	}

	cfg, err := config.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Handle validation-only request
	if args.Validate {
		fmt.Printf("Configuration validation successful: %s\n", args.ConfigPath)
		return nil
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

	logger.Info("goback started",
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit,
		"config_path", args.ConfigPath,
	)

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
	backupDirs := &activities.BackupDirs{}
	backupVMs := &activities.BackupVMs{}
	powerOffPBS := &activities.PowerOffPBS{}
	o.AddActivity(powerOnPBS, backupDirs, backupVMs, powerOffPBS)

	// Execute orchestrator
	ctx := context.Background()
	if err := o.Execute(ctx); err != nil {
		return fmt.Errorf("orchestrator execution failed: %w", err)
	}

	return nil
}

func showVersion() {
	fmt.Printf("goback version %s\n", Version)
	fmt.Printf("Built: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n", GitCommit)
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
	showVersion := flag.Bool("version", false, "Show version information")
	versionShort := flag.Bool("v", false, "Show version information (shorthand)")
	validate := flag.Bool("validate", false, "Validate configuration and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nPBS Backup Automation Tool\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --config /etc/goback/config.yaml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --version\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --config config.yaml --validate\n", os.Args[0])
	}

	flag.Parse()

	path := *configPath
	if path == "" && *configPathShort != "" {
		path = *configPathShort
	}

	version := *showVersion || *versionShort

	return Args{
		ConfigPath:  path,
		ShowVersion: version,
		Validate:    *validate,
	}
}