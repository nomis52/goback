package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/logging"
	"github.com/nomis52/goback/metrics"
)

type Args struct {
	ConfigPath string
}

const jobName = "goback"

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func doMain() error {
	args := parseArgs()
	if args.ConfigPath == "" {
		return fmt.Errorf("-c or --config flag is required")
	}

	cfg, err := config.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("Error loading config: %w", err)
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

	logger.Info("goback started", "config_path", args.ConfigPath)

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Error getting hostname: %w", err)
	}

	// Initialize IPMI controller
	ctrl := ipmi.NewIPMIController(
		cfg.IPMI.Host,
		ipmi.WithUsername(cfg.IPMI.Username),
		ipmi.WithPassword(cfg.IPMI.Password),
		ipmi.WithLogger(logger),
	)

	logger.Info("checking PBS power status", "host", cfg.IPMI.Host)
	s, err := ctrl.Status()
	if err != nil {
		return err
	}
	logger.Info("power status retrieved", "status", s)

	// Initialize metrics client
	metricsClient := metrics.NewClient(
		cfg.Monitoring.VictoriaMetricsURL,
		metrics.WithPrefix(cfg.Monitoring.MetricsPrefix),
		metrics.WithJob(jobName),
		metrics.WithInstance(hostname),
	)

	ctx := context.Background()
	ms := []metrics.Metric{
		{
			Name:      "wake",
			Value:     1,
			Timestamp: time.Now(),
			Labels: map[string]string{
				"job":      jobName,
				"instance": hostname,
			},
		},
	}
	logger.Info("pushing metrics", "count", len(ms), "url", cfg.Monitoring.VictoriaMetricsURL)
	err = metricsClient.PushMetrics(ctx, ms...)
	if err != nil {
		return err
	}
	logger.Info("metrics pushed successfully")

	logger.Info("goback completed successfully")
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
