package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/nomis52/goback/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Helpers
// ---------------------------------------------------------------------

// getResult is a helper to get a result for an activity from the orchestrator
func getResult(o *Orchestrator, activity Activity) *Result {
	allResults := o.GetAllResults()
	activityID := GetActivityID(activity)
	return allResults[activityID]
}

// Tests
// ---------------------------------------------------------------------

// TestOrchestrator_NoActivities tests orchestrator with no activities
func TestOrchestrator_NoActivities(t *testing.T) {
	orchestrator := NewOrchestrator()

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Should handle no activities gracefully")

	allResults := orchestrator.GetAllResults()
	assert.Empty(t, allResults, "Should have no results")
}

// TestOrchestrator_DuplicateActivityDetection tests that duplicate activities are rejected
func TestOrchestrator_DuplicateActivityDetection(t *testing.T) {
	orchestrator := NewOrchestrator()

	// Add first activity
	activity1 := &PassActivity{}
	err := orchestrator.AddActivity(activity1)
	require.NoError(t, err, "First activity should be added successfully")

	// Try to add another activity of the same type
	activity2 := &PassActivity{}
	err = orchestrator.AddActivity(activity2)
	require.Error(t, err, "Should reject duplicate activity")
	assert.Contains(t, err.Error(), "already exists", "Error message should mention duplicate")
}

// TestOrchestrator_BasicFeatures tests basic orchestrator functionality
func TestOrchestrator_BasicFeatures(t *testing.T) {
	t.Run("ActivityExecution", func(t *testing.T) {
		orchestrator := NewOrchestrator()
		activity := &PassActivity{}

		err := orchestrator.AddActivity(activity)
		require.NoError(t, err)

		err = orchestrator.Execute(context.Background())
		require.NoError(t, err)

		assert.True(t, activity.Executed, "Activity should be executed")
		result := getResult(orchestrator, activity)
		assert.Equal(t, Completed, result.State)
		assert.Nil(t, result.Error)
	})

	t.Run("ActivityResults", func(t *testing.T) {
		orchestrator := NewOrchestrator()
		activity := &PassActivity{}

		// Results available before Execute
		err := orchestrator.AddActivity(activity)
		require.NoError(t, err)

		result := getResult(orchestrator, activity)
		require.NotNil(t, result)
		assert.Equal(t, NotStarted, result.State)

		// Execute
		err = orchestrator.Execute(context.Background())
		require.NoError(t, err)

		// Results updated after Execute
		result = getResult(orchestrator, activity)
		assert.Equal(t, Completed, result.State)
	})

	t.Run("GetAllResults", func(t *testing.T) {
		orchestrator := NewOrchestrator()
		activity1 := &PassActivity{}
		activity2 := &FailActivity{}

		err := orchestrator.AddActivity(activity1, activity2)
		require.NoError(t, err)

		err = orchestrator.Execute(context.Background())
		require.Error(t, err) // FailActivity will cause an error

		results := orchestrator.GetAllResults()
		assert.Len(t, results, 2)
	})
}

// TestOrchestrator_ComprehensiveFeatures tests complete orchestrator functionality
func TestOrchestrator_ComprehensiveFeatures(t *testing.T) {
	t.Run("ResultAccessPatterns", func(t *testing.T) {
		orchestrator := NewOrchestrator()
		activity := &PassActivity{}

		// 1. Add activity
		err := orchestrator.AddActivity(activity)
		require.NoError(t, err)

		// 2. Results immediately available in NotStarted state
		result := getResult(orchestrator, activity)
		require.NotNil(t, result)
		assert.Equal(t, NotStarted, result.State)
		assert.Nil(t, result.Error)

		// 3. Execute
		err = orchestrator.Execute(context.Background())
		require.NoError(t, err)

		// 4. Results reflect execution outcome
		result = getResult(orchestrator, activity)
		assert.Equal(t, Completed, result.State)
		assert.Nil(t, result.Error)
		assert.True(t, activity.Executed)
	})
}

// TestOrchestrator_FailureHandling tests how orchestrator handles failures
func TestOrchestrator_FailureHandling(t *testing.T) {
	t.Run("ConfigurationError", func(t *testing.T) {
		// Invalid config: missing host
		config := TestConfig{
			Database: DatabaseConfig{
				Host: "", // Empty host will cause Init() to fail
				Port: 5432,
			},
		}

		logger := &MockLogger{}

		orchestrator := NewOrchestrator(WithConfig(config))
		err := orchestrator.Inject(logger)
		require.NoError(t, err)

		activity := &DatabaseSetupActivity{}

		err = orchestrator.AddActivity(activity)
		require.NoError(t, err)

		err = orchestrator.Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "initialization failed")

		result := getResult(orchestrator, activity)
		assert.Equal(t, NotStarted, result.State)
	})

	t.Run("MissingDependency", func(t *testing.T) {
		orchestrator := NewOrchestrator()
		activity := &DatabaseSetupActivity{}

		err := orchestrator.AddActivity(activity)
		require.NoError(t, err)

		// Don't inject logger
		err = orchestrator.Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has nil dependency")

		result := getResult(orchestrator, activity)
		assert.Equal(t, NotStarted, result.State)
	})
}

