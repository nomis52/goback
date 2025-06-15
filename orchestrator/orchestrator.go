package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
)

// Orchestrator manages the execution of activities with optimized lookups.
type Orchestrator struct {
	config interface{}
	logger *slog.Logger

	// Dependency injection map: type -> instance
	injectedTypes map[reflect.Type]interface{}

	// Core data structures using ActivityID as primary key
	activityMap     map[ActivityID]Activity      // Primary storage for activities
	dependencyMap   map[ActivityID][]ActivityID  // activity ID -> list of dependency IDs
	completionChans map[ActivityID]chan struct{} // activity ID -> completion signal (closed when done)
	resultMap       map[ActivityID]*Result       // activity ID -> result (protected by mutex)

	// Performance optimization: cache ActivityID lookups to avoid repeated reflection
	activityIDCache map[Activity]ActivityID

	mu sync.RWMutex
}

// OrchestratorOption is a function that configures an Orchestrator
type OrchestratorOption func(*Orchestrator)

// WithLogger sets a custom logger for the orchestrator
func WithLogger(logger *slog.Logger) OrchestratorOption {
	return func(o *Orchestrator) {
		o.logger = logger.With("component", "orchestrator")
	}
}

// WithConfig sets the configuration for the orchestrator
func WithConfig(config interface{}) OrchestratorOption {
	return func(o *Orchestrator) {
		o.config = config
	}
}

// NewOrchestrator creates a new orchestrator instance with optional configuration
func NewOrchestrator(opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		logger:          slog.Default().With("component", "orchestrator"),
		injectedTypes:   make(map[reflect.Type]interface{}),
		activityMap:     make(map[ActivityID]Activity),
		dependencyMap:   make(map[ActivityID][]ActivityID),
		completionChans: make(map[ActivityID]chan struct{}),
		resultMap:       make(map[ActivityID]*Result),
		activityIDCache: make(map[Activity]ActivityID),
	}

	// Apply options
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Inject adds one or more typed dependencies to the orchestrator for injection into activities
func (o *Orchestrator) Inject(deps ...interface{}) error {
	for _, dep := range deps {
		if dep == nil {
			o.logger.Warn("attempted to inject nil dependency")
			continue
		}

		depType := reflect.TypeOf(dep)
		if _, exists := o.injectedTypes[depType]; exists {
			return fmt.Errorf("dependency type %s already injected", depType.String())
		}

		o.injectedTypes[depType] = dep
		o.logger.Debug("dependency injected", "type", depType.String())
	}
	return nil
}

// AddActivity adds one or more activities to the orchestrator.
// Upon return, the activity results are available via GetResult().
// Returns an error if an activity of the same type already exists.
func (o *Orchestrator) AddActivity(activities ...Activity) error {
	for _, activity := range activities {
		id := o.getOrCacheActivityID(activity)

		// Check for duplicate activity type using map lookup (O(1))
		if _, exists := o.resultMap[id]; exists {
			return fmt.Errorf("activity of type %s already exists", id.String())
		}

		// Store in primary map-based storage
		o.activityMap[id] = activity

		// Immediately create result in NotStarted state
		o.resultMap[id] = &Result{State: NotStarted, Error: nil}
		o.logger.Debug("activity added with initial result", "activity_id", id.String())
	}

	o.logger.Debug("activities added", "count", len(activities), "total", len(o.activityMap))
	return nil
}

