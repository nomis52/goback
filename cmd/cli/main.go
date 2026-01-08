package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nomis52/goback/backup"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflow"
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

	// Create status reporter for tracking activity progress
	sr := statusreporter.New(logger)

	// Create backup workflow (PowerOnPBS → BackupDirs → BackupVMs)
	backupWorkflow, err := backup.NewBackupWorkflow(&cfg, logger, sr)
	if err != nil {
		return fmt.Errorf("failed to create backup workflow: %w", err)
	}

	// Create power off workflow (PowerOffPBS)
	powerOffWorkflow, err := backup.NewPowerOffWorkflow(&cfg, logger, sr)
	if err != nil {
		return fmt.Errorf("failed to create power off workflow: %w", err)
	}

	// Compose workflows to run backup then power off
	composedWorkflow := workflow.Compose(backupWorkflow, powerOffWorkflow)

	// Execute composed workflow
	ctx := context.Background()
	if err := composedWorkflow.Execute(ctx); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}

func showVersion() {
	fmt.Printf("goback version %s\n", Version)
	fmt.Printf("Built: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n", GitCommit)
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
