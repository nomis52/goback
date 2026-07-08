package backup

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/workflow"
	"github.com/nomis52/goback/workflows"
)

func validConfig() *config.Config {
	return &config.Config{
		Proxmox: config.ProxmoxConfig{Host: "https://proxmox.test"},
		PBS: config.PBSConfig{
			Host: "https://pbs.test",
			IPMI: config.IPMIConfig{Host: "ipmi.test"},
		},
	}
}

func activitySet(wf workflow.Workflow) map[string]bool {
	activities := make(map[string]bool)
	for id := range wf.GetAllResults() {
		activities[id.ShortString()] = true
	}
	return activities
}

// TestBackupWorkflows checks that every public backup factory builds a workflow
// whose activity set is exactly the expected one. Crucially, every backup workflow
// must include poweroff.PowerOffPBS — the power-on → work → power-off cycle is the
// invariant these factories exist to guarantee. This is the regression guard for the
// old "full-backup" bug, where the workflow powered on and did work but never powered
// off.
func TestBackupWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		build    func(workflows.Params) (workflow.Workflow, error)
		expected map[string]bool
	}{
		{
			name:  "full-backup powers on, backs up VMs and dirs, then powers off",
			build: NewFullBackupWorkflow,
			expected: map[string]bool{
				"backup.PowerOnPBS":    true,
				"backup.BackupVMs":     true,
				"backup.BackupDirs":    true,
				"poweroff.PowerOffPBS": true,
			},
		},
		{
			name:  "serial-full-backup powers on, backs up VMs and dirs, then powers off",
			build: NewSerialFullBackupWorkflow,
			expected: map[string]bool{
				"backup.PowerOnPBS":    true,
				"backup.BackupVMs":     true,
				"backup.BackupDirs":    true,
				"poweroff.PowerOffPBS": true,
			},
		},
		{
			name:  "compute-backup powers on, backs up VMs, then powers off",
			build: NewComputeBackupWorkflow,
			expected: map[string]bool{
				"backup.PowerOnPBS":    true,
				"backup.BackupVMs":     true,
				"poweroff.PowerOffPBS": true,
			},
		},
		{
			name:  "dir-backup powers on, backs up dirs, then powers off",
			build: NewDirBackupWorkflow,
			expected: map[string]bool{
				"backup.PowerOnPBS":    true,
				"backup.BackupDirs":    true,
				"poweroff.PowerOffPBS": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wf, err := tc.build(workflows.Params{
				Config: validConfig(),
				Logger: slog.Default(),
			})
			require.NoError(t, err)
			require.NotNil(t, wf)
			assert.Equal(t, tc.expected, activitySet(wf))
		})
	}
}

// TestBackupWorkflows_InvalidConfig ensures factory construction fails cleanly when
// the PBS host is malformed (surfaced while building the shared dependencies).
func TestBackupWorkflows_InvalidConfig(t *testing.T) {
	builds := map[string]func(workflows.Params) (workflow.Workflow, error){
		"full-backup":        NewFullBackupWorkflow,
		"serial-full-backup": NewSerialFullBackupWorkflow,
		"compute-backup":     NewComputeBackupWorkflow,
		"dir-backup":         NewDirBackupWorkflow,
	}

	cfg := &config.Config{
		Proxmox: config.ProxmoxConfig{Host: "https://proxmox.test"},
		PBS:     config.PBSConfig{Host: "pbs-without-scheme"},
	}

	for name, build := range builds {
		t.Run(name, func(t *testing.T) {
			wf, err := build(workflows.Params{Config: cfg, Logger: slog.Default()})
			require.Error(t, err)
			assert.Nil(t, wf)
		})
	}
}