// Execute runs all activities with proper dependency management using goroutines.
//
// After Execute() returns (success or failure), every activity will have a Result
// available via GetResultByActivity() that accurately reflects what happened:
//
// AFTER EXECUTE() - depending on what occurred:
//
// 1. Circular Dependency Detection:
//   - State: NotStarted, Error: nil
//   - Activities never progressed due to structural issues
//
// 2. Other Validation Failures (config errors, nil dependencies, etc.):
//   - State: NotStarted, Error: "validation failed: <reason>"
//   - Activities never progressed beyond initial registration
//
// 3. Initialization Failures:
//   - State: NotStarted, Error: "initialization blocked by <activity>: <reason>"
//   - Execution phase never started due to Init() failure
//
// 4. Waiting for Dependencies:
//   - State: Pending, Error: nil
//   - Execution started, activity is waiting for dependencies
//
// 5. Dependency Failures:
//   - State: Skipped, Error: "dependency <dep> failed: <reason>"
//   - Activity was ready to run but dependency failed
//
// 6. Context Cancellation:
//   - State: Skipped, Error: "cancelled: context deadline exceeded"
//
// 7. Currently Executing:
//   - State: Running, Error: nil
//
// 8. Execution Failures:
//   - State: Completed, Error: <activity's execution error>
//
// 9. Success:
//   - State: Completed, Error: nil
func (o *Orchestrator) Execute(ctx context.Context) error {
	if len(o.activityMap) == 0 {
		o.logger.Info("no activities to execute")
		return nil
	}

	o.logger.Info("starting execution", "activity_count", len(o.activityMap))

	// 1. Build dependency graph and inject dependencies/config
	if err := o.buildDependencyGraph(); err != nil {
		o.logger.Error("dependency analysis failed", "error", err)
		// Update all results to show validation failure (using map iteration)
		for id := range o.activityMap {
			o.resultMap[id] = &Result{State: NotStarted, Error: fmt.Errorf("validation failed: %w", err)}
		}
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	o.logger.Debug("dependency graph built successfully")

	// 2. Initialize all activities (using map iteration instead of slice)
	for id, activity := range o.activityMap {
		activityLogger := o.logger.With("activity_module", id.Module, "activity_type", id.Type, "activity_id", id.String())

		activityLogger.Debug("initializing activity")
		if err := activity.Init(); err != nil {
			activityLogger.Error("activity initialization failed", "error", err)
			// Update results for remaining activities that haven't been initialized
			for activityID := range o.activityMap {
				// Only update if still in NotStarted state (hasn't been processed yet)
				if result := o.resultMap[activityID]; result != nil && result.State == NotStarted {
					o.resultMap[activityID] = &Result{State: NotStarted, Error: fmt.Errorf("initialization blocked by %s: %w", id.String(), err)}
				}
			}
			return fmt.Errorf("activity %s initialization failed: %w", id.String(), err)
		}
		activityLogger.Debug("activity initialized successfully")
	}

	// 3. Create completion channels for each activity
	for id := range o.activityMap {
		o.completionChans[id] = make(chan struct{}) // Unbuffered channel, closed when activity completes
		o.logger.Debug("created completion channel", "activity_id", id.String())
	}

	// 4. Start goroutines for each activity using map iteration
	var wg sync.WaitGroup
	errorChan := make(chan error, len(o.activityMap))

	for id, activity := range o.activityMap {
		wg.Add(1)
		o.logger.Debug("starting activity goroutine", "activity_id", id.String())
		go o.runActivity(ctx, id, activity, &wg, errorChan)
	}

	// 5. Wait for all activities to complete and collect all errors
	go func() {
		o.logger.Debug("waiting for all activities to complete")
		wg.Wait()
		o.logger.Debug("all activities completed, closing error channel")
		close(errorChan)
	}()

	// 6. Collect all errors before returning
	o.logger.Debug("collecting errors")
	var errors []error
	for err := range errorChan {
		if err != nil {
			o.logger.Error("activity execution error", "error", err)
			errors = append(errors, err)
		}
	}

	// Return the first error if any occurred
	if len(errors) > 0 {
		o.logger.Error("execution completed with errors", "error_count", len(errors))
		return errors[0]
	}

	o.logger.Info("execution completed successfully")
	return nil
}

// runActivity executes a single activity after waiting for its dependencies
func (o *Orchestrator) runActivity(ctx context.Context, id ActivityID, activity Activity, wg *sync.WaitGroup, errorChan chan<- error) {
	defer wg.Done()
	activityLogger := o.logger.With("activity_module", id.Module, "activity_type", id.Type, "activity_id", id.String())
	activityLogger.Debug("activity goroutine started")

	// Get the pre-initialized result and update it to Pending
	o.mu.Lock()
	result := o.resultMap[id]
	result.State = Pending // Activity is now waiting for dependencies
	o.mu.Unlock()

	// Wait for all dependencies to complete successfully
	dependencies := o.dependencyMap[id]
	activityLogger.Debug("checking dependencies", "dependency_count", len(dependencies))

	for _, depID := range dependencies {
		activityLogger.Debug("waiting for dependency", "dependency", depID.String())
		select {
		case <-ctx.Done():
			activityLogger.Warn("activity cancelled due to context", "error", ctx.Err())
			// Update result to show cancellation
			result = &Result{State: Skipped, Error: fmt.Errorf("cancelled: %w", ctx.Err())}
			o.mu.Lock()
			o.resultMap[id] = result
			o.mu.Unlock()
			errorChan <- fmt.Errorf("activity %s cancelled: %w", id.String(), ctx.Err())
			return
		case <-o.completionChans[depID]:
			activityLogger.Debug("dependency completed", "dependency", depID.String())

			// Check if the dependency was successful using map lookup (O(1))
			o.mu.RLock()
			depResult, exists := o.resultMap[depID]
			o.mu.RUnlock()

			if !exists {
				err := fmt.Errorf("dependency %s completed but no result found", depID.String())
				activityLogger.Error("dependency completed but no result found", "dependency", depID.String())
				result = &Result{State: Skipped, Error: err}
				o.mu.Lock()
				o.resultMap[id] = result
				o.mu.Unlock()
				errorChan <- fmt.Errorf("activity %s: %w", id.String(), err)
				return
			}

			if !depResult.IsSuccess() {
				err := fmt.Errorf("dependency %s failed: %w", depID.String(), depResult.Error)
				activityLogger.Error("dependency failed", "dependency", depID.String(), "error", depResult.Error)
				result = &Result{State: Skipped, Error: err}
				o.mu.Lock()
				o.resultMap[id] = result
				o.mu.Unlock()
				errorChan <- fmt.Errorf("activity %s failed because dependency %s failed", id.String(), depID.String())
				return
			}
		}
	}

	activityLogger.Info("all dependencies satisfied, executing activity")

	// Mark as running
	result = &Result{State: Running, Error: nil}
	o.mu.Lock()
	o.resultMap[id] = result
	o.mu.Unlock()

	// Execute the activity
	err := activity.Execute(ctx)

	// Create final result
	if err != nil {
		activityLogger.Error("activity execution failed", "error", err)
		result = &Result{State: Completed, Error: err}
	} else {
		activityLogger.Info("activity execution completed successfully")
		result = &Result{State: Completed, Error: nil}
	}

	// Store final result
	o.mu.Lock()
	o.resultMap[id] = result
	o.mu.Unlock()

	// Signal completion
	close(o.completionChans[id])
	activityLogger.Debug("completion signal sent")

	// Report error if execution failed
	if err != nil {
		errorChan <- fmt.Errorf("activity %s failed: %w", id.String(), err)
	}
}

// buildDependencyGraph analyzes activity dependencies and injects config/dependencies
// Optimized to eliminate redundant activity ID calculations and use map operations
func (o *Orchestrator) buildDependencyGraph() error {
	o.logger.Debug("building dependency graph")

	// Create reverse lookup map: activity type -> ActivityID (for dependency resolution)
	activityTypeMap := make(map[reflect.Type]ActivityID)

	// First pass: build activity type map and inject config
	// Use map iteration which is more efficient
	for id, activity := range o.activityMap {
		activityType := reflect.TypeOf(activity).Elem()
		activityTypeMap[activityType] = id

		o.logger.Debug("registered activity", "activity_id", id.String())

		// Inject config values (pass the already-computed ID)
		if err := o.injectConfig(activity, id); err != nil {
			o.logger.Error("config injection failed", "activity_id", id.String(), "error", err)
			return fmt.Errorf("config injection failed for %s: %w", id.String(), err)
		}
	}

	// Second pass: build dependency graph and inject activity dependencies
	for id, activity := range o.activityMap {
		dependencies := []ActivityID{}

		activityValue := reflect.ValueOf(activity).Elem()
		activityType := activityValue.Type()

		for i := 0; i < activityType.NumField(); i++ {
			field := activityType.Field(i)
			fieldValue := activityValue.Field(i)

			// Skip unexported fields and config fields
			if !fieldValue.CanSet() || field.Tag.Get("config") != "" {
				continue
			}

			// Handle activity dependency injection (only if not already injected via type injection)
			if field.Type.Kind() == reflect.Ptr {
				pointedType := field.Type.Elem()
				// Skip if this type was already injected via type injection
				if _, alreadyInjected := o.injectedTypes[field.Type]; alreadyInjected {
					o.logger.Debug("skipping activity dependency - already injected via type injection", "activity_id", id.String(), "field", field.Name)
					continue
				}
				if _, alreadyInjected := o.injectedTypes[pointedType]; alreadyInjected {
					o.logger.Debug("skipping activity dependency - pointed type already injected", "activity_id", id.String(), "field", field.Name)
					continue
				}
				// Use map lookup for dependency resolution (O(1))
				if depID, exists := activityTypeMap[pointedType]; exists {
					// This is a dependency - record the dependency
					dependencies = append(dependencies, depID)
					o.logger.Debug("activity dependency detected", "activity_id", id.String(), "dependency", depID.String(), "field_name", field.Name)

					// Only inject the value if it's not an unnamed field (unnamed fields are for ordering only)
					if field.Name != "_" {
						// Use map lookup to get dependency activity (O(1))
						dependencyActivity := o.activityMap[depID]
						fieldValue.Set(reflect.ValueOf(dependencyActivity))
						o.logger.Debug("dependency injected into named field", "activity_id", id.String(), "field", field.Name)
					} else {
						o.logger.Debug("unnamed dependency registered for ordering only", "activity_id", id.String(), "dependency", depID.String())
					}
				}
			} else if _, exists := activityTypeMap[field.Type]; exists {
				// Direct struct dependency - this is not allowed!
				return fmt.Errorf("activity %s dependency field %s must be a pointer (*%s), not a struct (%s)",
					id.String(), field.Name, field.Type.Name(), field.Type.Name())
			}
		}

		o.dependencyMap[id] = dependencies
	}

	// Log dependency graph
	o.logger.Debug("dependency graph built")
	for id, deps := range o.dependencyMap {
		depStrings := make([]string, len(deps))
		for i, dep := range deps {
			depStrings[i] = dep.String()
		}
		o.logger.Debug("dependency mapping", "activity_id", id.String(), "dependencies", depStrings)
	}

	// Validate no circular dependencies
	if err := o.validateNoCycles(); err != nil {
		o.logger.Error("circular dependency detected", "error", err)
		// For circular dependencies, leave results in NotStarted state
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Validate dependencies after injection
	if err := o.validateDependencies(); err != nil {
		// For other validation failures, activities remain in NotStarted state with error
		for id := range o.activityMap {
			o.resultMap[id] = &Result{State: NotStarted, Error: fmt.Errorf("validation failed: %w", err)}
		}
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	return nil
}

// injectConfig handles config and type injection for a single activity
// Optimized version that takes precomputed ActivityID to avoid redundant reflection
func (o *Orchestrator) injectConfig(activity Activity, activityID ActivityID) error {
	activityValue := reflect.ValueOf(activity).Elem()
	activityType := activityValue.Type()

	for i := 0; i < activityType.NumField(); i++ {
		field := activityType.Field(i)
		fieldValue := activityValue.Field(i)

		// Skip unexported fields
		if !fieldValue.CanSet() {
			continue
		}

		// Handle config injection
		if configTag := field.Tag.Get("config"); configTag != "" {
			o.logger.Debug("injecting config", "activity_id", activityID.String(), "field", field.Name, "config_path", configTag)
			if err := o.injectConfigValue(fieldValue, configTag); err != nil {
				return fmt.Errorf("config injection failed for field %s: %w", field.Name, err)
			}
			continue
		}

		// Handle type injection for non-activity dependencies
		if injectedValue, exists := o.injectedTypes[field.Type]; exists {
			o.logger.Debug("injecting type dependency", "activity_id", activityID.String(), "field", field.Name, "type", field.Type.String())
			fieldValue.Set(reflect.ValueOf(injectedValue))
			continue
		}

		// Handle pointer type injection
		if field.Type.Kind() == reflect.Ptr {
			if injectedValue, exists := o.injectedTypes[field.Type]; exists {
				o.logger.Debug("injecting pointer type dependency", "activity_id", activityID.String(), "field", field.Name, "type", field.Type.String())
				fieldValue.Set(reflect.ValueOf(injectedValue))
				continue
			}
			// Also check if we have the pointed-to type
			pointedType := field.Type.Elem()
			if injectedValue, exists := o.injectedTypes[pointedType]; exists {
				o.logger.Debug("injecting pointed-to type dependency", "activity_id", activityID.String(), "field", field.Name, "type", pointedType.String())
				// Create a pointer to the injected value
				injectedPtr := reflect.New(pointedType)
				injectedPtr.Elem().Set(reflect.ValueOf(injectedValue))
				fieldValue.Set(injectedPtr)
				continue
			}
		}
	}

	return nil
}

// validateNoCycles performs a topological sort to detect circular dependencies
// Uses optimized map operations but same algorithm
func (o *Orchestrator) validateNoCycles() error {
	// Use Kahn's algorithm for topological sorting
	inDegree := make(map[ActivityID]int)

	// Initialize in-degree counts using map iteration
	for id := range o.activityMap {
		inDegree[id] = 0
	}

	// Calculate in-degrees: if A depends on B, then A has incoming edge from B
	for activityID, deps := range o.dependencyMap {
		inDegree[activityID] = len(deps) // Number of dependencies = in-degree
	}

	o.logger.Debug("calculated in-degrees")

	// Find activities with no incoming edges (no dependencies)
	queue := []ActivityID{}
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
			o.logger.Debug("activity has no dependencies", "activity_id", id.String())
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++
		o.logger.Debug("processing activity", "activity_id", current.String(), "processed", processed, "total", len(o.activityMap))

		// For each activity that depends on current, reduce its in-degree
		for activityID, deps := range o.dependencyMap {
			for _, dep := range deps {
				if dep.Equal(current) {
					inDegree[activityID]--
					if inDegree[activityID] == 0 {
						queue = append(queue, activityID)
						o.logger.Debug("activity dependencies satisfied", "activity_id", activityID.String())
					}
				}
			}
		}
	}

	if processed != len(o.activityMap) {
		return fmt.Errorf("circular dependency detected - only %d of %d activities could be processed", processed, len(o.activityMap))
	}

	o.logger.Debug("dependency validation completed", "processed", processed)
	return nil
}

// validateDependencies checks that all required dependencies are properly injected
// Uses map iteration for efficiency
func (o *Orchestrator) validateDependencies() error {
	for id, activity := range o.activityMap {
		activityLogger := o.logger.With("activity_id", id.String())
		activityLogger.Debug("validating activity dependencies")

		// Get activity type
		activityType := reflect.TypeOf(activity).Elem()
		activityValue := reflect.ValueOf(activity).Elem()

		// Check each field
		for i := 0; i < activityType.NumField(); i++ {
			field := activityType.Field(i)
			fieldValue := activityValue.Field(i)

			// Skip non-pointer fields and fields with config tags
			if fieldValue.Kind() != reflect.Ptr || field.Tag.Get("config") != "" {
				continue
			}

			// Check if the field is nil (skip unnamed fields as they're used for ordering only)
			if fieldValue.IsNil() && field.Name != "_" {
				activityLogger.Error("nil dependency found", "field", field.Name, "type", field.Type.String())
				return fmt.Errorf("activity %s has nil dependency: %s (%s)", id.String(), field.Name, field.Type.String())
			}
		}
	}

	return nil
}

// injectConfigValue injects a config value using dot notation path
func (o *Orchestrator) injectConfigValue(fieldValue reflect.Value, configPath string) error {
	if o.config == nil {
		o.logger.Debug("no config provided, skipping config injection", "config_path", configPath)
		return nil
	}

	value := reflect.ValueOf(o.config)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Navigate through the config using dot notation
	parts := strings.Split(configPath, ".")
	for _, part := range parts {
		if value.Kind() != reflect.Struct {
			return fmt.Errorf("config path %s: expected struct, got %s", configPath, value.Kind())
		}

		// Try to find the field by name, handling common Go naming conventions
		var fieldVal reflect.Value

		// Try exact match first
		fieldVal = value.FieldByName(part)
		if !fieldVal.IsValid() {
			// Try capitalized version
			fieldName := strings.ToUpper(part[:1]) + part[1:]
			fieldVal = value.FieldByName(fieldName)
		}
		if !fieldVal.IsValid() {
			// Try all uppercase (for acronyms like API)
			fieldVal = value.FieldByName(strings.ToUpper(part))
		}

		// If still not found, try looking for YAML tags
		if !fieldVal.IsValid() {
			typ := value.Type()
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
					// Split the YAML tag to handle options like omitempty
					yamlName := strings.Split(yamlTag, ",")[0]
					if yamlName == part {
						fieldVal = value.Field(i)
						break
					}
				}
			}
		}

		if !fieldVal.IsValid() {
			return fmt.Errorf("config path %s: field for '%s' not found", configPath, part)
		}
		value = fieldVal
	}

	// Set the field value
	if !value.Type().AssignableTo(fieldValue.Type()) {
		return fmt.Errorf("config path %s: type %s not assignable to %s", configPath, value.Type(), fieldValue.Type())
	}

	fieldValue.Set(value)
	return nil
}

