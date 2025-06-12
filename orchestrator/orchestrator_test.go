package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test config structure
type TestConfig struct {
	Database struct {
		Host string
		Port int
	}
	API struct {
		Timeout string
	}
}

// Shared counter to verify execution order
var executionCounter int64

// FirstActivity - no dependencies (foundation activity)
type FirstActivity struct {
	DBHost string `config:"database.host"`
	DBPort int    `config:"database.port"`

	initialized    bool
	executed       bool
	executionOrder int64
}

func (a *FirstActivity) Init() error {
	if a.DBHost == "" {
		return fmt.Errorf("database host not configured")
	}
	a.initialized = true
	return nil
}

func (a *FirstActivity) Run(ctx context.Context) (Result, error) {
	if !a.initialized {
		return NewFailureResult(), fmt.Errorf("not initialized")
	}

	// Simulate some work
	time.Sleep(100 * time.Millisecond)

	a.executionOrder = atomic.AddInt64(&executionCounter, 1)
	a.executed = true
	return NewSuccessResult(), nil
}

// SecondActivity - depends on FirstActivity
type SecondActivity struct {
	APITimeout string `config:"api.timeout"`
	First      *FirstActivity

	initialized    bool
	executed       bool
	executionOrder int64
}

func (a *SecondActivity) Init() error {
	if a.First == nil {
		return fmt.Errorf("first activity dependency not injected")
	}
	a.initialized = true
	return nil
}

func (a *SecondActivity) Run(ctx context.Context) (Result, error) {
	if !a.First.executed {
		return NewFailureResult(), fmt.Errorf("first activity not executed")
	}

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	a.executionOrder = atomic.AddInt64(&executionCounter, 1)
	a.executed = true
	return NewSuccessResult(), nil
}

// ThirdActivity - also depends on FirstActivity (can run parallel to SecondActivity)
type ThirdActivity struct {
	First *FirstActivity

	initialized    bool
	executed       bool
	executionOrder int64
}

func (a *ThirdActivity) Init() error {
	if a.First == nil {
		return fmt.Errorf("first activity dependency not injected")
	}
	a.initialized = true
	return nil
}

func (a *ThirdActivity) Run(ctx context.Context) (Result, error) {
	if !a.First.executed {
		return NewFailureResult(), fmt.Errorf("first activity not executed")
	}

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	a.executionOrder = atomic.AddInt64(&executionCounter, 1)
	a.executed = true
	return NewSuccessResult(), nil
}

// FourthActivity - depends on both SecondActivity and ThirdActivity
type FourthActivity struct {
	Second *SecondActivity
	Third  *ThirdActivity

	initialized    bool
	executed       bool
	executionOrder int64
}

func (a *FourthActivity) Init() error {
	if a.Second == nil || a.Third == nil {
		return fmt.Errorf("activity dependencies not injected")
	}
	a.initialized = true
	return nil
}

func (a *FourthActivity) Run(ctx context.Context) (Result, error) {
	if !a.Second.executed || !a.Third.executed {
		return NewFailureResult(), fmt.Errorf("dependent activities not executed")
	}

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	a.executionOrder = atomic.AddInt64(&executionCounter, 1)
	a.executed = true
	return NewSuccessResult(), nil
}

func TestOrchestrator_ConcurrentExecution(t *testing.T) {
	// Reset execution counter
	atomic.StoreInt64(&executionCounter, 0)

	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432
	config.API.Timeout = "30s"

	// Create activities
	first := &FirstActivity{}
	second := &SecondActivity{}
	third := &ThirdActivity{}
	fourth := &FourthActivity{}

	// Create orchestrator and add activities in random order to prove order doesn't matter
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(fourth, second, first, third) // Intentionally mixed up order

	// Execute
	ctx := context.Background()
	start := time.Now()
	err := orchestrator.Execute(ctx)
	duration := time.Since(start)

	// Verify success
	require.NoError(t, err, "Expected no error during execution")

	// Verify all activities executed
	assert.True(t, first.executed, "FirstActivity should have executed")
	assert.True(t, second.executed, "SecondActivity should have executed")
	assert.True(t, third.executed, "ThirdActivity should have executed")
	assert.True(t, fourth.executed, "FourthActivity should have executed")

	// Verify execution order constraints
	assert.NotZero(t, first.executionOrder, "FirstActivity should have executed")

	// First should execute before Second and Third
	assert.Greater(t, second.executionOrder, first.executionOrder, "SecondActivity should execute after FirstActivity")
	assert.Greater(t, third.executionOrder, first.executionOrder, "ThirdActivity should execute after FirstActivity")

	// Fourth should execute after both Second and Third
	assert.Greater(t, fourth.executionOrder, second.executionOrder, "FourthActivity should execute after SecondActivity")
	assert.Greater(t, fourth.executionOrder, third.executionOrder, "FourthActivity should execute after ThirdActivity")

	// Verify execution time
	assert.Less(t, duration, 300*time.Millisecond, "Execution should take less than 300ms due to parallel execution")
}

