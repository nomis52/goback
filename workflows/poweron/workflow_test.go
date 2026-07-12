package poweron

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/workflows"
)

func TestNewWorkflow(t *testing.T) {
	t.Run("valid config builds a power-on-only workflow", func(t *testing.T) {
		wf, err := NewWorkflow(workflows.Params{
			Config: &config.Config{
				PBS: config.PBSConfig{
					Host: "https://pbs.test",
					IPMI: config.IPMIConfig{Host: "ipmi.test"},
				},
			},
			Logger: slog.Default(),
		})
		require.NoError(t, err)
		require.NotNil(t, wf)

		activities := make(map[string]bool)
		for id := range wf.GetAllResults() {
			activities[id.ShortString()] = true
		}
		assert.Equal(t, map[string]bool{"poweron.PowerOnPBS": true}, activities)
	})

	t.Run("invalid PBS host fails", func(t *testing.T) {
		wf, err := NewWorkflow(workflows.Params{
			Config: &config.Config{
				PBS: config.PBSConfig{Host: "pbs-without-scheme"},
			},
			Logger: slog.Default(),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create PBS client")
		assert.Nil(t, wf)
	})
}
