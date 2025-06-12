package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
)

// Orchestrator manages the execution of activities with dependency injection
type Orchestrator struct {
	activities []Activity
	results    map[string]Result
	config     interface{}
	logger     *slog.Logger

	// Dependency injection map: type -> instance
	injectedTypes map[reflect.Type]interface{}

	// Dependency graph and execution coordination
	activityMap     map[string]Activity
	dependencyMap   map[string][]string      // activity name -> list of dependency names
	completionChans map[string]chan struct{} // activity name -> completion signal (closed when done)
	resultMap       map[string]Result        // activity name -> result (protected by mutex)
	mu              sync.RWMutex
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config interface{}) *Orchestrator {
	return &Orchestrator{
		activities:      make([]Activity, 0),
		results:         make(map[string]Result),
		config:          config,
		logger:          slog.Default().With("component", "orchestrator"),
		injectedTypes:   make(map[reflect.Type]interface{}),
		activityMap:     make(map[string]Activity),
		dependencyMap:   make(map[string][]string),
		completionChans: make(map[string]chan struct{}),
		resultMap:       make(map[string]Result),
	}
}

// NewOrchestratorWithLogger creates a new orchestrator instance with a custom logger
func NewOrchestratorWithLogger(config interface{}, logger *slog.Logger) *Orchestrator {
	o := NewOrchestrator(config)
	o.logger = logger.With("component", "orchestrator")
	return o
}

// Inject adds one or more typed dependencies to the orchestrator for injection into activities
func (o *Orchestrator) Inject(deps ...interface{}) {
	for _, dep := range deps {
		if dep == nil {
			o.logger.Warn("attempted to inject nil dependency")
			continue
		}

		depType := reflect.TypeOf(dep)
		o.injectedTypes[depType] = dep
		o.logger.Debug("dependency injected", "type", depType.String())
	}
}

// AddActivity adds one or more activities to the orchestrator
func (o *Orchestrator) AddActivity(activities ...Activity) {
	o.activities = append(o.activities, activities...)
	o.logger.Debug("activities added", "count", len(activities), "total", len(o.activities))
}