func TestOrchestrator_FailurePropagation(t *testing.T) {
	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create activities
	first := &FirstActivity{}
	second := &SecondActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, second)

	// Execute with a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := orchestrator.Execute(ctx)
	require.Error(t, err, "Expected error due to cancelled context")
	assert.Contains(t, err.Error(), "context canceled", "Error should indicate context cancellation")

	// Verify activities were not executed
	assert.False(t, first.executed, "FirstActivity should not have executed")
	assert.False(t, second.executed, "SecondActivity should not have executed")
}

// CircularDependencyActivity creates a circular dependency for testing
type CircularDependencyActivity struct {
	First *FirstActivity
}

func (a *CircularDependencyActivity) Init() error { return nil }
func (a *CircularDependencyActivity) Run(ctx context.Context) (Result, error) {
	return NewSuccessResult(), nil
}

// ModifiedFirstActivity that depends on CircularDependencyActivity to create a cycle
type ModifiedFirstActivity struct {
	DBHost   string `config:"database.host"`
	Circular *CircularDependencyActivity
}

func (a *ModifiedFirstActivity) Init() error { return nil }
func (a *ModifiedFirstActivity) Run(ctx context.Context) (Result, error) {
	return NewSuccessResult(), nil
}

func TestOrchestrator_CircularDependencyDetection(t *testing.T) {
	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create activities with circular dependency
	first := &ModifiedFirstActivity{}
	circular := &CircularDependencyActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, circular)

	// Execute
	err := orchestrator.Execute(context.Background())
	require.Error(t, err, "Expected error due to circular dependency")
	assert.Contains(t, err.Error(), "circular dependency", "Error should indicate circular dependency")
}

// BadActivity has a struct dependency instead of pointer - should fail validation
type BadActivity struct {
	First FirstActivity // This should be *FirstActivity
}

func (a *BadActivity) Init() error { return nil }
func (a *BadActivity) Run(ctx context.Context) (Result, error) {
	return NewSuccessResult(), nil
}

func TestOrchestrator_BadDependencyValidation(t *testing.T) {
	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create activity with bad dependency type
	bad := &BadActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(bad)

	// Execute
	err := orchestrator.Execute(context.Background())
	require.Error(t, err, "Expected error due to bad dependency type")
	assert.Contains(t, err.Error(), "invalid dependency type", "Error should indicate invalid dependency type")
}

func TestOrchestrator_ContextCancellation(t *testing.T) {
	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create activities
	first := &FirstActivity{}
	second := &SecondActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, second)

	// Execute with a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := orchestrator.Execute(ctx)
	require.Error(t, err, "Expected error due to cancelled context")
	assert.Contains(t, err.Error(), "context canceled", "Error should indicate context cancellation")

	// Verify activities were not all executed
	assert.True(t, first.executed, "FirstActivity should have executed")
	assert.False(t, second.executed, "SecondActivity should not have executed due to cancellation")
}

// Mock types for testing type injection
type MockLogger struct {
	messages []string
}

func (m *MockLogger) Log(message string) {
	m.messages = append(m.messages, message)
}

type MockMetricsClient struct {
	metrics map[string]float64
}

func (m *MockMetricsClient) Record(name string, value float64) {
	if m.metrics == nil {
		m.metrics = make(map[string]float64)
	}
	m.metrics[name] = value
}

// ActivityWithInjectedTypes tests type injection
type ActivityWithInjectedTypes struct {
	Logger        *MockLogger       // Pointer to injected type
	MetricsClient MockMetricsClient // Direct injected type
	Slogger       *slog.Logger      // Standard library type

	executed bool
}

