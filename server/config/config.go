package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents the server runtime configuration.
type ServerConfig struct {
	Listener ListenerConfig `yaml:"listener"`
	Cron     []CronTrigger  `yaml:"cron"`
	// The path to the directory used to store the workflow history
	StateDir string `yaml:"state_dir"`
	LogLevel string `yaml:"log_level"`
	// The path to the workflow config file
	WorkflowConfig string `yaml:"workflow_config"`
}

// ListenerConfig holds HTTP server listener settings.
type ListenerConfig struct {
	// The listen address, defaults to :8080
	Addr string `yaml:"addr"`
}

// CronTrigger defines a set of workflows to run on a schedule.
type CronTrigger struct {
	// A comma separated list of workflows to run
	Workflows []string `yaml:"workflows"`
	// The cron spec to execute the workflows at
	Schedule string `yaml:"schedule"`
}

// LoadConfig reads the YAML config file at the given path and returns a ServerConfig struct.
func LoadConfig(path string) (*ServerConfig, error) {
	var cfg ServerConfig
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open server config file %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode YAML server config: %w", err)
	}

	cfg.SetDefaults()

	// Basic validation could go here, but strictly following plan for now.
	// Only Listener Addr has a default in the plan.

	return &cfg, nil
}

// SetDefaults sets reasonable default values for optional fields.
func (c *ServerConfig) SetDefaults() {
	if c.Listener.Addr == "" {
		c.Listener.Addr = ":8080"
	}
}
