// Package orchestrator provides dependency-resolved execution of activities with comprehensive result tracking.
//
// # Overview
//
// The orchestrator manages the execution of activities with automatic dependency resolution,
// configuration injection, and detailed result tracking. Activities are executed in dependency
// order with proper error handling and state management.
//
// # Core Guarantees
//
// Result Availability: Results are available immediately after AddActivity() in NotStarted state
// and persist with final state after Execute() completes. All result access methods are thread-safe.
//
// Dependency Management: Activities with pointer fields to other activity types automatically
// get dependencies injected. Both named and unnamed dependency patterns are supported for
// flexible ordering and access control.
//
// Error Isolation: Individual activity failures don't stop execution of other activities.
// However, if an activity's Execute() method returns an error, all downstream activities
// that depend on it will be skipped.
//
// # Activity Contract
//
// Activities must implement the Activity interface:
//
//	type Activity interface {
//	    Init() error                       // Structural validation after injection
//	    Execute(ctx context.Context) error // Perform the work
//	}
//
// Init() is for "fail-fast" structural validation called after ALL injection is complete.
// At Init() time, all dependencies are injected but NO activities have executed yet.
//
// Init() validates STRUCTURE AND CONFIGURATION:
// - Required configuration fields are set and valid
// - Required dependencies are injected (not nil)
// - Static relationships between configuration values
// - Configuration ranges, formats, and business rules
//
// Init() should NOT depend on runtime state of dependencies.
// Execute() performs the actual work and handles runtime validation.
//
// # Dependency Patterns
//
// Named Dependencies (Access + Ordering):
//
//	type DataMigrationActivity struct {
//	    Database *DatabaseSetupActivity // Available for nil-check in Init()
//	}
//
// Unnamed Dependencies (Ordering Only):
//
//	type CleanupActivity struct {
//	    Migration *DataMigrationActivity  // Named - can access
//	    _         *BackupServiceActivity  // Unnamed - ordering only
//	}
//
// # Configuration Injection
//
// Use struct tags for configuration injection:
//
//	type DatabaseMigrationActivity struct {
//	    DBHost   string `config:"database.host"`
//	    DBPort   int    `config:"database.port"`
//	    Logger   *Logger
//	    Database *DatabaseSetupActivity
//	}
//
//	func (a *DatabaseMigrationActivity) Init() error {
//	    // GOOD: Validate configuration
//	    if a.DBHost == "" {
//	        return fmt.Errorf("database host required")
//	    }
//	    if a.DBPort < 1 || a.DBPort > 65535 {
//	        return fmt.Errorf("invalid port: %d", a.DBPort)
//	    }
//	    // GOOD: Validate dependencies are injected
//	    if a.Logger == nil {
//	        return fmt.Errorf("logger required")
//	    }
//	    if a.Database == nil {
//	        return fmt.Errorf("database dependency required")
//	    }
//	    // BAD: Don't check if Database executed - it hasn't yet!
//	    // if !a.Database.Executed { return fmt.Errorf("...") }
//	    return nil
//	}
//
//	func (a *DatabaseMigrationActivity) Execute(ctx context.Context) error {
//	    // GOOD: Runtime validation happens here
//	    if !a.Database.IsReady() {
//	        return fmt.Errorf("database not ready")
//	    }
//	    // Perform actual work...
//	    return nil
//	}
//
// Configuration supports dot notation for nested values and handles YAML tag matching.
//
// # State Progression
//
// Activities progress through states in a defined order:
//
//	NotStarted -> Pending -> Running -> (Completed|Skipped)
//
// Final states after Execute() depend on what occurred:
//
//	NotStarted: Activity was not executed (validation/circular dependency issues)
//	Skipped:    Activity was prevented from running (dependency failed, context cancelled)
//	Completed:  Activity's Execute() method was called (check Error for success/failure)
//
// The Result.Error field contains ONLY errors returned by the activity's Execute() method.
// Validation errors, dependency failures, and cancellations are reflected in State only.
//
// # Usage Example
//
//	// Create activities
//	dbSetup := &DatabaseSetupActivity{}
//	migration := &DataMigrationActivity{}
//	cleanup := &CleanupActivity{}
//
//	// Create orchestrator with config
//	config := &AppConfig{Database: DatabaseConfig{Host: "localhost"}}
//	logger := &Logger{}
//
//	orchestrator := NewOrchestrator(WithConfig(config))
//	orchestrator.Inject(logger)
//
//	// Add activities (results immediately available)
//	err := orchestrator.AddActivity(dbSetup, migration, cleanup)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Execute with dependency resolution
//	ctx := context.Background()
//	err = orchestrator.Execute(ctx)
//
//	// Check individual results
//	results := orchestrator.GetAllResults()
//	dbSetupID := GetActivityID(dbSetup)
//	if result := results[dbSetupID]; !result.IsSuccess() {
//	    log.Printf("Database setup failed: %v", result.Error)
//	}
//
// # Error Handling
//
// The orchestrator separates structural/validation errors from activity execution errors:
//
// Structural Errors: Configuration errors, missing dependencies, circular dependencies,
// and context cancellation prevent activities from executing. These are reflected in the
// activity State (NotStarted or Skipped) but do NOT set the Result.Error field.
//
// Execution Errors: Only errors returned by an activity's Execute() method are stored
// in Result.Error. These activities will have State == Completed with a non-nil Error.
//
// Isolation: Individual activity execution failures don't stop other activities from
// running. However, if an activity's Execute() method returns an error, all activities
// that depend on it will be skipped (marked as Skipped state with nil Error).
//
// # Thread Safety
//
// All orchestrator methods are thread-safe. Activities can safely call result methods
// during execution. Result objects are immutable once set and GetAllResults() returns
// a copy to prevent external modification.
//
// # Best Practices
//
// Init() vs Execute() Validation:
// - Init(): Validate structure, configuration, and dependency injection (fail-fast)
// - Execute(): Validate runtime state and perform the actual work
//
// Use named dependencies when you need to access the dependency in your activity.
// Use unnamed dependencies (_) when you only need ordering constraints.
// Validate early in Init() to fail fast for configuration issues.
// Handle context cancellation gracefully in Execute().
package workflow
