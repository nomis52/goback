package config

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Default timeouts
	defaultPBSBootTimeout   = 10 * time.Minute
	defaultServiceWaitTime  = 30 * time.Second
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
	Password string `yaml:"password" sensitive:"true"`
}

// PBSConfig holds Proxmox Backup Server settings
type PBSConfig struct {
	// Host is the address of the Proxmox Backup Server
	Host string `yaml:"host"`

	// IPMI holds BMC connection settings for the PBS server
	IPMI IPMIConfig `yaml:"ipmi"`

	// BootTimeout is the maximum time to wait for the PBS server to become available after boot
	BootTimeout time.Duration `yaml:"boot_timeout"`

	// ServiceWaitTime is the additional time to wait for PBS services to stabilize after ping succeeds
	ServiceWaitTime time.Duration `yaml:"service_wait_time"`

	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// ProxmoxConfig holds Proxmox API connection settings
type ProxmoxConfig struct {
	Host          string        `yaml:"host"`
	Token         string        `yaml:"token" sensitive:"true"`
	Storage       string        `yaml:"storage"`
	BackupTimeout time.Duration `yaml:"backup_timeout"`
}

// ComputeConfig defines backup behavior settings for VMs and LXCs
type ComputeConfig struct {
	MaxBackupAge time.Duration `yaml:"max_backup_age"`
	Mode         string        `yaml:"mode"`     // backup mode: snapshot, suspend, stop
	Compress     string        `yaml:"compress"` // compression: "0", "1", "gzip", "lzo", "zstd"
}

// FilesConfig defines a single SSH backup job for file-based backups
type FilesConfig struct {
	Host           string   `yaml:"host"`
	User           string   `yaml:"user"`
	PrivateKeyPath string   `yaml:"private_key_path"`
	Token          string   `yaml:"token" sensitive:"true"`
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
	// PBS validation
	if c.PBS.IPMI.Host == "" {
		return fmt.Errorf("PBS IPMI host is required")
	}
	if c.PBS.IPMI.Username == "" {
		return fmt.Errorf("PBS IPMI username is required")
	}
	if c.PBS.IPMI.Password == "" {
		return fmt.Errorf("PBS IPMI password is required")
	}
	if c.PBS.Host == "" {
		return fmt.Errorf("PBS host is required")
	}
	if c.PBS.BootTimeout <= 0 {
		return fmt.Errorf("PBS boot timeout must be positive")
	}
	if c.PBS.ShutdownTimeout <= 0 {
		return fmt.Errorf("PBS shutdown timeout must be positive")
	}

	// Proxmox validation
	if c.Proxmox.Host == "" {
		return fmt.Errorf("Proxmox host is required")
	}
	if c.Proxmox.Storage == "" {
		return fmt.Errorf("Proxmox storage is required")
	}
	if c.Proxmox.BackupTimeout <= 0 {
		return fmt.Errorf("proxmox backup timeout must be positive")
	}

	// Monitoring validation
	if c.Monitoring.VictoriaMetricsURL == "" {
		return fmt.Errorf("VictoriaMetrics URL is required")
	}

	// Files validation (if files backup is configured)
	if c.Files.Host != "" {
		if c.Files.User == "" {
			return fmt.Errorf("files user is required when host is set")
		}
		if c.Files.PrivateKeyPath == "" {
			return fmt.Errorf("files private_key_path is required when host is set")
		}
		if c.Files.Token == "" {
			return fmt.Errorf("files token is required when host is set")
		}
		if c.Files.Target == "" {
			return fmt.Errorf("files target is required when host is set")
		}
		if len(c.Files.Sources) == 0 {
			return fmt.Errorf("files sources cannot be empty when host is set")
		}

		// Validate SSH private key file exists and is readable
		if _, err := os.Stat(c.Files.PrivateKeyPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("files private key file not found: %s", c.Files.PrivateKeyPath)
			}
			return fmt.Errorf("files private key file not accessible: %s (%w)", c.Files.PrivateKeyPath, err)
		}
	}

	// Compute validation
	if c.Compute.MaxBackupAge < 0 {
		return fmt.Errorf("compute max_backup_age cannot be negative")
	}

	// Validate compute mode if specified
	if c.Compute.Mode != "" {
		validModes := []string{"snapshot", "suspend", "stop"}
		found := false
		for _, mode := range validModes {
			if c.Compute.Mode == mode {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("compute mode must be one of: %v", validModes)
		}
	}

	// Validate compute compression if specified
	if c.Compute.Compress != "" {
		validCompress := []string{"0", "1", "gzip", "lzo", "zstd"}
		found := false
		for _, comp := range validCompress {
			if c.Compute.Compress == comp {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("compute compress must be one of: %v", validCompress)
		}
	}

	return nil
}

// SetDefaults sets reasonable default values for optional fields
func (c *Config) SetDefaults() {
	if c.PBS.BootTimeout == 0 {
		c.PBS.BootTimeout = defaultPBSBootTimeout
	}
	if c.PBS.ServiceWaitTime == 0 {
		c.PBS.ServiceWaitTime = defaultServiceWaitTime
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

	// Set files defaults (if files backup is configured but user not specified)
	if c.Files.Host != "" && c.Files.User == "" {
		c.Files.User = "root" // Default SSH user
	}

	// Defaults for boolean fields are already false, which is appropriate
}

// LoadConfig reads the YAML config file at the given path and returns a Config struct
func LoadConfig(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to decode YAML config: %w", err)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("config validation failed: %w", err)
	}
	return cfg, nil
}

// Redacted returns a copy of the config with sensitive fields masked.
func (c *Config) Redacted() Config {
	redacted := *c
	redactSensitiveFields(reflect.ValueOf(&redacted).Elem())
	return redacted
}

// redactSensitiveFields recursively walks a struct and redacts fields tagged with sensitive:"true"
func redactSensitiveFields(v reflect.Value) {
	if !v.IsValid() || !v.CanSet() {
		return
	}

	t := v.Type()

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := t.Field(i)

			// Check if field is marked as sensitive
			if fieldType.Tag.Get("sensitive") == "true" {
				// Redact the field based on its type
				if field.Kind() == reflect.String && field.CanSet() {
					if field.String() != "" {
						field.SetString("***REDACTED***")
					}
				}
			} else {
				// Recursively process nested structs
				redactSensitiveFields(field)
			}
		}
	}
}
