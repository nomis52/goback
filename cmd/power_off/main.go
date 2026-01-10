package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/statusreporter"
	"github.com/nomis52/goback/workflows/poweroff"
)

type Args struct {
	ConfigPath string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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

func run() error {
	args := parseArgs()
	if args.ConfigPath == "" {
		return fmt.Errorf("config flag (-c or --config) is required")
	}

	cfg, err := config.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
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

	logger.Info("power_off utility started", "config_path", args.ConfigPath)

	// Create status reporter for activity status tracking
	statusReporter := statusreporter.New(logger)

	// Create power-off workflow
	workflow, err := poweroff.NewWorkflow(&cfg, logger, statusReporter)
	if err != nil {
		return fmt.Errorf("failed to create power-off workflow: %w", err)
	}

	logger.Info("starting PBS shutdown")

	// Execute the workflow
	ctx := context.Background()
	if err := workflow.Execute(ctx); err != nil {
		logger.Error("power-off workflow failed", "error", err)
		return fmt.Errorf("power-off workflow failed: %w", err)
	}

	logger.Info("PBS shutdown completed successfully")
	return nil
}
