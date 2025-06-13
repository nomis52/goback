package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// Mock types for testing service injection
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

// Test activities with various dependency patterns

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

func (a *DatabaseSetupActivity) Run(ctx context.Context) (Result, error) {
	a.Logger.Log("Setting up database")
	a.Executed = true
	return NewSuccessResult(), nil
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

func (a *DataMigrationActivity) Run(ctx context.Context) (Result, error) {
	if !a.Setup.Executed {
		return NewFailureResult(), fmt.Errorf("database setup not executed")
	}
	a.Logger.Log("Running data migration")
	a.Executed = true
	return NewSuccessResult(), nil
}

// BackupServiceActivity - Can run parallel to DataMigrationActivity
type BackupServiceActivity struct {
	Setup          *DatabaseSetupActivity // Activity dependency
	ServiceName    string                 `config:"service.name"`
	Executed       bool
	ExecutionOrder int
}

func (a *BackupServiceActivity) Init() error { return nil }

func (a *BackupServiceActivity) Run(ctx context.Context) (Result, error) {
	a.Executed = true
	return NewSuccessResult(), nil
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

func (a *CleanupTaskActivity) Run(ctx context.Context) (Result, error) {
	// Can access Migration but not the BackupServiceActivity (unnamed dep)
	// The unnamed dependency still ensures BackupServiceActivity runs first
	if a.Migration != nil && !a.Migration.Executed {
		return NewFailureResult(), fmt.Errorf("migration dependency not satisfied")
	}
	a.Executed = true
	return NewSuccessResult(), nil
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
	ExecutionOrder int
}

func (a *AdvancedOrderingActivity) Init() error { return nil }

func (a *AdvancedOrderingActivity) Run(ctx context.Context) (Result, error) {
	// We can access Database but not the unnamed dependencies
	// This is perfect for "run after these complete" scenarios
	if a.Database != nil && !a.Database.Executed {
		return NewFailureResult(), fmt.Errorf("database not ready")
	}
	a.Executed = true
	return NewSuccessResult(), nil
}

// FailingActivity - For testing failure scenarios
type FailingActivity struct {
	ShouldFail bool
	Executed   bool
}

func (a *FailingActivity) Init() error { return nil }

func (a *FailingActivity) Run(ctx context.Context) (Result, error) {
	a.Executed = true
	if a.ShouldFail {
		return NewFailureResult(), fmt.Errorf("intentional failure")
	}
	return NewSuccessResult(), nil
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
	orchestrator := NewOrchestrator(config, WithLogger(slogLogger))

	// Test service injection
	orchestrator.Inject(logger)

	// Test activity registration in random order
	orchestrator.AddActivity(cleanupTask, dataMigration)
	orchestrator.AddActivity(dbSetup, backupService)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

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
		result, ok := orchestrator.GetResultByActivity(dbSetup)
		require.True(t, ok, "Should find result for DatabaseSetupActivity")
		assert.True(t, result.IsSuccess(), "DatabaseSetupActivity should have succeeded")

		result, ok = orchestrator.GetResultByActivity(dataMigration)
		require.True(t, ok, "Should find result for DataMigrationActivity")
		assert.True(t, result.IsSuccess(), "DataMigrationActivity should have succeeded")

		// Pattern 2: Access by ActivityID
		dbSetupID := ActivityID{
			Module: "github.com/nomis52/goback/orchestrator", // This test package
			Type:   "DatabaseSetupActivity",
		}
		result, ok = orchestrator.GetResult(dbSetupID)
		require.True(t, ok, "Should find result by ActivityID")
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

		// Pattern 4: Get results by module
		moduleResults := orchestrator.GetResultsByModule("github.com/nomis52/goback/orchestrator")
		assert.Len(t, moduleResults, 4, "Should have all results from this test module")
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

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger)

	// Add activities in random order to test dependency resolution
	orchestrator.AddActivity(advancedOrdering, cleanupTask, backupService)
	orchestrator.AddActivity(dataMigration, dbSetup)

	err := orchestrator.Execute(context.Background())
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
		failingActivity := &FailingActivity{ShouldFail: true}

		orchestrator := NewOrchestrator(config)
		orchestrator.AddActivity(failingActivity)

		err := orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail when activity fails")

		// Check that failing activity result is available
		result, ok := orchestrator.GetResultByActivity(failingActivity)
		require.True(t, ok, "Should have result for failing activity")
		assert.False(t, result.IsSuccess(), "Failing activity should report failure")
	})

	t.Run("ConfigurationError", func(t *testing.T) {
		// Empty config should cause validation error
		emptyConfig := &TestConfig{}
		dbSetup := &DatabaseSetupActivity{}

		orchestrator := NewOrchestrator(emptyConfig)
		orchestrator.AddActivity(dbSetup)

		err := orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail with empty database host")
		assert.Contains(t, err.Error(), "database host not configured")
	})

	t.Run("MissingDependency", func(t *testing.T) {
		// Activity without required service injection
		dbSetup := &DatabaseSetupActivity{}

		orchestrator := NewOrchestrator(config)
		// Don't inject logger - should cause validation error
		orchestrator.AddActivity(dbSetup)

		err := orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail with missing logger dependency")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		dbSetup := &DatabaseSetupActivity{}
		logger := &MockLogger{}

		orchestrator := NewOrchestrator(config)
		orchestrator.Inject(logger)
		orchestrator.AddActivity(dbSetup)

		// Cancel context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := orchestrator.Execute(ctx)
		require.Error(t, err, "Should fail with cancelled context")
		assert.Contains(t, err.Error(), "cancelled", "Error should indicate cancellation")
	})

	t.Run("UnnamedDependencyFailure", func(t *testing.T) {
		// Test that unnamed dependency failures properly prevent dependent activities
		logger := &MockLogger{}
		dbSetup := &DatabaseSetupActivity{}
		failingDep := &FailingActivity{ShouldFail: true}
		
		// Activity that depends on the failing activity via unnamed dependency
		type DependentActivity struct {
			_ *FailingActivity // Unnamed dependency on failing activity
			Executed bool
		}
		dependent := &struct{ DependentActivity }{}
		dependent.Init = func() error { return nil }
		dependent.Run = func(ctx context.Context) (Result, error) {
			dependent.Executed = true
			return NewSuccessResult(), nil
		}

		orchestrator := NewOrchestrator(config)
		orchestrator.Inject(logger)
		orchestrator.AddActivity(dbSetup, failingDep, dependent)

		err := orchestrator.Execute(context.Background())
		require.Error(t, err, "Should fail due to failing dependency")

		// Verify dependent activity didn't execute due to failed unnamed dependency
		assert.False(t, dependent.Executed, "Dependent activity should not execute when unnamed dependency fails")
	})
}

