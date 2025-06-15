package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test activities for circular dependency scenarios
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

// BasicActivity - For testing failure scenarios
type BasicActivity struct {
	ShouldFail bool
	Executed   bool
}

func (a *BasicActivity) Init() error { return nil }

func (a *BasicActivity) Execute(ctx context.Context) error {
	a.Executed = true
	if a.ShouldFail {
		return fmt.Errorf("intentional failure")
	}
	return nil
}

// Test activities for failure scenarios
type DependentOnFailingActivity struct {
	_        *BasicActivity // Unnamed dependency on failing activity
	Executed bool
}

func (d *DependentOnFailingActivity) Init() error { return nil }
func (d *DependentOnFailingActivity) Execute(ctx context.Context) error {
	d.Executed = true
	return nil
}

// DatabaseSetupActivity - Foundation activity with config injection
type DatabaseSetupActivity struct {
	Host           string      `config:"database.host"`
	Port           int         `config:"database.port"`
	Logger         *MockLogger // Service injection
	Executed       bool
	ExecutionOrder int
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
	ExecutionOrder int
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

// BackupServiceActivity - Can run parallel to DataMigrationActivity
type BackupServiceActivity struct {
	Setup          *DatabaseSetupActivity // Activity dependency
	ServiceName    string                 `config:"service.name"`
	Executed       bool
	ExecutionOrder int
}

func (a *BackupServiceActivity) Init() error { return nil }

func (a *BackupServiceActivity) Execute(ctx context.Context) error {
	a.Executed = true
	return nil
}

// CleanupTaskActivity - Depends on both DataMigrationActivity and BackupServiceActivity
// Demonstrates both named and unnamed dependency patterns
type CleanupTaskActivity struct {
	Migration      *DataMigrationActivity // Named dependency - can access the activity
	_              *BackupServiceActivity // Unnamed dependency - ensures ordering only
	Executed       bool
	ExecutionOrder int
}

func (a *CleanupTaskActivity) Init() error { return nil }

func (a *CleanupTaskActivity) Execute(ctx context.Context) error {
	// Can access Migration but not the BackupServiceActivity (unnamed dep)
	// The unnamed dependency still ensures BackupServiceActivity runs first
	if a.Migration != nil && !a.Migration.Executed {
		return fmt.Errorf("migration dependency not satisfied")
	}
	a.Executed = true
	return nil
}

// AdvancedOrderingActivity - Demonstrates complex unnamed dependency pattern
// This activity depends on multiple activities but doesn't need to reference them
type AdvancedOrderingActivity struct {
	// Named dependency - we need to check its state
	Database *DatabaseSetupActivity
	// Unnamed dependencies - we just need them to run first for ordering
	_              *DataMigrationActivity
	_              *BackupServiceActivity
	_              *CleanupTaskActivity
	Executed       bool
	ExecutionOrder int
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

// TestOrchestrator_ComprehensiveFeatures tests all major orchestrator features
func TestOrchestrator_ComprehensiveFeatures(t *testing.T) {
	// Create test config
	config := &TestConfig{
		Database: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
			Port: 5432,
		},
		Service: struct {
			Name    string `yaml:"name"`
			Timeout int    `yaml:"timeout"`
		}{
			Name:    "test-service",
			Timeout: 30,
		},
	}

	// Create mock services
	logger := &MockLogger{}

	// Create activities (keep references for result access)
	dbSetup := &DatabaseSetupActivity{}
	dataMigration := &DataMigrationActivity{}
	backupService := &BackupServiceActivity{}
	cleanupTask := &CleanupTaskActivity{}

	// Create orchestrator with custom logger
	slogLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	orchestrator := NewOrchestrator(WithConfig(config), WithLogger(slogLogger))

	// Test service injection
	orchestrator.Inject(logger)

	// Test activity registration in random order
	err := orchestrator.AddActivity(cleanupTask, dataMigration)
	require.NoError(t, err, "Should add activities successfully")
	err = orchestrator.AddActivity(dbSetup, backupService)
	require.NoError(t, err, "Should add activities successfully")

	// Execute
	ctx := context.Background()
	err = orchestrator.Execute(ctx)

	// Verify successful execution
	require.NoError(t, err, "Expected no error during execution")

	// Verify all activities executed
	assert.True(t, dbSetup.Executed, "DatabaseSetupActivity should have executed")
	assert.True(t, dataMigration.Executed, "DataMigrationActivity should have executed")
	assert.True(t, backupService.Executed, "BackupServiceActivity should have executed")
	assert.True(t, cleanupTask.Executed, "CleanupTaskActivity should have executed")

	// Verify config injection
	assert.Equal(t, "localhost", dbSetup.Host, "Database host should be injected")
	assert.Equal(t, 5432, dbSetup.Port, "Database port should be injected")
	assert.Equal(t, 30, dataMigration.ServiceTimeout, "Service timeout should be injected")
	assert.Equal(t, "test-service", backupService.ServiceName, "Service name should be injected")

	// Verify service injection
	assert.Equal(t, logger, dbSetup.Logger, "Logger should be injected into DatabaseSetupActivity")
	assert.Equal(t, logger, dataMigration.Logger, "Logger should be injected into DataMigrationActivity")

	// Verify execution order constraints
	assert.Greater(t, dataMigration.ExecutionOrder, dbSetup.ExecutionOrder,
		"DataMigrationActivity should execute after DatabaseSetupActivity")
	assert.Greater(t, backupService.ExecutionOrder, dbSetup.ExecutionOrder,
		"BackupServiceActivity should execute after DatabaseSetupActivity")
	assert.Greater(t, cleanupTask.ExecutionOrder, dataMigration.ExecutionOrder,
		"CleanupTaskActivity should execute after DataMigrationActivity")
	assert.Greater(t, cleanupTask.ExecutionOrder, backupService.ExecutionOrder,
		"CleanupTaskActivity should execute after BackupServiceActivity (unnamed dependency)")

	// Test result access patterns
	t.Run("ResultAccessPatterns", func(t *testing.T) {
		// Pattern 1: Access by activity reference (recommended)
		result := orchestrator.GetResultByActivity(dbSetup)
		require.NotNil(t, result, "Should find result for DatabaseSetupActivity")
		assert.True(t, result.IsSuccess(), "DatabaseSetupActivity should have succeeded")

		result = orchestrator.GetResultByActivity(dataMigration)
		require.NotNil(t, result, "Should find result for DataMigrationActivity")
		assert.True(t, result.IsSuccess(), "DataMigrationActivity should have succeeded")

		// Pattern 2: Access by ActivityID
		dbSetupID := ActivityID{
			Module: "github.com/nomis52/goback/orchestrator", // This test package
			Type:   "DatabaseSetupActivity",
		}
		result = orchestrator.GetResult(dbSetupID)
		require.NotNil(t, result, "Should find result by ActivityID")
		assert.True(t, result.IsSuccess(), "Result should indicate success")

		// Pattern 3: Get all results
		allResults := orchestrator.GetAllResults()
		assert.Len(t, allResults, 4, "Should have results for all 4 activities")

		successCount := 0
		for _, result := range allResults {
			if result.IsSuccess() {
				successCount++
			}
		}
		assert.Equal(t, 4, successCount, "All activities should have succeeded")
	})

	// Test mock services were used
	messages := logger.GetMessages()
	assert.Contains(t, messages, "Setting up database", "Logger should have been used")
	assert.Contains(t, messages, "Running data migration", "Logger should have been used")
}

// TestOrchestrator_UnnamedDependencies specifically tests unnamed dependency patterns
func TestOrchestrator_UnnamedDependencies(t *testing.T) {
	config := &TestConfig{
		Database: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
		},
	}
	logger := &MockLogger{}

	// Create activities including one with complex unnamed dependencies
	dbSetup := &DatabaseSetupActivity{}
	dataMigration := &DataMigrationActivity{}
	backupService := &BackupServiceActivity{}
	cleanupTask := &CleanupTaskActivity{}
	advancedOrdering := &AdvancedOrderingActivity{}

	orchestrator := NewOrchestrator(WithConfig(config))
	orchestrator.Inject(logger)

	// Add activities in random order to test dependency resolution
	err := orchestrator.AddActivity(advancedOrdering, cleanupTask, backupService)
	require.NoError(t, err, "Should add activities successfully")
	err = orchestrator.AddActivity(dataMigration, dbSetup)
	require.NoError(t, err, "Should add activities successfully")

	err = orchestrator.Execute(context.Background())
	require.NoError(t, err, "Should execute with unnamed dependencies")

	// Verify all activities executed
	assert.True(t, dbSetup.Executed, "DatabaseSetupActivity should have executed")
	assert.True(t, dataMigration.Executed, "DataMigrationActivity should have executed")
	assert.True(t, backupService.Executed, "BackupServiceActivity should have executed")
	assert.True(t, cleanupTask.Executed, "CleanupTaskActivity should have executed")
	assert.True(t, advancedOrdering.Executed, "AdvancedOrderingActivity should have executed")

	// Verify complex ordering constraints with unnamed dependencies
	assert.Greater(t, cleanupTask.ExecutionOrder, backupService.ExecutionOrder,
		"CleanupTaskActivity should execute after BackupServiceActivity (unnamed dep)")
	assert.Greater(t, advancedOrdering.ExecutionOrder, dataMigration.ExecutionOrder,
		"AdvancedOrderingActivity should execute after DataMigrationActivity (unnamed dep)")
	assert.Greater(t, advancedOrdering.ExecutionOrder, backupService.ExecutionOrder,
		"AdvancedOrderingActivity should execute after BackupServiceActivity (unnamed dep)")
	assert.Greater(t, advancedOrdering.ExecutionOrder, cleanupTask.ExecutionOrder,
		"AdvancedOrderingActivity should execute after CleanupTaskActivity (unnamed dep)")

	// Verify that named dependencies are accessible while unnamed are not
	assert.NotNil(t, cleanupTask.Migration, "Named dependency should be accessible")
	assert.NotNil(t, advancedOrdering.Database, "Named dependency should be accessible")

	t.Log("Successfully tested complex unnamed dependency patterns")
}

