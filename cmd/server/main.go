package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nomis52/goback/server"
	serverconfig "github.com/nomis52/goback/server/config"
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
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := parseArgs()

	if args.ShowVersion {
		showVersion()
		return nil
	}

	if args.ConfigPath == "" {
		return fmt.Errorf("config flag (-c or --config) is required")
	}

	// Load server configuration
	srvCfg, err := serverconfig.LoadConfig(args.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load server config: %w", err)
	}

	srv, err := server.New(srvCfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
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

func showVersion() {
	fmt.Printf("goback-server version %s\n", Version)
	fmt.Printf("Built: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n", GitCommit)
}

func parseArgs() Args {
	configPath := flag.String("config", "", "Path to server config file")
	configPathShort := flag.String("c", "", "Path to server config file (shorthand)")
	showVersion := flag.Bool("version", false, "Show version information")
	versionShort := flag.Bool("v", false, "Show version information (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGoback Server - PBS Backup Automation Web Interface\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --config /etc/goback/server_config.yaml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --version\n", os.Args[0])
	}

	flag.Parse()

	path := *configPath
	if path == "" && *configPathShort != "" {
		path = *configPathShort
	}

	return Args{
		ConfigPath:  path,
		ShowVersion: *showVersion || *versionShort,
	}
}
