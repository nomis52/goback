package config

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Test timeout constants
	testBootTimeout     = 10 * time.Minute
	testShutdownTimeout = time.Minute
	testBackupTimeout   = time.Minute
	testMaxBackupAge    = 24 * time.Hour
	
	// Test time strings for parsing tests
	test24Hours = 24 * time.Hour
	test48Hours = 48 * time.Hour
	test30Min   = 30 * time.Minute
	test90Min   = 90 * time.Minute
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
					BootTimeout:     testBootTimeout,
					ShutdownTimeout: testShutdownTimeout,
				},
				Proxmox: ProxmoxConfig{
					Host:          "1.2.3.5",
					Token:         "t",
					Storage:       "s",
					BackupTimeout: testBackupTimeout,
				},
				Compute:    ComputeConfig{MaxBackupAge: testMaxBackupAge},
				Monitoring: MonitoringConfig{VictoriaMetricsURL: "http://vm"},
			},
			wantErr: false,
		},
		{
			name:    "missing IPMI host",
			cfg:     Config{Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s"}, PBS: PBSConfig{Host: "h"}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing Proxmox host",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: testShutdownTimeout, ShutdownTimeout: testShutdownTimeout}, Proxmox: ProxmoxConfig{Token: "t", Storage: "s", BackupTimeout: testShutdownTimeout}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing PBS host",
			cfg:     Config{PBS: PBSConfig{IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: testShutdownTimeout, ShutdownTimeout: testShutdownTimeout}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: testShutdownTimeout}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "missing VictoriaMetrics URL",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: testShutdownTimeout, ShutdownTimeout: testShutdownTimeout}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: testShutdownTimeout}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}},
			wantErr: true,
		},
		{
			name:    "non-positive boot timeout",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: 0, ShutdownTimeout: testShutdownTimeout}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: testShutdownTimeout}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
			wantErr: true,
		},
		{
			name:    "non-positive backup job timeout",
			cfg:     Config{PBS: PBSConfig{Host: "h", IPMI: IPMIConfig{Host: "h", Username: "u", Password: "p"}, BootTimeout: testShutdownTimeout, ShutdownTimeout: testShutdownTimeout}, Proxmox: ProxmoxConfig{Host: "h", Token: "t", Storage: "s", BackupTimeout: 0}, Compute: ComputeConfig{MaxBackupAge: testMaxBackupAge}, Monitoring: MonitoringConfig{VictoriaMetricsURL: "u"}},
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
	
	assert.Equal(t, testBootTimeout, cfg.PBS.BootTimeout, "BootTimeout default")
	assert.Equal(t, 2*time.Hour, cfg.Proxmox.BackupTimeout, "BackupTimeout default")
	assert.Equal(t, 2*time.Minute, cfg.PBS.ShutdownTimeout, "ShutdownTimeout default")
	assert.Equal(t, testMaxBackupAge, cfg.Compute.MaxBackupAge, "MaxBackupAge default")
	assert.Equal(t, "pbs_automation", cfg.Monitoring.MetricsPrefix, "MetricsPrefix default")
	assert.Equal(t, "goback", cfg.Monitoring.JobName, "JobName default")
}

func TestLoadConfig(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	require.NoError(t, err, "failed to create temp file")
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
	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err, "failed to write temp config")
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	require.NoError(t, err, "LoadConfig should succeed")
	
	assert.Equal(t, "1.2.3.4", cfg.PBS.IPMI.Host, "PBS IPMI host")
	assert.Equal(t, "1.2.3.5", cfg.Proxmox.Host, "Proxmox host")
	assert.Equal(t, "1.2.3.6", cfg.PBS.Host, "PBS host")
	assert.Equal(t, testMaxBackupAge, cfg.Compute.MaxBackupAge, "Compute max backup age")
	assert.Equal(t, "http://vm", cfg.Monitoring.VictoriaMetricsURL, "VictoriaMetrics URL")
}

func TestLoadConfig_TimeStrings(t *testing.T) {
	tests := []struct {
	name     string
	maxAge   string
	expected time.Duration
	}{
	{"24h", "24h", test24Hours},
	{"48h", "48h", test48Hours},
	{"30m", "30m", test30Min},
	{"1h30m", "1h30m", test90Min},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
			require.NoError(t, err, "failed to create temp file")
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

			_, err = tmpfile.Write([]byte(content))
			require.NoError(t, err, "failed to write temp config")
			tmpfile.Close()

			cfg, err := LoadConfig(tmpfile.Name())
			require.NoError(t, err, "LoadConfig should succeed")

			assert.Equal(t, tt.expected, cfg.Compute.MaxBackupAge, "Compute max backup age")
		})
	}
}

func TestLoadConfig_Files(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "goback_config_test.yaml")
	require.NoError(t, err, "failed to create temp file")
	defer os.Remove(tmpfile.Name())

	// Create a temporary private key file for testing
	tmpKeyFile, err := os.CreateTemp("", "test_key")
	require.NoError(t, err, "failed to create temp key file")
	defer os.Remove(tmpKeyFile.Name())
	tmpKeyFile.WriteString("dummy key content")
	tmpKeyFile.Close()

	content := fmt.Sprintf(`pbs:
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
  user: root
  private_key_path: %s
  token: mytoken
  target: backup-client@pbs!token-name@10.6.0.10:tank
  sources:
    - home.pxar:/p1/home
    - root.pxar:/p1/root
`, tmpKeyFile.Name())
	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err, "failed to write temp config")
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	require.NoError(t, err, "LoadConfig should succeed")
	
	b := cfg.Files
	assert.Equal(t, "pve2", b.Host, "Files host")
	assert.Equal(t, "root", b.User, "Files user")
	assert.Equal(t, tmpKeyFile.Name(), b.PrivateKeyPath, "Files private key path")
	assert.Equal(t, "mytoken", b.Token, "Files token")
	assert.Equal(t, "backup-client@pbs!token-name@10.6.0.10:tank", b.Target, "Files target")
	assert.Len(t, b.Sources, 2, "Files sources length")
	assert.Equal(t, "home.pxar:/p1/home", b.Sources[0], "Files first source")
	assert.Equal(t, "root.pxar:/p1/root", b.Sources[1], "Files second source")
}
