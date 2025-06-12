package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				IPMI:    IPMIConfig{Host: "1.2.3.4", Username: "u", Password: "p"},
				Proxmox: ProxmoxConfig{Host: "1.2.3.5", Username: "u", Password: "p"},
				PBS:     PBSConfig{Host: "1.2.3.6", Datastore: "ds", BootTimeout: 10 * time.Minute},
				Timeouts: TimeoutsConfig{
					BootTimeout:         time.Minute,
					BackupJobTimeout:    time.Minute,
					TotalRuntimeTimeout: time.Minute,
					ShutdownTimeout:     time.Minute,
				},
				Backup:     BackupConfig{MaxConcurrent: 1},
				Monitoring: MonitoringConfig{VictoriaMetricsURL: "http://vm"},
			},
			wantErr: false,
		},
		{
			name:    "missing IPMI host",
			cfg:     Config{Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing Proxmox host",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS host",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS datastore",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing VictoriaMetrics URL",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}},
			wantErr: true,
		},
		{
			name:    "non-positive boot timeout",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: 0, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "non-positive backup job timeout",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: 0}, Backup: BackupConfig{MaxConcurrent: 1}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "max concurrent < 1",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxConcurrent: 0}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := Config{}
	cfg.SetDefaults()
	if cfg.Timeouts.BootTimeout != 5*time.Minute {
		t.Errorf("BootTimeout default = %v, want %v", cfg.Timeouts.BootTimeout, 5*time.Minute)
	}
	if cfg.Timeouts.BackupJobTimeout != 2*time.Hour {
		t.Errorf("BackupJobTimeout default = %v, want %v", cfg.Timeouts.BackupJobTimeout, 2*time.Hour)
	}
	if cfg.Timeouts.TotalRuntimeTimeout != 8*time.Hour {
		t.Errorf("TotalRuntimeTimeout default = %v, want %v", cfg.Timeouts.TotalRuntimeTimeout, 8*time.Hour)
	}
	if cfg.Timeouts.ShutdownTimeout != 2*time.Minute {
		t.Errorf("ShutdownTimeout default = %v, want %v", cfg.Timeouts.ShutdownTimeout, 2*time.Minute)
	}
	if cfg.Backup.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent default = %v, want %v", cfg.Backup.MaxConcurrent, 3)
	}
	if cfg.Monitoring.MetricsPrefix != "pbs_automation" {
		t.Errorf("MetricsPrefix default = %v, want %v", cfg.Monitoring.MetricsPrefix, "pbs_automation")
	}
	if cfg.Monitoring.JobName != "goback" {
		t.Errorf("JobName default = %v, want %v", cfg.Monitoring.JobName, "goback")
	}
	if cfg.Behavior.MaxRetries != 2 {
		t.Errorf("MaxRetries default = %v, want %v", cfg.Behavior.MaxRetries, 2)
	}
	if cfg.Behavior.RetryDelay != 30*time.Second {
		t.Errorf("RetryDelay default = %v, want %v", cfg.Behavior.RetryDelay, 30*time.Second)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	content := `ipmi:
  host: 1.2.3.4
  username: user
  password: pass
proxmox:
  host: 1.2.3.5
  username: user
  password: pass
pbs:
  host: 1.2.3.6
  datastore: ds
timeouts:
  boot_timeout: 60s
  backup_job_timeout: 60s
  total_runtime_timeout: 60s
  shutdown_timeout: 60s
backup:
  max_concurrent: 2
monitoring:
  victoriametrics_url: http://vm
behavior:
  max_retries: 1
  retry_delay: 10s
`
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.IPMI.Host != "1.2.3.4" {
		t.Errorf("IPMI.Host = %v, want %v", cfg.IPMI.Host, "1.2.3.4")
	}
	if cfg.Proxmox.Host != "1.2.3.5" {
		t.Errorf("Proxmox.Host = %v, want %v", cfg.Proxmox.Host, "1.2.3.5")
	}
	if cfg.PBS.Host != "1.2.3.6" {
		t.Errorf("PBS.Host = %v, want %v", cfg.PBS.Host, "1.2.3.6")
	}
	if cfg.PBS.Datastore != "ds" {
		t.Errorf("PBS.Datastore = %v, want %v", cfg.PBS.Datastore, "ds")
	}
	if cfg.Backup.MaxConcurrent != 2 {
		t.Errorf("Backup.MaxConcurrent = %v, want %v", cfg.Backup.MaxConcurrent, 2)
	}
	if cfg.Monitoring.VictoriaMetricsURL != "http://vm" {
		t.Errorf("VictoriaMetricsURL = %v, want %v", cfg.Monitoring.VictoriaMetricsURL, "http://vm")
	}
	if cfg.Behavior.MaxRetries != 1 {
		t.Errorf("Behavior.MaxRetries = %v, want %v", cfg.Behavior.MaxRetries, 1)
	}
	if cfg.Behavior.RetryDelay != 10*time.Second {
		t.Errorf("Behavior.RetryDelay = %v, want %v", cfg.Behavior.RetryDelay, 10*time.Second)
	}
}