// TestOrchestrator_FailureHandling tests various failure scenarios
func TestOrchestrator_FailureHandling(t *testing.T) {
	config := &TestConfig{
		Database: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
		},
	}

	t.Run("ActivityFailure", func(t *testing.T) {
		failingActivity := &BasicActivity{ShouldFail: true}

		orchestrator := NewOrchestrator()
		err := orchestrator.AddActivity(failingActivity)
		require.NoError(t, err, "Should add activity successfully")

		err = orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail when activity fails")

		// Check that failing activity result is available
		result := orchestrator.GetResultByActivity(failingActivity)
		require.NotNil(t, result, "Should have result for failing activity")
		assert.False(t, result.IsSuccess(), "Failing activity should report failure")
	})

	t.Run("ConfigurationError", func(t *testing.T) {
		// Empty config should cause validation error
		emptyConfig := &TestConfig{}
		logger := &MockLogger{}
		dbSetup := &DatabaseSetupActivity{}

		orchestrator := NewOrchestrator(WithConfig(emptyConfig))
		orchestrator.Inject(logger)
		err := orchestrator.AddActivity(dbSetup)
		require.NoError(t, err, "Should add activity successfully")

		err = orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail with empty database host")
		assert.Contains(t, err.Error(), "database host not configured")
	})

	t.Run("MissingDependency", func(t *testing.T) {
		// Activity without required service injection
		dbSetup := &DatabaseSetupActivity{}

		orchestrator := NewOrchestrator(WithConfig(config))
		// Don't inject logger - should cause validation error
		err := orchestrator.AddActivity(dbSetup)
		require.NoError(t, err, "Should add activity successfully")

		err = orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail with missing logger dependency")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		dbSetup := &DatabaseSetupActivity{}
		logger := &MockLogger{}

		orchestrator := NewOrchestrator(WithConfig(config))
		orchestrator.Inject(logger)
		err := orchestrator.AddActivity(dbSetup)
		require.NoError(t, err, "Should add activity successfully")

		// Cancel context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = orchestrator.Execute(ctx)
		require.Error(t, err, "Should fail with cancelled context")
		assert.Contains(t, err.Error(), "cancelled", "Error should indicate cancellation")
	})

	t.Run("UnnamedDependencyFailure", func(t *testing.T) {
		// Test that unnamed dependency failures properly prevent dependent activities
		failingDep := &BasicActivity{ShouldFail: true}
		dependent := &DependentOnFailingActivity{}

		orchestrator := NewOrchestrator()
		err := orchestrator.AddActivity(failingDep, dependent)
		require.NoError(t, err, "Should add activities successfully")

		err = orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail due to failing dependency")

		// Verify dependent activity didn't execute due to failed unnamed dependency
		assert.False(t, dependent.Executed, "Dependent activity should not execute when unnamed dependency fails")
		fmt.Println(orchestrator.GetResultByActivity((dependent)))
		assert.False(t, orchestrator.GetResultByActivity(dependent).IsSuccess())
	})
}