// Execute runs all activities with proper dependency management using goroutines
func (o *Orchestrator) Execute(ctx context.Context) error {
	if len(o.activities) == 0 {
		o.logger.Info("no activities to execute")
		return nil
	}

	o.logger.Info("starting execution", "activity_count", len(o.activities))

	// 1. Build dependency graph and inject dependencies/config
	if err := o.buildDependencyGraph(); err != nil {
		o.logger.Error("dependency analysis failed", "error", err)
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	o.logger.Debug("dependency graph built successfully")

	// 2. Initialize all activities
	for _, activity := range o.activities {
		activityName := o.getActivityName(activity)
		activityLogger := o.logger.With("activity", activityName)

		activityLogger.Debug("initializing activity")
		if err := activity.Init(); err != nil {
			activityLogger.Error("activity initialization failed", "error", err)
			return fmt.Errorf("activity %s initialization failed: %w", activityName, err)
		}
		activityLogger.Debug("activity initialized successfully")
	}

	// 3. Create completion channels for each activity
	for name := range o.activityMap {
		o.completionChans[name] = make(chan struct{}) // Unbuffered channel, closed when activity completes
		o.logger.Debug("created completion channel", "activity", name)
	}

	// 4. Start goroutines for each activity
	var wg sync.WaitGroup
	errorChan := make(chan error, len(o.activities))

	for name, activity := range o.activityMap {
		wg.Add(1)
		o.logger.Debug("starting activity goroutine", "activity", name)
		go o.runActivity(ctx, name, activity, &wg, errorChan)
	}

	// 5. Wait for all activities to complete
	go func() {
		o.logger.Debug("waiting for all activities to complete")
		wg.Wait()
		o.logger.Debug("all activities completed, closing error channel")
		close(errorChan)
	}()

	// 6. Check for any errors
	o.logger.Debug("checking for errors")
	for err := range errorChan {
		if err != nil {
			o.logger.Error("activity execution error", "error", err)
			return err
		}
	}

	return nil
}

// runActivity executes a single activity after waiting for its dependencies
func (o *Orchestrator) runActivity(ctx context.Context, name string, activity Activity, wg *sync.WaitGroup, errorChan chan<- error) {
	defer wg.Done()
	activityLogger := o.logger.With("activity", name)
	activityLogger.Debug("activity goroutine started")

	// Wait for all dependencies to complete successfully
	dependencies := o.dependencyMap[name]
	activityLogger.Debug("checking dependencies", "dependency_count", len(dependencies), "dependencies", dependencies)

	for _, depName := range dependencies {
		activityLogger.Debug("waiting for dependency", "dependency", depName)
		select {
		case <-ctx.Done():
			activityLogger.Warn("activity cancelled due to context", "error", ctx.Err())
			errorChan <- fmt.Errorf("activity %s cancelled: %w", name, ctx.Err())
			return
		case <-o.completionChans[depName]:
			activityLogger.Debug("dependency completed", "dependency", depName)

			// Check if the dependency was successful
			o.mu.RLock()
			result, exists := o.resultMap[depName]
			o.mu.RUnlock()

			if !exists {
				activityLogger.Error("dependency completed but no result found", "dependency", depName)
				errorChan <- fmt.Errorf("activity %s: dependency %s completed but no result found", name, depName)
				return
			}

			if !result.IsSuccess() {
				activityLogger.Error("dependency failed", "dependency", depName)
				errorChan <- fmt.Errorf("activity %s failed because dependency %s failed", name, depName)
				return
			}
		}
	}

	activityLogger.Info("all dependencies satisfied, executing activity")

	// All dependencies satisfied, run the activity
	result, err := activity.Run(ctx)
	if err != nil {
		activityLogger.Error("activity execution failed", "error", err)
		errorChan <- fmt.Errorf("activity %s failed: %w", name, err)
		return
	}

	activityLogger.Info("activity execution completed", "success", result.IsSuccess())

	// Store result first
	o.mu.Lock()
	o.resultMap[name] = result
	o.results[name] = result // Also store in legacy results map
	o.mu.Unlock()

	// Signal completion by closing the channel
	close(o.completionChans[name])
	activityLogger.Debug("completion signal sent")

	// If this activity failed, signal error
	if !result.IsSuccess() {
		activityLogger.Error("activity reported failure")
		errorChan <- fmt.Errorf("activity %s reported failure", name)
	}
}

// buildDependencyGraph analyzes activity dependencies and injects config/dependencies
func (o *Orchestrator) buildDependencyGraph() error {
	o.logger.Debug("building dependency graph")

	// First pass: build activity map and inject config
	activityTypeMap := make(map[reflect.Type]string)

	for _, activity := range o.activities {
		name := o.getActivityName(activity)
		activityType := reflect.TypeOf(activity).Elem()

		o.activityMap[name] = activity
		activityTypeMap[activityType] = name
		o.logger.Debug("registered activity", "activity", name)

		// Inject config values
		if err := o.injectConfig(activity); err != nil {
			o.logger.Error("config injection failed", "activity", name, "error", err)
			return fmt.Errorf("config injection failed for %s: %w", name, err)
		}
	}

	// Validate dependencies before proceeding
	if err := o.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	// Second pass: build dependency graph and inject activity dependencies
	for _, activity := range o.activities {
		name := o.getActivityName(activity)
		dependencies := []string{}

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
					o.logger.Debug("skipping activity dependency - already injected via type injection", "activity", name, "field", field.Name)
					continue
				}
				if _, alreadyInjected := o.injectedTypes[pointedType]; alreadyInjected {
					o.logger.Debug("skipping activity dependency - pointed type already injected", "activity", name, "field", field.Name)
					continue
				}
				if depName, exists := activityTypeMap[pointedType]; exists {
					// This is a dependency - inject it and record the dependency
					fieldValue.Set(reflect.ValueOf(o.activityMap[depName]))
					dependencies = append(dependencies, depName)
					o.logger.Debug("activity dependency detected", "activity", name, "dependency", depName)
				}
			} else if _, exists := activityTypeMap[field.Type]; exists {
				// Direct struct dependency - this is not allowed!
				return fmt.Errorf("activity %s dependency field %s must be a pointer (*%s), not a struct (%s)",
					name, field.Name, field.Type.Name(), field.Type.Name())
			}
		}

		o.dependencyMap[name] = dependencies
	}

	// Log dependency graph
	o.logger.Debug("dependency graph built")
	for name, deps := range o.dependencyMap {
		o.logger.Debug("dependency mapping", "activity", name, "dependencies", deps)
	}

	// Validate no circular dependencies
	if err := o.validateNoCycles(); err != nil {
		o.logger.Error("circular dependency detected", "error", err)
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	return nil
}

