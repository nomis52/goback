package cron

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testAvailableWorkflows = map[string]bool{
	"backup":   true,
	"poweroff": true,
	"test":     true,
}

func TestParseTriggerSpecs_ValidSingleTrigger(t *testing.T) {
	specs, err := ParseTriggerSpecs("backup:0 2 * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 1)

	assert.Equal(t, []string{"backup"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)
}

func TestParseTriggerSpecs_ValidMultipleWorkflows(t *testing.T) {
	specs, err := ParseTriggerSpecs("backup,poweroff:0 2 * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 1)

	assert.Equal(t, []string{"backup", "poweroff"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)
}

func TestParseTriggerSpecs_ValidMultipleTriggers(t *testing.T) {
	specs, err := ParseTriggerSpecs("backup,poweroff:0 2 * * *;test:0 3 * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 2)

	assert.Equal(t, []string{"backup", "poweroff"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)

	assert.Equal(t, []string{"test"}, specs[1].Workflows)
	assert.Equal(t, "0 3 * * *", specs[1].CronSpec)
}

func TestParseTriggerSpecs_WhitespaceHandling(t *testing.T) {
	specs, err := ParseTriggerSpecs("  backup , poweroff : 0 2 * * * ; test : 0 3 * * *  ", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 2)

	assert.Equal(t, []string{"backup", "poweroff"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)

	assert.Equal(t, []string{"test"}, specs[1].Workflows)
	assert.Equal(t, "0 3 * * *", specs[1].CronSpec)
}

func TestParseTriggerSpecs_TrailingSemicolon(t *testing.T) {
	specs, err := ParseTriggerSpecs("backup:0 2 * * *;", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 1)

	assert.Equal(t, []string{"backup"}, specs[0].Workflows)
}

func TestParseTriggerSpecs_DuplicateWorkflowsAcrossTriggers(t *testing.T) {
	// Duplicate workflows across different triggers should be allowed
	specs, err := ParseTriggerSpecs("backup:0 2 * * *;backup:0 14 * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 2)

	assert.Equal(t, []string{"backup"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)

	assert.Equal(t, []string{"backup"}, specs[1].Workflows)
	assert.Equal(t, "0 14 * * *", specs[1].CronSpec)
}

func TestParseTriggerSpecs_EmptySpec(t *testing.T) {
	_, err := ParseTriggerSpecs("", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestParseTriggerSpecs_WhitespaceOnlySpec(t *testing.T) {
	_, err := ParseTriggerSpecs("   ", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestParseTriggerSpecs_MissingColon(t *testing.T) {
	_, err := ParseTriggerSpecs("backup,poweroff", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected format 'workflows:cron'")
}

func TestParseTriggerSpecs_MissingWorkflows(t *testing.T) {
	_, err := ParseTriggerSpecs(":0 2 * * *", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing workflows")
}

func TestParseTriggerSpecs_MissingCronSpec(t *testing.T) {
	_, err := ParseTriggerSpecs("backup,poweroff:", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing cron schedule")
}

func TestParseTriggerSpecs_InvalidCronExpression(t *testing.T) {
	_, err := ParseTriggerSpecs("backup:invalid cron", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron expression")
}

func TestParseTriggerSpecs_UnknownWorkflow(t *testing.T) {
	_, err := ParseTriggerSpecs("unknown:0 2 * * *", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown workflow 'unknown'")
	assert.Contains(t, err.Error(), "(available: ")
}

func TestParseTriggerSpecs_DuplicateWorkflowInTrigger(t *testing.T) {
	_, err := ParseTriggerSpecs("backup,backup:0 2 * * *", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate workflow 'backup'")
}

func TestParseTriggerSpecs_OnlySemicolons(t *testing.T) {
	_, err := ParseTriggerSpecs(";;;", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid triggers")
}

func TestParseTriggerSpecs_EmptyWorkflowInList(t *testing.T) {
	// Should skip empty workflow names and succeed
	specs, err := ParseTriggerSpecs("backup,,poweroff:0 2 * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, []string{"backup", "poweroff"}, specs[0].Workflows)
}

func TestParseTriggerSpecs_AllWorkflowsEmpty(t *testing.T) {
	_, err := ParseTriggerSpecs(",,:0 2 * * *", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid workflows")
}

func TestParseTriggerSpecs_ComplexValid(t *testing.T) {
	specs, err := ParseTriggerSpecs("backup:0 2 * * *;poweroff:0 3 * * *;test:*/5 * * * *", testAvailableWorkflows)
	require.NoError(t, err)
	require.Len(t, specs, 3)

	assert.Equal(t, []string{"backup"}, specs[0].Workflows)
	assert.Equal(t, "0 2 * * *", specs[0].CronSpec)

	assert.Equal(t, []string{"poweroff"}, specs[1].Workflows)
	assert.Equal(t, "0 3 * * *", specs[1].CronSpec)

	assert.Equal(t, []string{"test"}, specs[2].Workflows)
	assert.Equal(t, "*/5 * * * *", specs[2].CronSpec)
}

func TestParseTriggerSpecs_MultipleColons(t *testing.T) {
	_, err := ParseTriggerSpecs("backup:0:2:* * *", testAvailableWorkflows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected format 'workflows:cron'")
}
