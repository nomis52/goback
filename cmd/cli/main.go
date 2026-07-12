package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nomis52/goback/buildinfo"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
	"github.com/nomis52/goback/workflows/backup"
	"github.com/nomis52/goback/workflows/poweroff"
)

type Args struct {
	ConfigPath  string
	ShowVersion bool
	Validate    bool
}

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

	props := buildinfo.Get()
	logger.Info("goback started",
		"build_time", props.BuildTime,
		"git_commit", props.GitCommit,
		"config_path", args.ConfigPath,
	)

	// Get hostname for metrics
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	// Create push-based metrics registry for CLI mode
	registry := metrics.NewPushRegistry(metrics.PushConfig{
		URL:      cfg.Monitoring.VictoriaMetricsURL,
		Prefix:   cfg.Monitoring.MetricsPrefix,
		Job:      cfg.Monitoring.JobName,
		Instance: hostname,
	})

	// Run a full backup, then power PBS off. combined-backup powers PBS on itself (as
	// a dependency) and backs up VMs and dirs concurrently; power-off is composed
	// after it so it runs even if the backup fails, the same way the cron config does.
	params := workflows.Params{
		Config:           &cfg,
		Logger:           logger,
		StatusCollection: nil,
		LoggerFactory:    nil,
		Registry:         registry,
	}

	backupWorkflow, err := backup.NewCombinedWorkflow(params)
	if err != nil {
		return fmt.Errorf("failed to create backup workflow: %w", err)
	}

	powerOffWorkflow, err := poweroff.NewWorkflow(params)
	if err != nil {
		return fmt.Errorf("failed to create power off workflow: %w", err)
	}

	// Compose so power-off runs after the backup, even if the backup fails.
	composedWorkflow := workflow.Compose(backupWorkflow, powerOffWorkflow)

	// Execute the workflow
	ctx := context.Background()
	if err := composedWorkflow.Execute(ctx); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}

func showVersion() {
	props := buildinfo.Get()
	fmt.Printf("goback\n")
	fmt.Printf("Built: %s\n", props.BuildTime)
	fmt.Printf("Commit: %s\n", props.GitCommit)
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
