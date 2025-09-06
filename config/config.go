package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Default timeouts
	defaultPBSBootTimeout   = 10 * time.Minute
	defaultShutdownTimeout  = 2 * time.Minute
	defaultBackupJobTimeout = 2 * time.Hour

	// Default backup settings
	defaultMaxAge     = 24 * time.Hour // 24 hours default
	defaultBackupMode = "snapshot"     // default backup mode
	defaultCompress   = "1"            // default compression enabled (1 = gzip)

	// Default monitoring settings
	defaultMetricsPrefix = "pbs_automation"
	defaultJobName       = "goback"

	// Default logging settings
	defaultLogLevel  = "info"
	defaultLogFormat = "json"
	defaultLogOutput = "stdout"
)

// Config represents the complete application configuration
type Config struct {
	PBS        PBSConfig        `yaml:"pbs"`
	Proxmox    ProxmoxConfig    `yaml:"proxmox"`
	Compute    ComputeConfig    `yaml:"compute"`
	Files      FilesConfig      `yaml:"files"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// IPMIConfig holds IPMI connection settings
type IPMIConfig struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// PBSConfig holds Proxmox Backup Server settings
type PBSConfig struct {
	// Host is the address of the Proxmox Backup Server
	Host string `yaml:"host"`

	// IPMI holds BMC connection settings for the PBS server
	IPMI IPMIConfig `yaml:"ipmi"`

	// BootTimeout is the maximum time to wait for the PBS server to become available after boot
	BootTimeout time.Duration `yaml:"boot_timeout"`

	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// ProxmoxConfig holds Proxmox API connection settings
type ProxmoxConfig struct {
	Host          string        `yaml:"host"`
	Token         string        `yaml:"token"`
	Storage       string        `yaml:"storage"`
	BackupTimeout time.Duration `yaml:"backup_timeout"`
}



// ComputeConfig defines backup behavior settings for VMs and LXCs
type ComputeConfig struct {
	MaxBackupAge time.Duration `yaml:"max_backup_age"`
	Mode         string        `yaml:"mode"`           // backup mode: snapshot, suspend, stop
	Compress     string        `yaml:"compress"`       // compression: "0", "1", "gzip", "lzo", "zstd"
}

// FilesConfig defines a single SSH backup job for file-based backups
type FilesConfig struct {
	Host           string   `yaml:"host"`
	User           string   `yaml:"user"`
	PrivateKeyPath string   `yaml:"private_key_path"`
	Token          string   `yaml:"token"`
	Target         string   `yaml:"target"`
	Sources        []string `yaml:"sources"`
}

// MonitoringConfig holds metrics and monitoring settings
type MonitoringConfig struct {
	VictoriaMetricsURL string `yaml:"victoriametrics_url"`
	MetricsPrefix      string `yaml:"metrics_prefix"`
	JobName            string `yaml:"jobname"`
}



// LoggingConfig defines logging behavior settings
type LoggingConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	Output    string `yaml:"output"`
	AddSource bool   `yaml:"add_source"`
}

// Validate performs basic validation on the configuration
func (c *Config) Validate() error {
	if c.PBS.IPMI.Host == "" {
		return fmt.Errorf("PBS IPMI host is required")
	}
	if c.PBS.IPMI.Username == "" {
		return fmt.Errorf("PBS IPMI username is required")
	}
	if c.PBS.IPMI.Password == "" {
		return fmt.Errorf("PBS IPMI password is required")
	}
	if c.Proxmox.Host == "" {
		return fmt.Errorf("Proxmox host is required")
	}
	if c.Proxmox.Storage == "" {
		return fmt.Errorf("Proxmox storage is required")
	}
	if c.PBS.Host == "" {
		return fmt.Errorf("PBS host is required")
	}
	if c.PBS.BootTimeout <= 0 {
		return fmt.Errorf("PBS boot timeout must be positive")
	}
	if c.Monitoring.VictoriaMetricsURL == "" {
		return fmt.Errorf("VictoriaMetrics URL is required")
	}
	if c.PBS.ShutdownTimeout <= 0 {
		return fmt.Errorf("PBS shutdown timeout must be positive")
	}
	if c.Proxmox.BackupTimeout <= 0 {
		return fmt.Errorf("proxmox backup timeout must be positive")
	}
	return nil
}

// SetDefaults sets reasonable default values for optional fields
func (c *Config) SetDefaults() {
	if c.PBS.BootTimeout == 0 {
		c.PBS.BootTimeout = defaultPBSBootTimeout
	}
	if c.PBS.ShutdownTimeout == 0 {
		c.PBS.ShutdownTimeout = defaultShutdownTimeout
	}
	if c.Proxmox.BackupTimeout == 0 {
		c.Proxmox.BackupTimeout = defaultBackupJobTimeout
	}
	if c.Monitoring.MetricsPrefix == "" {
		c.Monitoring.MetricsPrefix = defaultMetricsPrefix
	}
	if c.Monitoring.JobName == "" {
		c.Monitoring.JobName = defaultJobName
	}
	if c.Compute.MaxBackupAge == 0 {
		c.Compute.MaxBackupAge = defaultMaxAge
	}
	if c.Compute.Mode == "" {
		c.Compute.Mode = defaultBackupMode
	}
	if c.Compute.Compress == "" {
		c.Compute.Compress = defaultCompress
	}
	// Set logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = defaultLogLevel
	}
	if c.Logging.Format == "" {
		c.Logging.Format = defaultLogFormat
	}
	if c.Logging.Output == "" {
		c.Logging.Output = defaultLogOutput
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
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
