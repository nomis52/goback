package dirbackup

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/workflows"
)

func TestNewWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr string
	}{
		{
			name: "valid config builds workflow",
			cfg: &config.Config{
				Proxmox: config.ProxmoxConfig{Host: "https://proxmox.test"},
				PBS: config.PBSConfig{
					Host: "https://pbs.test",
					IPMI: config.IPMIConfig{Host: "ipmi.test"},
				},
			},
		},
		{
			name: "invalid PBS host fails",
			cfg: &config.Config{
				Proxmox: config.ProxmoxConfig{Host: "https://proxmox.test"},
				PBS:     config.PBSConfig{Host: "pbs-without-scheme"},
			},
			wantErr: "failed to create directory backup workflow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wf, err := NewWorkflow(workflows.Params{
				Config: tc.cfg,
				Logger: slog.Default(),
			})

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Nil(t, wf)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, wf)

			// The composed workflow must run PowerOnPBS → BackupDirs then
			// PowerOffPBS, and must NOT include the VM backup activity.
			activities := make(map[string]bool)
			for id := range wf.GetAllResults() {
				activities[id.ShortString()] = true
			}
			assert.Equal(t, map[string]bool{
				"backup.PowerOnPBS":    true,
				"backup.BackupDirs":    true,
				"poweroff.PowerOffPBS": true,
			}, activities)
			assert.NotContains(t, activities, "backup.BackupVMs")
		})
	}
}
