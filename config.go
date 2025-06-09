package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	IPMI       IPMIConfig       `yaml:"ipmi"`
	Proxmox    ProxmoxConfig    `yaml:"proxmox"`
	PBS        PBSConfig        `yaml:"pbs"`
	Timeouts   TimeoutsConfig   `yaml:"timeouts"`
	Backup     BackupConfig     `yaml:"backup"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Behavior   BehaviorConfig   `yaml:"behavior"`
}

// IPMIConfig holds IPMI connection settings
type IPMIConfig struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ProxmoxConfig holds Proxmox API connection settings
type ProxmoxConfig struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// PBSConfig holds Proxmox Backup Server settings
type PBSConfig struct {
	Host      string `yaml:"host"`
	Datastore string `yaml:"datastore"`
}

// TimeoutsConfig defines various timeout durations
type TimeoutsConfig struct {
	BootTimeout         time.Duration `yaml:"boot_timeout"`
	BackupJobTimeout    time.Duration `yaml:"backup_job_timeout"`
	TotalRuntimeTimeout time.Duration `yaml:"total_runtime_timeout"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
}

// BackupConfig defines backup behavior settings
type BackupConfig struct {
	MaxConcurrent      int      `yaml:"max_concurrent"`
	ExcludeTemplates   bool     `yaml:"exclude_templates"`
	ExcludeStopped     bool     `yaml:"exclude_stopped"`
	IncludeZFSDatasets bool     `yaml:"include_zfs_datasets"`
	ZFSDatasets        []string `yaml:"zfs_datasets"`
}

// MonitoringConfig holds metrics and monitoring settings
type MonitoringConfig struct {
	VictoriaMetricsURL string `yaml:"victoriametrics_url"`
	MetricsPrefix      string `yaml:"metrics_prefix"`
}

// BehaviorConfig defines application behavior settings
type BehaviorConfig struct {
	ShutdownOnPartialFailure bool          `yaml:"shutdown_on_partial_failure"`
	MaxRetries               int           `yaml:"max_retries"`
	RetryDelay               time.Duration `yaml:"retry_delay"`
}

// Validate performs basic validation on the configuration
func (c *Config) Validate() error {
	if c.IPMI.Host == "" {
		return fmt.Errorf("IPMI host is required")
	}
	if c.Proxmox.Host == "" {
		return fmt.Errorf("Proxmox host is required")
	}
	if c.PBS.Host == "" {
		return fmt.Errorf("PBS host is required")
	}
	if c.PBS.Datastore == "" {
		return fmt.Errorf("PBS datastore is required")
	}
	if c.Monitoring.VictoriaMetricsURL == "" {
		return fmt.Errorf("VictoriaMetrics URL is required")
	}
	if c.Timeouts.BootTimeout <= 0 {
		return fmt.Errorf("boot timeout must be positive")
	}
	if c.Timeouts.BackupJobTimeout <= 0 {
		return fmt.Errorf("backup job timeout must be positive")
	}
	if c.Backup.MaxConcurrent < 1 {
		return fmt.Errorf("max concurrent jobs must be at least 1")
	}
	return nil
}

// SetDefaults sets reasonable default values for optional fields
func (c *Config) SetDefaults() {
	if c.Timeouts.BootTimeout == 0 {
		c.Timeouts.BootTimeout = 5 * time.Minute
	}
	if c.Timeouts.BackupJobTimeout == 0 {
		c.Timeouts.BackupJobTimeout = 2 * time.Hour
	}
	if c.Timeouts.TotalRuntimeTimeout == 0 {
		c.Timeouts.TotalRuntimeTimeout = 8 * time.Hour
	}
	if c.Timeouts.ShutdownTimeout == 0 {
		c.Timeouts.ShutdownTimeout = 2 * time.Minute
	}
	if c.Backup.MaxConcurrent == 0 {
		c.Backup.MaxConcurrent = 3
	}
	if c.Monitoring.MetricsPrefix == "" {
		c.Monitoring.MetricsPrefix = "pbs_automation"
	}
	if c.Behavior.MaxRetries == 0 {
		c.Behavior.MaxRetries = 2
	}
	if c.Behavior.RetryDelay == 0 {
		c.Behavior.RetryDelay = 30 * time.Second
	}
	// Defaults for boolean fields are already false, which is appropriate
}

// LoadConfig reads the YAML config file at the given path and returns a Config struct
func LoadConfig(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
