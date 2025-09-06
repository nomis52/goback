package activities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowerOffPBS_Init(t *testing.T) {
	activity := &PowerOffPBS{}
	err := activity.Init()
	assert.NoError(t, err)
}
