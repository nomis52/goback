package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nomis52/goback/server"
	serverconfig "github.com/nomis52/goback/server/config"
)

type Args struct {
	ConfigPath string
}

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

	// Load server configuration
	srvCfg, err := serverconfig.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load server config: %w", err)
	}

	var opts []server.Option
	if srvCfg.Listener.Addr != "" {
		opts = append(opts, server.WithListenAddr(srvCfg.Listener.Addr))
	}
	if len(srvCfg.Cron) > 0 {
		opts = append(opts, server.WithCron(srvCfg.Cron))
	}
	if srvCfg.StateDir != "" {
		opts = append(opts, server.WithStateDir(srvCfg.StateDir))
	}

	srv, err := server.New(
		srvCfg.WorkflowConfig,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Configure log level if specified
	if srvCfg.LogLevel != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(srvCfg.LogLevel)); err != nil {
			return fmt.Errorf("invalid log level '%s': %w", srvCfg.LogLevel, err)
		}
		srv.SetLogLevel(level)
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		srv.Logger().Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	return srv.Run(ctx)
}

func parseArgs() Args {
	configPath := flag.String("config", "", "Path to server config file")
	configPathShort := flag.String("c", "", "Path to server config file (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGoback Server - PBS Backup Automation Web Interface\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --config /etc/goback/server_config.yaml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -c server_config.yaml\n", os.Args[0])
	}

	flag.Parse()

	path := *configPath
	if path == "" && *configPathShort != "" {
		path = *configPathShort
	}

	return Args{
		ConfigPath: path,
	}
}
