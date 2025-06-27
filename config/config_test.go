package config

import (
	"fmt"
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
				Proxmox: ProxmoxConfig{Host: "1.2.3.5", Token: "t", Storage: "s"},
				PBS:     PBSConfig{Host: "1.2.3.6", Datastore: "ds", BootTimeout: 10 * time.Minute},
				Timeouts: TimeoutsConfig{
					BootTimeout:         time.Minute,
					BackupJobTimeout:    time.Minute,
					TotalRuntimeTimeout: time.Minute,
					ShutdownTimeout:     time.Minute,
				},
				Backup:     BackupConfig{MaxAge: 24 * time.Hour},
				Monitoring: MonitoringConfig{VictoriaMetricsURL: "http://vm"},
			},
			wantErr: false,
		},
		{
			name:    "missing IPMI host",
			cfg:     Config{Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing Proxmox host",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS host",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS datastore",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing VictoriaMetrics URL",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}},
			wantErr: true,
		},
		{
			name:    "non-positive boot timeout",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: 0, BackupJobTimeout: time.Second}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "non-positive backup job timeout",
			cfg:     Config{IPMI: IPMIConfig{Host: "h"}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h", Datastore: "d"}, Timeouts: TimeoutsConfig{BootTimeout: time.Second, BackupJobTimeout: 0}, Backup: BackupConfig{MaxAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
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
	if cfg.Backup.MaxAge != 24*time.Hour {
		t.Errorf("MaxAge default = %v, want %v", cfg.Backup.MaxAge, 24*time.Hour)
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
  token: token123
  storage: storage1
pbs:
  host: 1.2.3.6
  datastore: ds
timeouts:
  boot_timeout: 60s
  backup_job_timeout: 60s
  total_runtime_timeout: 60s
  shutdown_timeout: 60s
backup:
  max_age: 24h
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
	if cfg.Backup.MaxAge != 24*time.Hour {
		t.Errorf("Backup.MaxAge = %v, want %v", cfg.Backup.MaxAge, 24*time.Hour)
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

func TestLoadConfig_TimeStrings(t *testing.T) {
	tests := []struct {
		name     string
		maxAge   string
		expected time.Duration
	}{
		{"24h", "24h", 24 * time.Hour},
		{"48h", "48h", 48 * time.Hour},
		{"30m", "30m", 30 * time.Minute},
		{"1h30m", "1h30m", 90 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			content := fmt.Sprintf(`ipmi:
  host: 1.2.3.4
  username: user
  password: pass
proxmox:
  host: 1.2.3.5
  token: token123
  storage: storage1
pbs:
  host: 1.2.3.6
  datastore: ds
timeouts:
  boot_timeout: 60s
  backup_job_timeout: 60s
  total_runtime_timeout: 60s
  shutdown_timeout: 60s
backup:
  max_age: %s
monitoring:
  victoriametrics_url: http://vm
behavior:
  max_retries: 1
  retry_delay: 10s
`, tt.maxAge)

			if _, err := tmpfile.Write([]byte(content)); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}
			tmpfile.Close()

			cfg, err := LoadConfig(tmpfile.Name())
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}

			if cfg.Backup.MaxAge != tt.expected {
				t.Errorf("Backup.MaxAge = %v, want %v", cfg.Backup.MaxAge, tt.expected)
			}
		})
	}
}

func TestLoadConfig_Directories(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	content := `directories:
  - host: pve2
    token: mytoken
    target: backup-client@pbs!token-name@10.6.0.10:tank
    sources:
      - home.pxar:/p1/home
      - root.pxar:/p1/root
`
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if len(cfg.Directories) != 1 {
		t.Fatalf("expected 1 directory, got %d", len(cfg.Directories))
	}
	b := cfg.Directories[0]
	if b.Host != "pve2" {
		t.Errorf("Host = %v, want %v", b.Host, "pve2")
	}
	if b.Token != "mytoken" {
		t.Errorf("Token = %v, want %v", b.Token, "mytoken")
	}
	if b.Target != "backup-client@pbs!token-name@10.6.0.10:tank" {
		t.Errorf("Target = %v, want %v", b.Target, "backup-client@pbs!token-name@10.6.0.10:tank")
	}
	if len(b.Sources) != 2 || b.Sources[0] != "home.pxar:/p1/home" || b.Sources[1] != "root.pxar:/p1/root" {
		t.Errorf("Sources = %v, want [home.pxar:/p1/home root.pxar:/p1/root]", b.Sources)
	}
}