// getOrCacheActivityID returns the ActivityID for an activity, using cache for performance
// This is the key optimization that eliminates redundant reflection calls
func (o *Orchestrator) getOrCacheActivityID(activity Activity) ActivityID {
	// Check cache first for O(1) lookup
	if id, exists := o.activityIDCache[activity]; exists {
		return id
	}

	// Calculate and cache the ActivityID
	activityType := reflect.TypeOf(activity).Elem()
	id := ActivityID{
		Module: activityType.PkgPath(),
		Type:   activityType.Name(),
	}
	o.activityIDCache[activity] = id
	return id
}

// GetResult returns the result of a completed activity by ActivityID (thread-safe)
func (o *Orchestrator) GetResult(id ActivityID) *Result {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.resultMap[id]
}

// GetResultByActivity returns the result of a completed activity by activity reference (thread-safe)
// Optimized to use cached ActivityID lookup
func (o *Orchestrator) GetResultByActivity(activity Activity) *Result {
	id := o.getOrCacheActivityID(activity)
	return o.GetResult(id)
}

// GetAllResults returns all activity results (thread-safe)
func (o *Orchestrator) GetAllResults() map[ActivityID]*Result {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Return a copy to prevent external modification
	results := make(map[ActivityID]*Result, len(o.resultMap))
	for id, result := range o.resultMap {
		results[id] = result
	}
	return results
}
