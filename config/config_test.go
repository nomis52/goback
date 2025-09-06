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
				PBS: PBSConfig{
					Host: "1.2.3.6",
					IPMI: IPMIConfig{Host: "1.2.3.4", Username: "u", Password: "p"},
					BootTimeout:     10 * time.Minute,
					ShutdownTimeout: time.Minute,
				},
				Proxmox: ProxmoxConfig{
					Host:          "1.2.3.5",
					Token:         "t",
					Storage:       "s",
					BackupTimeout: time.Minute,
				},
				Compute:    ComputeConfig{MaxBackupAge: 24 * time.Hour},
				Monitoring: MonitoringConfig{VictoriaMetricsURL: "http://vm"},
			},
			wantErr: false,
		},
		{
			name:    "missing IPMI host",
			cfg:     Config{Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h"}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing Proxmox host",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: time.Second, ShutdownTimeout: time.Second}, Proxmox: ProxmoxConfig{Token: "t", Storage: "s", BackupTimeout: time.Second}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS host",
			cfg:     Config{PBS: PBSConfig{IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: time.Second, ShutdownTimeout: time.Second}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: time.Second}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing VictoriaMetrics URL",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: time.Second, ShutdownTimeout: time.Second}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: time.Second}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}},
			wantErr: true,
		},
		{
			name:    "non-positive boot timeout",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: 0, ShutdownTimeout: time.Second}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: time.Second}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "non-positive backup job timeout",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: time.Second, ShutdownTimeout: time.Second}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: 0}, Compute: ComputeConfig{MaxBackupAge: 24 * time.Hour}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
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
	if cfg.PBS.BootTimeout != 10*time.Minute {
		t.Errorf("BootTimeout default = %v, want %v", cfg.PBS.BootTimeout, 10*time.Minute)
	}
	if cfg.Proxmox.BackupTimeout != 2*time.Hour {
		t.Errorf("BackupTimeout default = %v, want %v", cfg.Proxmox.BackupTimeout, 2*time.Hour)
	}
	if cfg.PBS.ShutdownTimeout != 2*time.Minute {
		t.Errorf("ShutdownTimeout default = %v, want %v", cfg.PBS.ShutdownTimeout, 2*time.Minute)
	}
	if cfg.Compute.MaxBackupAge != 24*time.Hour {
		t.Errorf("MaxBackupAge default = %v, want %v", cfg.Compute.MaxBackupAge, 24*time.Hour)
	}
	if cfg.Monitoring.MetricsPrefix != "pbs_automation" {
		t.Errorf("MetricsPrefix default = %v, want %v", cfg.Monitoring.MetricsPrefix, "pbs_automation")
	}
	if cfg.Monitoring.JobName != "goback" {
		t.Errorf("JobName default = %v, want %v", cfg.Monitoring.JobName, "goback")
	}

}

func TestLoadConfig(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	content := `pbs:
  host: 1.2.3.6
  ipmi:
    host: 1.2.3.4
    username: user
    password: pass
  boot_timeout: 60s
  shutdown_timeout: 60s
proxmox:
  host: 1.2.3.5
  token: token123
  storage: storage1
  backup_timeout: 60s
compute:
  max_backup_age: 24h
monitoring:
  victoriametrics_url: http://vm
`
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.PBS.IPMI.Host != "1.2.3.4" {
		t.Errorf("PBS.IPMI.Host = %v, want %v", cfg.PBS.IPMI.Host, "1.2.3.4")
	}
	if cfg.Proxmox.Host != "1.2.3.5" {
		t.Errorf("Proxmox.Host = %v, want %v", cfg.Proxmox.Host, "1.2.3.5")
	}
	if cfg.PBS.Host != "1.2.3.6" {
		t.Errorf("PBS.Host = %v, want %v", cfg.PBS.Host, "1.2.3.6")
	}

	if cfg.Compute.MaxBackupAge != 24*time.Hour {
		t.Errorf("Compute.MaxBackupAge = %v, want %v", cfg.Compute.MaxBackupAge, 24*time.Hour)
	}
	if cfg.Monitoring.VictoriaMetricsURL != "http://vm" {
		t.Errorf("VictoriaMetricsURL = %v, want %v", cfg.Monitoring.VictoriaMetricsURL, "http://vm")
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

			content := fmt.Sprintf(`pbs:
  host: 1.2.3.6
  ipmi:
    host: 1.2.3.4
    username: user
    password: pass
  boot_timeout: 60s
  shutdown_timeout: 60s
proxmox:
  host: 1.2.3.5
  token: token123
  storage: storage1
  backup_timeout: 60s
compute:
  max_backup_age: %s
monitoring:
  victoriametrics_url: http://vm
`, tt.maxAge)

			if _, err := tmpfile.Write([]byte(content)); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}
			tmpfile.Close()

			cfg, err := LoadConfig(tmpfile.Name())
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}

			if cfg.Compute.MaxBackupAge != tt.expected {
				t.Errorf("Compute.MaxBackupAge = %v, want %v", cfg.Compute.MaxBackupAge, tt.expected)
			}
		})
	}
}

func TestLoadConfig_Files(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	content := `pbs:
  host: localhost
  ipmi:
    host: localhost
    username: user
    password: pass
  boot_timeout: 60s
  shutdown_timeout: 60s
proxmox:
  host: localhost
  token: token123
  storage: storage1
  backup_timeout: 60s
compute:
  max_backup_age: 24h
monitoring:
  victoriametrics_url: http://vm
files:
  host: pve2
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
	b := cfg.Files
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