// TestOrchestrator_CircularDependencyDetection tests circular dependency detection
func TestOrchestrator_CircularDependencyDetection(t *testing.T) {
	orchestrator := NewOrchestrator()
	first := &FirstCircularActivity{}
	second := &SecondCircularActivity{}

	err := orchestrator.AddActivity(first, second)
	require.NoError(t, err)

	err = orchestrator.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestOrchestrator_DependencyInjection tests comprehensive dependency injection features
func TestOrchestrator_DependencyInjection(t *testing.T) {
	// Configuration for activities
	config := TestConfig{
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 5432,
		},
		Service: ServiceConfig{
			Timeout: 30,
		},
	}

	logger := &MockLogger{}

	orchestrator := NewOrchestrator(
		WithConfig(config),
	)

	err := orchestrator.Inject(logger)
	require.NoError(t, err)

	setup := &DatabaseSetupActivity{}
	migration := &DataMigrationActivity{}

	err = orchestrator.AddActivity(setup, migration)
	require.NoError(t, err)

	err = orchestrator.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, setup.Executed)
	assert.True(t, migration.Executed)
}

// TestOrchestrator_ImmediateResultAvailability tests that results are available immediately after AddActivity
func TestOrchestrator_ImmediateResultAvailability(t *testing.T) {
	orchestrator := NewOrchestrator()
	activity := &PassActivity{}

	// Before adding activity
	results := orchestrator.GetAllResults()
	assert.Empty(t, results)

	// Add activity
	err := orchestrator.AddActivity(activity)
	require.NoError(t, err)

	// Results immediately available
	results = orchestrator.GetAllResults()
	assert.Len(t, results, 1)

	id := GetActivityID(activity)
	result, exists := results[id]
	require.True(t, exists)
	assert.Equal(t, NotStarted, result.State)
	assert.Nil(t, result.Error)
}

// TestOrchestrator_LogCapture tests that logs are captured via LoggerHook
func TestOrchestrator_LogCapture(t *testing.T) {
	// Create collector and hook using actual logging package
	collector := logging.NewLogCollector()
	hook := logging.NewCapturingLoggerHook(collector)

	// Create base logger
	baseLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create orchestrator with logger and hook
	orchestrator := NewOrchestrator(
		WithLogHook(hook),
	)

	// Inject the base logger so activities can receive it
	err := orchestrator.Inject(baseLogger)
	require.NoError(t, err)

	// Create and add logging activity
	activity := &LoggingActivity{}
	err = orchestrator.AddActivity(activity)
	require.NoError(t, err)

	// Execute
	err = orchestrator.Execute(context.Background())
	require.NoError(t, err)

	// Verify activity executed
	assert.True(t, activity.Executed)

	// Get activity ID
	activityID := GetActivityID(activity).String()

	// Verify logs were captured
	logs := collector.GetLogs(activityID)
	require.NotEmpty(t, logs, "Expected logs to be captured")

	// Verify we got logs from both Init and Execute
	var hasInitLog bool
	var hasExecuteLog bool
	for _, log := range logs {
		if log.Message == "Initializing LoggingActivity" {
			hasInitLog = true
		}
		if log.Message == "Executing LoggingActivity" {
			hasExecuteLog = true
		}
	}

	assert.True(t, hasInitLog, "Expected log from Init()")
	assert.True(t, hasExecuteLog, "Expected log from Execute()")

	t.Logf("Captured %d logs for activity %s", len(logs), activityID)
	for i, log := range logs {
		t.Logf("  Log %d: [%s] %s", i, log.Level, log.Message)
	}
}

// ---------------------------------------------------------------------
// Test Activity Definitions
// ---------------------------------------------------------------------
// PassActivity - For testing successful scenarios
type PassActivity struct {
	Executed bool
}

func (a *PassActivity) Init() error { return nil }

func (a *PassActivity) Execute(ctx context.Context) error {
	a.Executed = true
	return nil
}

// FailActivity - For testing failure scenarios
type FailActivity struct {
	Executed bool
}

func (a *FailActivity) Init() error { return nil }

func (a *FailActivity) Execute(ctx context.Context) error {
	a.Executed = true
	return fmt.Errorf("intentional failure")
}

// Test activities for failure scenarios
type DependentOnFailingActivity struct {
	_        *FailActivity // Unnamed dependency on failing activity
	Executed bool
}

