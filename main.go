package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/ipmi"
)

type Args struct {
	ConfigPath string
}

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

	ctrl := ipmi.NewIPMIController(cfg.IPMI.Host, cfg.IPMI.Username, cfg.IPMI.Password)
	s, err := ctrl.Status()

	if err != nil {
		return err
	}
	fmt.Println(s)
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
