package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Default timeouts
	defaultBootTimeout         = 5 * time.Minute
	defaultPBSBootTimeout      = 10 * time.Minute
	defaultBackupJobTimeout    = 2 * time.Hour
	defaultTotalRuntimeTimeout = 8 * time.Hour
	defaultShutdownTimeout     = 2 * time.Minute

	// Default backup settings
	defaultMaxAge     = 24 * time.Hour // 24 hours default
	defaultBackupMode = "snapshot"     // default backup mode
	defaultCompress   = "1"            // default compression enabled (1 = gzip)

	// Default monitoring settings
	defaultMetricsPrefix = "pbs_automation"
	defaultJobName       = "goback"

	// Default behavior settings
	defaultMaxRetries = 2
	defaultRetryDelay = 30 * time.Second

	// Default logging settings
	defaultLogLevel  = "info"
	defaultLogFormat = "json"
	defaultLogOutput = "stdout"
)

// Config represents the complete application configuration
type Config struct {
	IPMI        IPMIConfig        `yaml:"ipmi"`
	PBS         PBSConfig         `yaml:"pbs"`
	Proxmox     ProxmoxConfig     `yaml:"proxmox"`
	Timeouts    TimeoutsConfig    `yaml:"timeouts"`
	Backup      BackupConfig      `yaml:"backup"`
	Directories []DirectoryConfig `yaml:"directories"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
	Behavior    BehaviorConfig    `yaml:"behavior"`
	Logging     LoggingConfig     `yaml:"logging"`
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

	// Datastore is the name of the backup datastore to use
	Datastore string `yaml:"datastore"`

	// BootTimeout is the maximum time to wait for the PBS server to become available after boot
	BootTimeout time.Duration `yaml:"boot_timeout"`
}

// ProxmoxConfig holds Proxmox API connection settings
type ProxmoxConfig struct {
	Host          string        `yaml:"host"`
	Token         string        `yaml:"token"`
	Storage       string        `yaml:"storage"`
	BackupTimeout time.Duration `yaml:"backup_timeout"`
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
	MaxAge   time.Duration `yaml:"max_age"`
	Mode     string        `yaml:"mode"`     // backup mode: snapshot, suspend, stop
	Compress string        `yaml:"compress"` // compression: "0", "1", "gzip", "lzo", "zstd"
}

// DirectoryConfig defines a single SSH backup job for the directories stanza
type DirectoryConfig struct {
	Host    string   `yaml:"host"`
	User    string   `yaml:"user"`
	Token   string   `yaml:"token"`
	Target  string   `yaml:"target"`
	Sources []string `yaml:"sources"`
}

// MonitoringConfig holds metrics and monitoring settings
type MonitoringConfig struct {
	VictoriaMetricsURL string `yaml:"victoriametrics_url"`
	MetricsPrefix      string `yaml:"metrics_prefix"`
	JobName            string `yaml:"jobname"`
}

// BehaviorConfig defines application behavior settings
type BehaviorConfig struct {
	ShutdownOnPartialFailure bool          `yaml:"shutdown_on_partial_failure"`
	MaxRetries               int           `yaml:"max_retries"`
	RetryDelay               time.Duration `yaml:"retry_delay"`
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
	if c.IPMI.Host == "" {
		return fmt.Errorf("IPMI host is required")
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
	if c.PBS.Datastore == "" {
		return fmt.Errorf("PBS datastore is required")
	}
	if c.PBS.BootTimeout <= 0 {
		return fmt.Errorf("PBS boot timeout must be positive")
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
	return nil
}

// SetDefaults sets reasonable default values for optional fields
func (c *Config) SetDefaults() {
	if c.Timeouts.BootTimeout == 0 {
		c.Timeouts.BootTimeout = defaultBootTimeout
	}
	if c.PBS.BootTimeout == 0 {
		c.PBS.BootTimeout = defaultPBSBootTimeout
	}
	if c.Timeouts.BackupJobTimeout == 0 {
		c.Timeouts.BackupJobTimeout = defaultBackupJobTimeout
	}
	if c.Timeouts.TotalRuntimeTimeout == 0 {
		c.Timeouts.TotalRuntimeTimeout = defaultTotalRuntimeTimeout
	}
	if c.Timeouts.ShutdownTimeout == 0 {
		c.Timeouts.ShutdownTimeout = defaultShutdownTimeout
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
	if c.Behavior.MaxRetries == 0 {
		c.Behavior.MaxRetries = defaultMaxRetries
	}
	if c.Behavior.RetryDelay == 0 {
		c.Behavior.RetryDelay = defaultRetryDelay
	}
	if c.Backup.MaxAge == 0 {
		c.Backup.MaxAge = defaultMaxAge
	}
	if c.Backup.Mode == "" {
		c.Backup.Mode = defaultBackupMode
	}
	if c.Backup.Compress == "" {
		c.Backup.Compress = defaultCompress
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