// injectConfig handles config and type injection for a single activity
func (o *Orchestrator) injectConfig(activity Activity) error {
	activityName := o.getActivityName(activity)
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
			o.logger.Debug("injecting config", "activity", activityName, "field", field.Name, "config_path", configTag)
			if err := o.injectConfigValue(fieldValue, configTag); err != nil {
				return fmt.Errorf("config injection failed for field %s: %w", field.Name, err)
			}
			continue
		}

		// Handle type injection for non-activity dependencies
		if injectedValue, exists := o.injectedTypes[field.Type]; exists {
			o.logger.Debug("injecting type dependency", "activity", activityName, "field", field.Name, "type", field.Type.String())
			fieldValue.Set(reflect.ValueOf(injectedValue))
			continue
		}

		// Handle pointer type injection
		if field.Type.Kind() == reflect.Ptr {
			if injectedValue, exists := o.injectedTypes[field.Type]; exists {
				o.logger.Debug("injecting pointer type dependency", "activity", activityName, "field", field.Name, "type", field.Type.String())
				fieldValue.Set(reflect.ValueOf(injectedValue))
				continue
			}
			// Also check if we have the pointed-to type
			pointedType := field.Type.Elem()
			if injectedValue, exists := o.injectedTypes[pointedType]; exists {
				o.logger.Debug("injecting pointed-to type dependency", "activity", activityName, "field", field.Name, "type", pointedType.String())
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
func (o *Orchestrator) validateNoCycles() error {
	// Use Kahn's algorithm for topological sorting
	inDegree := make(map[string]int)

	// Initialize in-degree counts
	for name := range o.activityMap {
		inDegree[name] = 0
	}

	// Calculate in-degrees: if A depends on B, then A has incoming edge from B
	for activityName, deps := range o.dependencyMap {
		inDegree[activityName] = len(deps) // Number of dependencies = in-degree
	}

	o.logger.Debug("calculated in-degrees", "in_degrees", inDegree)

	// Find activities with no incoming edges (no dependencies)
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
			o.logger.Debug("activity has no dependencies", "activity", name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++
		o.logger.Debug("processing activity", "activity", current, "processed", processed, "total", len(o.activityMap))

		// For each activity that depends on current, reduce its in-degree
		for activityName, deps := range o.dependencyMap {
			for _, dep := range deps {
				if dep == current {
					inDegree[activityName]--
					if inDegree[activityName] == 0 {
						queue = append(queue, activityName)
						o.logger.Debug("activity dependencies satisfied", "activity", activityName)
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
func (o *Orchestrator) validateDependencies() error {
	for name, activity := range o.activityMap {
		activityLogger := o.logger.With("activity", name)
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

			// Check if the field is nil
			if fieldValue.IsNil() {
				activityLogger.Error("nil dependency found", "field", field.Name, "type", field.Type.String())
				return fmt.Errorf("activity %s has nil dependency: %s (%s)", name, field.Name, field.Type.String())
			}
		}
	}

	return nil
}

// injectConfigValue injects a config value using dot notation path
func (o *Orchestrator) injectConfigValue(fieldValue reflect.Value, configPath string) error {
	if o.config == nil {
		return fmt.Errorf("no config provided")
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

// getActivityName returns the name of an activity type
func (o *Orchestrator) getActivityName(activity Activity) string {
	activityType := reflect.TypeOf(activity).Elem()
	return activityType.Name()
}

// GetResult returns the result of a completed activity (thread-safe)
func (o *Orchestrator) GetResult(activityName string) (Result, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result, exists := o.results[activityName]
	return result, exists
}
