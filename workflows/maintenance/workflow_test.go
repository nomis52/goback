package maintenance

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
		PBS: config.PBSConfig{
			Host:      "https://pbs.test",
			IPMI:      config.IPMIConfig{Host: "ipmi.test"},
			Token:     "goback@pbs!maint:secret",
			PruneJob:  "goback-prune",
			Datastore: "tank",
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

// TestMaintenanceWorkflows checks that every public factory builds a workflow with
// exactly the expected activities. Each one powers PBS on first (via a PowerOnPBS
// dependency); none of them power PBS off.
func TestMaintenanceWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		build    func(workflows.Params) (workflow.Workflow, error)
		expected map[string]bool
	}{
		{
			name:  "pbs-prune powers on, then prunes",
			build: NewPruneWorkflow,
			expected: map[string]bool{
				"poweron.PowerOnPBS":   true,
				"maintenance.PrunePBS": true,
			},
		},
		{
			name:  "pbs-gc powers on, then garbage-collects",
			build: NewGCWorkflow,
			expected: map[string]bool{
				"poweron.PowerOnPBS":            true,
				"maintenance.GarbageCollectPBS": true,
			},
		},
		{
			name:  "pbs-maintenance powers on, prunes, then garbage-collects",
			build: NewWorkflow,
			expected: map[string]bool{
				"poweron.PowerOnPBS":            true,
				"maintenance.PrunePBS":          true,
				"maintenance.GarbageCollectPBS": true,
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

func TestMaintenanceWorkflowsInvalidPBSHost(t *testing.T) {
	tests := []struct {
		name  string
		build func(workflows.Params) (workflow.Workflow, error)
	}{
		{name: "pbs-prune", build: NewPruneWorkflow},
		{name: "pbs-gc", build: NewGCWorkflow},
		{name: "pbs-maintenance", build: NewWorkflow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PBS.Host = "pbs-without-scheme"

			wf, err := tc.build(workflows.Params{Config: cfg, Logger: slog.Default()})
			require.Error(t, err)
			assert.Equal(t, "failed to create PBS client: host URL must include scheme (http:// or https://): pbs-without-scheme", err.Error())
			assert.Nil(t, wf)
		})
	}
}
