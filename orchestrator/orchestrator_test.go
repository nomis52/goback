package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"
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

// FirstActivity - no dependencies
type FirstActivity struct {
	DBHost string `config:"database.host"`
	DBPort int    `config:"database.port"`
	
	initialized bool
	executed    bool
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
	a.executed = true
	return NewSuccessResult(), nil
}

// SecondActivity - depends on FirstActivity
type SecondActivity struct {
	APITimeout string `config:"api.timeout"`
	First      *FirstActivity
	
	initialized bool
	executed    bool
}

func (a *SecondActivity) Init() error {
	if a.First == nil || !a.First.initialized {
		return fmt.Errorf("first activity not initialized")
	}
	a.initialized = true
	return nil
}

func (a *SecondActivity) Run(ctx context.Context) (Result, error) {
	if !a.First.executed {
		return NewFailureResult(), fmt.Errorf("first activity not executed")
	}
	a.executed = true
	return NewSuccessResult(), nil
}

// ThirdActivity - depends on SecondActivity
type ThirdActivity struct {
	Second *SecondActivity
	
	initialized bool
	executed    bool
}

func (a *ThirdActivity) Init() error {
	if a.Second == nil || !a.Second.initialized {
		return fmt.Errorf("second activity not initialized")
	}
	a.initialized = true
	return nil
}

func (a *ThirdActivity) Run(ctx context.Context) (Result, error) {
	if !a.Second.executed {
		return NewFailureResult(), fmt.Errorf("second activity not executed")
	}
	a.executed = true
	return NewSuccessResult(), nil
}

func TestOrchestrator_Execute(t *testing.T) {
	// Create test config
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432
	config.API.Timeout = "30s"

	// Create activities
	first := &FirstActivity{}
	second := &SecondActivity{}
	third := &ThirdActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, second, third)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	// Verify success
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config injection
	if first.DBHost != "localhost" {
		t.Errorf("Expected DBHost 'localhost', got '%s'", first.DBHost)
	}
	if first.DBPort != 5432 {
		t.Errorf("Expected DBPort 5432, got %d", first.DBPort)
	}
	if second.APITimeout != "30s" {
		t.Errorf("Expected APITimeout '30s', got '%s'", second.APITimeout)
	}

	// Verify dependency injection
	if second.First != first {
		t.Error("FirstActivity not properly injected into SecondActivity")
	}
	if third.Second != second {
		t.Error("SecondActivity not properly injected into ThirdActivity")
	}

	// Verify execution order
	if !first.executed {
		t.Error("FirstActivity was not executed")
	}
	if !second.executed {
		t.Error("SecondActivity was not executed")
	}
	if !third.executed {
		t.Error("ThirdActivity was not executed")
	}

	// Verify all activities were initialized
	if !first.initialized {
		t.Error("FirstActivity was not initialized")
	}
	if !second.initialized {
		t.Error("SecondActivity was not initialized")
	}
	if !third.initialized {
		t.Error("ThirdActivity was not initialized")
	}
}

func TestOrchestrator_ExecuteWithFailure(t *testing.T) {
	// Create test config with missing required field
	config := &TestConfig{}
	// Missing database.host to trigger failure

	// Create activities
	first := &FirstActivity{}
	second := &SecondActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, second)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	// Verify failure
	if err == nil {
		t.Fatal("Expected error due to missing config, got none")
	}

	// Should fail during Init phase
	if first.executed {
		t.Error("FirstActivity should not have been executed due to init failure")
	}
	if second.executed {
		t.Error("SecondActivity should not have been executed due to init failure")
	}
}

// BadActivity has a struct dependency instead of pointer - should fail validation
type BadActivity struct {
	First FirstActivity // This should be *FirstActivity
}

func (a *BadActivity) Init() error {
	return nil
}

func (a *BadActivity) Run(ctx context.Context) (Result, error) {
	return NewSuccessResult(), nil
}

func TestOrchestrator_ExecuteWithBadDependency(t *testing.T) {
	config := &TestConfig{}
	config.Database.Host = "localhost"
	config.Database.Port = 5432

	// Create activities with bad dependency
	first := &FirstActivity{}
	bad := &BadActivity{}

	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	orchestrator.AddActivity(first, bad)

	// Execute
	ctx := context.Background()
	err := orchestrator.Execute(ctx)

	// Verify failure due to struct dependency
	if err == nil {
		t.Fatal("Expected error due to struct dependency, got none")
	}

	// Should contain our specific error message
	expected := "activity dependency field First must be a pointer"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain '%s', got: %v", expected, err)
	}
}
