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

// TestBackupWorkflows checks that every public backup factory builds a workflow with
// exactly the expected activities. The backup workflows are pure work: they must NOT
// include PowerOnPBS or PowerOffPBS — powering PBS on and off is the job of the
// separate poweron/poweroff workflows, composed around these by config.
func TestBackupWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		build    func(workflows.Params) (workflow.Workflow, error)
		expected map[string]bool
	}{
		{
			name:  "combined-backup backs up VMs and dirs",
			build: NewCombinedWorkflow,
			expected: map[string]bool{
				"backup.BackupVMs":  true,
				"backup.BackupDirs": true,
			},
		},
		{
			name:  "compute-backup backs up VMs only",
			build: NewComputeWorkflow,
			expected: map[string]bool{
				"backup.BackupVMs": true,
			},
		},
		{
			name:  "dir-backup backs up dirs only",
			build: NewDirsWorkflow,
			expected: map[string]bool{
				"backup.BackupDirs": true,
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