// TestOrchestrator_CircularDependencyDetection tests circular dependency detection
func TestOrchestrator_CircularDependencyDetection(t *testing.T) {
	// Create activities with circular dependency
	firstActivity := &FirstCircularActivity{}
	secondActivity := &SecondCircularActivity{}

	orchestrator := NewOrchestrator()
	err := orchestrator.AddActivity(firstActivity, secondActivity)
	require.NoError(t, err, "Should add activities successfully")

	err = orchestrator.Execute(context.Background())
	require.Error(t, err, "Should detect circular dependency")
	assert.Contains(t, err.Error(), "circular dependency", "Error should indicate circular dependency")

	// Both activities should remain in NotStarted state with no individual errors
	// (circular dependency is a structural issue, not an individual activity failure)
	firstResult := orchestrator.GetResultByActivity(firstActivity)
	require.NotNil(t, firstResult, "First activity result should not be nil")
	assert.Equal(t, NotStarted, firstResult.State, "First activity should remain NotStarted")
	assert.NoError(t, firstResult.Error, "First activity should have no individual error")

	secondResult := orchestrator.GetResultByActivity(secondActivity)
	require.NotNil(t, secondResult, "Second activity result should not be nil")
	assert.Equal(t, NotStarted, secondResult.State, "Second activity should remain NotStarted")
	assert.NoError(t, secondResult.Error, "Second activity should have no individual error")
}

