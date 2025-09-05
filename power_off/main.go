package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nomis52/goback/activities"
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/logging"
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

	// Create IPMI controller
	controller := ipmi.NewIPMIController(
		cfg.IPMI.Host,
		ipmi.WithUsername(cfg.IPMI.Username),
		ipmi.WithPassword(cfg.IPMI.Password),
		ipmi.WithLogger(logger),
	)

	// Create and configure PowerOffPBS activity
	powerOffActivity := &activities.PowerOffPBS{
		Controller:      controller,
		Logger:          logger,
		ShutdownTimeout: cfg.Timeouts.ShutdownTimeout,
		// Note: We don't set BackupDirs and BackupVMs dependencies since we're testing standalone
	}

	// Initialize the activity
	if err := powerOffActivity.Init(); err != nil {
		return fmt.Errorf("failed to initialize PowerOffPBS activity: %w", err)
	}

	logger.Info("starting PBS shutdown test")

	// Execute the power off activity
	ctx := context.Background()
	if err := powerOffActivity.Execute(ctx); err != nil {
		logger.Error("PowerOffPBS execution failed", "error", err)
		return fmt.Errorf("PowerOffPBS execution failed: %w", err)
	}

	logger.Info("PBS shutdown test completed successfully")
	return nil
}