func (d *DependentOnFailingActivity) Init() error { return nil }
func (d *DependentOnFailingActivity) Execute(ctx context.Context) error {
	d.Executed = true
	return nil
}

// Two activities that depends on each other
type FirstCircularActivity struct {
	Second *SecondCircularActivity
}

func (f *FirstCircularActivity) Init() error { return nil }
func (f *FirstCircularActivity) Execute(ctx context.Context) error {
	return nil
}

type SecondCircularActivity struct {
	First *FirstCircularActivity
}

func (s *SecondCircularActivity) Init() error { return nil }
func (s *SecondCircularActivity) Execute(ctx context.Context) error {
	return nil
}

// DatabaseSetupActivity - Foundation activity with config injection
type DatabaseSetupActivity struct {
	Host     string      `config:"database.host"`
	Port     int         `config:"database.port"`
	Logger   *MockLogger // Service injection
	Executed bool
}

func (a *DatabaseSetupActivity) Init() error {
	if a.Host == "" {
		return fmt.Errorf("database host not configured")
	}
	if a.Logger == nil {
		return fmt.Errorf("logger not injected")
	}
	return nil
}

func (a *DatabaseSetupActivity) Execute(ctx context.Context) error {
	a.Logger.Log("Setting up database")
	a.Executed = true
	return nil
}

// DataMigrationActivity - Depends on DatabaseSetupActivity
type DataMigrationActivity struct {
	Setup          *DatabaseSetupActivity // Activity dependency
	ServiceTimeout int                    `config:"service.timeout"`
	Logger         *MockLogger            // Service injection
	Executed       bool
}

func (a *DataMigrationActivity) Init() error {
	if a.Setup == nil {
		return fmt.Errorf("database setup not injected")
	}
	return nil
}

func (a *DataMigrationActivity) Execute(ctx context.Context) error {
	if !a.Setup.Executed {
		return fmt.Errorf("database setup not executed")
	}
	a.Logger.Log("Running data migration")
	a.Executed = true
	return nil
}

// BackupServiceActivity - Depends on DataMigrationActivity
type BackupServiceActivity struct {
	Migration *DataMigrationActivity
	Executed  bool
}

func (b *BackupServiceActivity) Init() error { return nil }
func (b *BackupServiceActivity) Execute(ctx context.Context) error {
	if !b.Migration.Executed {
		return fmt.Errorf("migration not executed")
	}
	b.Executed = true
	return nil
}

// CleanupTaskActivity - Independent activity
type CleanupTaskActivity struct {
	Executed bool
}

func (c *CleanupTaskActivity) Init() error { return nil }
func (c *CleanupTaskActivity) Execute(ctx context.Context) error {
	c.Executed = true
	return nil
}

// AdvancedOrderingActivity - Demonstrates complex unnamed dependency pattern
// This activity depends on multiple activities but doesn't need to reference them
type AdvancedOrderingActivity struct {
	// Named dependency - we need to check its state
	Database *DatabaseSetupActivity
	// Unnamed dependencies - we just need them to run first for ordering
	_        *DataMigrationActivity
	_        *BackupServiceActivity
	_        *CleanupTaskActivity
	Executed bool
}

func (a *AdvancedOrderingActivity) Init() error { return nil }

func (a *AdvancedOrderingActivity) Execute(ctx context.Context) error {
	// We can access Database but not the unnamed dependencies
	// This is perfect for "run after these complete" scenarios
	if a.Database != nil && !a.Database.Executed {
		return fmt.Errorf("database not ready")
	}
	a.Executed = true
	return nil
}

// LoggingActivity - Activity that logs in both Init() and Execute()
type LoggingActivity struct {
	Logger   *slog.Logger
	Executed bool
}

func (a *LoggingActivity) Init() error {
	if a.Logger == nil {
		return fmt.Errorf("logger not injected")
	}
	a.Logger.Info("Initializing LoggingActivity")
	return nil
}

func (a *LoggingActivity) Execute(ctx context.Context) error {
	a.Logger.Info("Executing LoggingActivity")
	a.Logger.Debug("Debug message from Execute")
	a.Executed = true
	return nil
}

// ---------------------------------------------------------------------
// Mock Logger
// ---------------------------------------------------------------------
type MockLogger struct {
	Logs []string
	mu   sync.Mutex
}

func (m *MockLogger) Log(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logs = append(m.Logs, message)
}

// ---------------------------------------------------------------------
// Test Config Structs
// ---------------------------------------------------------------------
type DatabaseConfig struct {
	Host string
	Port int
}

type ServiceConfig struct {
	Timeout int
}

type TestConfig struct {
	Database DatabaseConfig
	Service  ServiceConfig
}
