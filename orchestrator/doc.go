// Package orchestrator provides a sophisticated activity orchestration system
// with dependency injection, concurrent execution, and comprehensive error handling.
//
// The orchestrator enables workflows by managing dependencies between
// activities, injecting required services and configuration, and executing
// activities in the correct order while respecting dependency constraints.
//
// # Core Concepts
//
// An Activity represents a single unit of work that can be executed within
// the orchestration system. Activities can depend on other activities,
// configuration values, and injected services.
//
// The Orchestrator manages the lifecycle of activities:
//   - Dependency resolution and injection
//   - Concurrent execution with proper ordering
//   - Error propagation and handling
//   - Result collection and access
//
// # Activity Identification
//
// Activities are identified using an ActivityID to prevent
// naming collisions in modular applications:
//
//	type ActivityID struct {
//		Module string  // Full import path: "github.com/user/project/activities"
//		Type   string  // Struct name: "PowerOnPBS"
//	}
//
// This ensures that activities with identical struct names from different
// packages remain uniquely identifiable:
//
//	// These are completely distinct activities:
//	ActivityID{Module: "github.com/user/app/activities", Type: "BackupTask"}
//	ActivityID{Module: "github.com/vendor/lib/activities", Type: "BackupTask"}
//
// # Activity Dependencies
//
// Activities can declare dependencies in three ways:
//
//  1. Activity Dependencies: Activities can depend on other activities by
//     declaring pointer fields of the activity type:
//
//     type ThirdActivity struct {
//     First *FirstActivity  // Depends on FirstActivity
//     _     *SecondActivity // Depends on SecondActvity
//     }
//
// The orchestrator detects these dependencies and ensures proper execution ordering.
//
//  2. Configuration Injection: Activities can receive configuration values
//     using struct tags:
//
//     type MyActivity struct {
//     DBHost string `config:"database.host"`
//     Port   int    `config:"database.port"`
//     }
//
// Configuration paths use dot notation to navigate nested structures and
// support multiple field name matching strategies including YAML tags.
//
// 3. Service Injection: Activities can receive injected services and clients:
//
//	type MyActivity struct {
//		Logger  *slog.Logger
//	}
//
// Services are matched by exact type and must be registered via Inject()
// before execution.
//
// # Execution Model
//
// The orchestrator executes activities concurrently while respecting dependency
// constraints. Activities with no dependencies start immediately, while dependent
// activities wait for their dependencies to complete successfully.
//
// Each activity runs in its own goroutine, enabling maximum parallelism while
// maintaining correctness. The execution order is determined by the dependency
// graph, not the order in which activities are added to the orchestrator.
//
// # Result Access Patterns
//
// The orchestrator provides multiple ways to access activity results, addressing
// the common problem of result utilization in complex workflows:
//
// Pattern 1 - Access by Activity Reference:
//
//	powerOnPBS := &activities.PowerOnPBS{}
//	o.AddActivity(powerOnPBS)
//	o.Execute(ctx)
//
//	if result, ok := o.GetResultByActivity(powerOnPBS); ok {
//		if result.IsSuccess() {
//			logger.Info("PBS powered on successfully")
//		}
//	}
//
// Pattern 2 - Access by ActivityID:
//
//	id := ActivityID{
//		Module: "github.com/user/project/activities",
//		Type:   "PowerOnPBS",
//	}
//	if result, ok := o.GetResult(id); ok {
//		// Process result
//	}
//
// Pattern 3 - Batch Access by Module:
//
//	results := o.GetResultsByModule("github.com/user/project/activities")
//	for id, result := range results {
//		logger.Info("activity result",
//			"activity", id.String(),
//			"success", result.IsSuccess())
//	}
//
// # Error Handling
//
// The orchestrator provides error handling:
//   - Dependency cycle detection before execution
//   - Validation of all dependencies and configuration
//   - Context cancellation support
//   - Failure propagation (dependent activities don't run if dependencies fail)
//   - Logging and debugging information
//
// # Practical Usage Example
//
// Complete example demonstrating best practices for result utilization:
//
//	// Define your activities
//	type DatabaseSetup struct {
//		ConnectionString string `config:"database.connection"`
//		Logger          *slog.Logger
//	}
//
//	func (d *DatabaseSetup) Init() error { return nil }
//	func (d *DatabaseSetup) Run(ctx context.Context) (Result, error) {
//		// Setup database
//		d.Logger.Info("setting up database")
//		return NewSuccessResult(), nil
//	}
//
//	type DataMigration struct {
//		Setup  *DatabaseSetup  // Depends on DatabaseSetup
//		Logger *slog.Logger
//	}
//
//	func (d *DataMigration) Init() error { return nil }
//	func (d *DataMigration) Run(ctx context.Context) (Result, error) {
//		// Run migrations
//		d.Logger.Info("running migrations")
//		return NewSuccessResult(), nil
//	}
//
//	// Configure and execute with result utilization
//	func main() {
//		config := &Config{Database: DatabaseConfig{Connection: "postgres://..."}}
//		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
//
//		// Keep references to activities for result access
//		dbSetup := &DatabaseSetup{}
//		dataMigration := &DataMigration{}
//
//		orchestrator := NewOrchestrator(config)
//		orchestrator.Inject(logger)
//		orchestrator.AddActivity(dbSetup, dataMigration)
//
//		if err := orchestrator.Execute(ctx); err != nil {
//			log.Fatal(err)
//		}
//
//		// Access individual results for decision making
//		if result, ok := orchestrator.GetResultByActivity(dbSetup); ok {
//			if result.IsSuccess() {
//				logger.Info("database setup completed successfully")
//			} else {
//				logger.Error("database setup failed")
//				return // Don't proceed with dependent operations
//			}
//		}
//
//		if result, ok := orchestrator.GetResultByActivity(dataMigration); ok {
//			if result.IsSuccess() {
//				logger.Info("migrations completed successfully")
//				// Emit success metrics, trigger notifications, etc.
//			}
//		}
//
//		// Example: Conditional logic based on results
//		allResults := orchestrator.GetAllResults()
//		successCount := 0
//		for _, result := range allResults {
//			if result.IsSuccess() {
//				successCount++
//			}
//		}
//
//		if successCount == len(allResults) {
//			logger.Info("all activities completed successfully")
//			// Trigger cleanup, shutdown resources, etc.
//		} else {
//			logger.Warn("some activities failed", "success_count", successCount, "total", len(allResults))
//			// Handle partial failure scenario
//		}
//	}
//
// # Avoiding Common Pitfalls
//
// 1. Result Access Timing: Always access results after Execute() completes
// 2. Error Handling: Check both Execute() errors and individual activity results
//
// # Advanced Features
//
// Custom Logging: Provide custom loggers for detailed observability:
//
//	orchestrator := NewOrchestrator(config, WithLogger(customLogger))
//
// # Thread Safety
//
// The orchestrator is thread-safe. Multiple goroutines can safely call
// result access methods while execution is in progress. However, activities
// should not be added after Execute() has been called.
//
// Safe operations during execution:
//   - GetResult(), GetResultByActivity(), GetAllResults()
//   - All result access methods
//   - Logging operations
//
// Unsafe operations during execution:
//   - AddActivity()
//   - Inject()
//   - Concurrent Execute() calls
//
// # Debugging and Observability
//
// The orchestrator provides extensive structured logging for debugging:
//   - Dependency graph construction details
//   - Activity execution order and timing
//   - Error details with full context
//   - ActivityID information in all log messages
package orchestrator