// TestOrchestrator_CircularDependencyDetection tests circular dependency detection
func TestOrchestrator_CircularDependencyDetection(t *testing.T) {
	// Create activities with circular dependency
	type FirstCircularActivity struct {
		Second *SecondCircularActivity
	}
	type SecondCircularActivity struct {
		First *FirstCircularActivity
	}

	// Implement interfaces
	firstActivity := &struct {
		FirstCircularActivity
	}{}
	secondActivity := &struct {
		SecondCircularActivity
	}{}

	firstActivity.Init = func() error { return nil }
	firstActivity.Run = func(ctx context.Context) (Result, error) { return NewSuccessResult(), nil }
	secondActivity.Init = func() error { return nil }
	secondActivity.Run = func(ctx context.Context) (Result, error) { return NewSuccessResult(), nil }

	config := &TestConfig{}
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(firstActivity, secondActivity)

	err := orchestrator.Execute(context.Background())
	require.Error(t, err, "Should detect circular dependency")
	assert.Contains(t, err.Error(), "circular dependency", "Error should indicate circular dependency")
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
	successActivity := &FailingActivity{ShouldFail: false}
	failActivity := &FailingActivity{ShouldFail: true}

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger)
	orchestrator.AddActivity(dbSetup, successActivity, failActivity)

	// Execute (will partially fail)
	err := orchestrator.Execute(context.Background())
	require.Error(t, err, "Should fail due to failing activity")

	t.Run("ConditionalLogicBasedOnResults", func(t *testing.T) {
		// Example: PBS automation logic
		dbResult, _ := orchestrator.GetResultByActivity(dbSetup)
		successResult, _ := orchestrator.GetResultByActivity(successActivity)
		failResult, _ := orchestrator.GetResultByActivity(failActivity)

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

	t.Run("ModuleLevelAnalysis", func(t *testing.T) {
		moduleResults := orchestrator.GetResultsByModule("github.com/nomis52/goback/orchestrator")
		assert.Len(t, moduleResults, 3, "Should have 3 activities from test module")

		// Check if all critical activities from this module succeeded
		criticalActivitiesSucceeded := true
		for id, result := range moduleResults {
			if id.Type == "DatabaseSetupActivity" && !result.IsSuccess() {
				criticalActivitiesSucceeded = false
			}
		}

		assert.True(t, criticalActivitiesSucceeded, "Critical activities should succeed")
	})
}

// TestOrchestrator_NoActivities tests orchestrator with no activities
func TestOrchestrator_NoActivities(t *testing.T) {
	config := &TestConfig{}
	orchestrator := NewOrchestrator(config)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Should handle no activities gracefully")

	allResults := orchestrator.GetAllResults()
	assert.Empty(t, allResults, "Should have no results")
}

// TestOrchestrator_CustomLogger tests custom logger functionality
func TestOrchestrator_CustomLogger(t *testing.T) {
	config := &TestConfig{
		Database: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
		},
	}

	// Create custom logger that captures output
	customLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	dbSetup := &DatabaseSetupActivity{}
	logger := &MockLogger{}

	orchestrator := NewOrchestrator(config, WithLogger(customLogger))
	orchestrator.Inject(logger)
	orchestrator.AddActivity(dbSetup)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Should execute with custom logger")

	// Verify activity executed
	assert.True(t, dbSetup.Executed, "Activity should have executed")
}