// TestOrchestrator_AdvancedResultUsage demonstrates advanced result usage patterns
func TestOrchestrator_AdvancedResultUsage(t *testing.T) {
	config := &TestConfig{
		Database: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
		},
		Service: struct {
			Name    string `yaml:"name"`
			Timeout int    `yaml:"timeout"`
		}{
			Name: "advanced-test",
		},
	}

	logger := &MockLogger{}
	dbSetup := &DatabaseSetupActivity{}
	successActivity := &BasicActivity{ShouldFail: false}
	failActivity := &BasicActivity{ShouldFail: true}

	orchestrator := NewOrchestrator(WithConfig(config))
	orchestrator.Inject(logger)
	err := orchestrator.AddActivity(dbSetup, successActivity, failActivity)
	require.NoError(t, err, "Should add activities successfully")

	// Execute (will partially fail)
	err = orchestrator.Execute(context.Background())
	require.Error(t, err, "Should fail due to failing activity")

	t.Run("ConditionalLogicBasedOnResults", func(t *testing.T) {
		// Example: PBS automation logic
		dbResult := orchestrator.GetResultByActivity(dbSetup)
		successResult := orchestrator.GetResultByActivity(successActivity)
		failResult := orchestrator.GetResultByActivity(failActivity)

		// Check that we have valid results before using them
		require.NotNil(t, dbResult, "Should have result for database setup activity")
		require.NotNil(t, successResult, "Should have result for success activity")
		require.NotNil(t, failResult, "Should have result for fail activity")

		if dbResult.IsSuccess() && successResult.IsSuccess() {
			t.Log("Database and success activity completed - could proceed with cleanup")
		}

		if !failResult.IsSuccess() {
			t.Log("Fail activity failed as expected - handling failure scenario")
		}

		// Verify expected states
		assert.True(t, dbResult.IsSuccess(), "Database setup should succeed")
		assert.True(t, successResult.IsSuccess(), "Success activity should succeed")
		assert.False(t, failResult.IsSuccess(), "Fail activity should fail")
	})

	t.Run("BatchResultProcessing", func(t *testing.T) {
		allResults := orchestrator.GetAllResults()
		successCount := 0
		failureCount := 0

		for id, result := range allResults {
			if result.IsSuccess() {
				successCount++
				t.Logf("Activity %s succeeded", id.ShortString())
			} else {
				failureCount++
				t.Logf("Activity %s failed", id.ShortString())
			}
		}

		assert.Equal(t, 2, successCount, "Should have 2 successful activities")
		assert.Equal(t, 1, failureCount, "Should have 1 failed activity")
	})
}

