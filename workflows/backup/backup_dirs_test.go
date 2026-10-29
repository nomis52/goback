package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildBackupCommand(t *testing.T) {
	target := "goback@pbs!goback@pbs.example:8007:store"
	sources := []string{"etc.pxar:/etc", "home.pxar:/home"}

	cmd := buildBackupCommand(target, sources)

	assert.Equal(t,
		"PBS_PASSWORD_FD=0 proxmox-backup-client backup etc.pxar:/etc home.pxar:/home --repository 'goback@pbs!goback@pbs.example:8007:store'",
		cmd)
	// The token must never be part of the command line.
	assert.NotContains(t, cmd, "PBS_PASSWORD=")
}