func (a *ActivityWithInjectedTypes) Init() error {
	if a.Logger == nil {
		return fmt.Errorf("logger not injected")
	}
	if a.Slogger == nil {
		return fmt.Errorf("slogger not injected")
	}
	return nil
}

func (a *ActivityWithInjectedTypes) Run(ctx context.Context) (Result, error) {
	a.Logger.Log("Activity executed")
	a.MetricsClient.Record("execution_count", 1.0)
	a.Slogger.Info("Activity running")
	a.executed = true
	return NewSuccessResult(), nil
}

func TestOrchestrator_TypeInjection(t *testing.T) {
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create mock logger and metrics client
	logger := &MockLogger{}
	metrics := MockMetricsClient{metrics: make(map[string]float64)}
	slogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	activity := &ActivityWithInjectedTypes{}

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger, metrics, slogger)
	orchestrator.AddActivity(activity)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Expected no error during execution")
	assert.True(t, activity.executed, "Activity should have executed")
	assert.Equal(t, logger, activity.Logger, "Logger should be injected")
	assert.Equal(t, metrics, activity.MetricsClient, "MetricsClient should be injected")
	assert.Equal(t, slogger, activity.Slogger, "Slogger should be injected")
}

func TestOrchestrator_InjectMultipleTypes(t *testing.T) {
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	logger := &MockLogger{}
	metrics := MockMetricsClient{metrics: make(map[string]float64)}

	activity := &ActivityWithInjectedTypes{}

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger)
	orchestrator.Inject(metrics)
	orchestrator.AddActivity(activity)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Expected no error during execution")
	assert.True(t, activity.executed, "Activity should have executed")
	assert.Equal(t, logger, activity.Logger, "Logger should be injected")
	assert.Equal(t, metrics, activity.MetricsClient, "MetricsClient should be injected")
}

// ActivityWithMixedDependencies tests both activity and type injection
type ActivityWithMixedDependencies struct {
	First         *FirstActivity    // Activity dependency
	Logger        *MockLogger       // Type dependency
	MetricsClient MockMetricsClient // Type dependency

	executed bool
}

func (a *ActivityWithMixedDependencies) Init() error {
	if a.First == nil {
		return fmt.Errorf("first activity not injected")
	}
	if a.Logger == nil {
		return fmt.Errorf("logger not injected")
	}
	return nil
}

func (a *ActivityWithMixedDependencies) Run(ctx context.Context) (Result, error) {
	if !a.First.executed {
		return NewFailureResult(), fmt.Errorf("first activity not executed")
	}

	a.Logger.Log("Mixed dependencies activity executed")
	a.MetricsClient.Record("mixed_execution", 1.0)
	a.executed = true
	return NewSuccessResult(), nil
}

func TestOrchestrator_MixedDependencies(t *testing.T) {
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	first := &FirstActivity{}
	logger := &MockLogger{}
	metrics := MockMetricsClient{metrics: make(map[string]float64)}
	activity := &ActivityWithMixedDependencies{}

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger, metrics)
	orchestrator.AddActivity(first, activity)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Expected no error during execution")
	assert.True(t, activity.executed, "Activity should have executed")
	assert.Equal(t, first, activity.First, "FirstActivity should be injected")
	assert.Equal(t, logger, activity.Logger, "Logger should be injected")
	assert.Equal(t, metrics, activity.MetricsClient, "MetricsClient should be injected")
}

func TestOrchestrator_InjectNilDependency(t *testing.T) {
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	logger := (*MockLogger)(nil)
	metrics := (*MockMetricsClient)(nil)
	activity := &ActivityWithInjectedTypes{}

	orchestrator := NewOrchestrator(config)
	orchestrator.Inject(logger, metrics)
	orchestrator.AddActivity(activity)

	err := orchestrator.Execute(context.Background())
	require.NoError(t, err, "Expected no error during execution with nil dependencies")
	assert.True(t, activity.executed, "Activity should have executed")
	assert.Nil(t, activity.Logger, "Logger should be nil")
	assert.Equal(t, MockMetricsClient{}, activity.MetricsClient, "MetricsClient should be zero value")
}