// TestOrchestrator_NoActivities tests orchestrator with no activities
func TestOrchestrator_NoActivities(t *testing.T) {
	orchestrator := NewOrchestrator()

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Should handle no activities gracefully")

	allResults := orchestrator.GetAllResults()
	assert.Empty(t, allResults, "Should have no results")
}

// TestOrchestrator_DependencyInjection tests dependency injection behavior
func TestOrchestrator_DependencyInjection(t *testing.T) {
	orchestrator := NewOrchestrator()
	logger := &MockLogger{}

	// Test injecting the same dependency multiple times
	err := orchestrator.Inject(logger)
	require.NoError(t, err, "First injection should succeed")

	err = orchestrator.Inject(logger)
	require.Error(t, err, "Second injection of same type should succeed")

	// Test injecting nil dependency
	err = orchestrator.Inject(nil)
	require.NoError(t, err, "Injecting nil should not return error")

	// Test injecting multiple dependencies at once
	anotherLogger := &MockLogger{}
	err = orchestrator.Inject(logger, anotherLogger)
	require.Error(t, err, "Injecting multiple dependencies should not succeed")
}

// TestOrchestrator_ImmediateResultAvailability tests that results are available immediately after AddActivity
func TestOrchestrator_ImmediateResultAvailability(t *testing.T) {
	activity := &BasicActivity{ShouldFail: false}

	orchestrator := NewOrchestrator()

	// Before adding activity - should return nil
	result := orchestrator.GetResultByActivity(activity)
	assert.Nil(t, result, "Result should be nil before activity is added")

	// Add activity
	err := orchestrator.AddActivity(activity)
	require.NoError(t, err, "Should add activity successfully")

	// Immediately after adding - should have NotStarted result
	result = orchestrator.GetResultByActivity(activity)
	require.NotNil(t, result, "Result should not be nil immediately after AddActivity")
	assert.Equal(t, NotStarted, result.State, "Initial state should be NotStarted")
	assert.NoError(t, result.Error, "Initial error should be nil")
	assert.False(t, activity.Executed, "Activity should not have executed yet")

	// Execute and verify state progression
	err = orchestrator.Execute(context.Background())
	require.NoError(t, err, "Execution should succeed")

	// Result should now be Completed
	result = orchestrator.GetResultByActivity(activity)
	require.NotNil(t, result, "Result should still not be nil after execution")
	assert.Equal(t, Completed, result.State, "Final state should be Completed")
	assert.NoError(t, result.Error, "Final error should be nil for successful execution")
	assert.True(t, activity.Executed, "Activity should have executed")
}

// TestOrchestrator_DuplicateActivityDetection tests that duplicate activities are rejected
func TestOrchestrator_DuplicateActivityDetection(t *testing.T) {
	orchestrator := NewOrchestrator()

	// Add first activity
	activity1 := &BasicActivity{ShouldFail: false}
	err := orchestrator.AddActivity(activity1)
	require.NoError(t, err, "First activity should be added successfully")

	// Try to add another activity of the same type
	activity2 := &BasicActivity{ShouldFail: true}
	err = orchestrator.AddActivity(activity2)
	require.Error(t, err, "Second activity of same type should be rejected")
	assert.Contains(t, err.Error(), "already exists", "Error should indicate duplicate")
}

// Helper Types for the tests

// Test config structure
type TestConfig struct {
	Database struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"database"`
	Service struct {
		Name    string `yaml:"name"`
		Timeout int    `yaml:"timeout"`
	} `yaml:"service"`
}

// Mock type for testing service injection
type MockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (m *MockLogger) Log(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
}

func (m *MockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid race conditions when reading
	result := make([]string, len(m.messages))
	copy(result, m.messages)
	return result
}
