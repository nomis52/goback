package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nomis52/goback/server"
)

const defaultListenAddr = ":8080"

type Args struct {
	ConfigPath string
	ListenAddr string
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

	srv, err := server.New(
		args.ListenAddr,
		args.ConfigPath,
	)
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

func parseArgs() Args {
	configPath := flag.String("config", "", "Path to config file")
	configPathShort := flag.String("c", "", "Path to config file (shorthand)")
	listenAddr := flag.String("listen", defaultListenAddr, "Address to listen on")
	listenAddrShort := flag.String("l", defaultListenAddr, "Address to listen on (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGoback Server - PBS Backup Automation Web Interface\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "  %s --config /etc/goback/config.yaml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -c config.yaml -l :9090\n", os.Args[0])
	}

	flag.Parse()

	path := *configPath
	if path == "" && *configPathShort != "" {
		path = *configPathShort
	}

	addr := *listenAddr
	if *listenAddrShort != defaultListenAddr {
		addr = *listenAddrShort
	}

	return Args{
		ConfigPath: path,
		ListenAddr: addr,
	}
}
