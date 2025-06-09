package orchestrator

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Orchestrator manages the execution of activities with dependency injection
type Orchestrator struct {
	activities []Activity
	results    map[string]Result
	config     interface{}
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config interface{}) *Orchestrator {
	return &Orchestrator{
		activities: make([]Activity, 0),
		results:    make(map[string]Result),
		config:     config,
	}
}

// AddActivity adds one or more activities to the orchestrator
func (o *Orchestrator) AddActivity(activities ...Activity) {
	o.activities = append(o.activities, activities...)
}

// Execute runs all activities with dependency injection
func (o *Orchestrator) Execute(ctx context.Context) error {
	// 1. Inject dependencies and config
	if err := o.injectDependencies(); err != nil {
		return fmt.Errorf("dependency injection failed: %w", err)
	}

	// 2. Initialize all activities
	for _, activity := range o.activities {
		if err := activity.Init(); err != nil {
			return fmt.Errorf("activity initialization failed: %w", err)
		}
	}

	// 3. Execute activities in order
	for _, activity := range o.activities {
		activityName := o.getActivityName(activity)
		
		result, err := activity.Run(ctx)
		if err != nil {
			return fmt.Errorf("activity %s failed: %w", activityName, err)
		}
		
		o.results[activityName] = result
		
		// Stop if activity failed
		if !result.IsSuccess() {
			return fmt.Errorf("activity %s reported failure", activityName)
		}
	}

	return nil
}

// injectDependencies uses reflection to wire activities and config
func (o *Orchestrator) injectDependencies() error {
	// Create a map of activity types for dependency lookup
	activityMap := make(map[reflect.Type]Activity)
	for _, activity := range o.activities {
		activityType := reflect.TypeOf(activity).Elem() // Get type without pointer
		activityMap[activityType] = activity
	}

	// Inject dependencies into each activity
	for _, activity := range o.activities {
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
				if err := o.injectConfigValue(fieldValue, configTag); err != nil {
					return fmt.Errorf("config injection failed for field %s: %w", field.Name, err)
				}
				continue
			}

			// Handle activity dependency injection
			if _, exists := activityMap[field.Type]; exists {
				// Direct struct dependency - this is not allowed!
				return fmt.Errorf("activity dependency field %s must be a pointer (*%s), not a struct (%s)", field.Name, field.Type.Name(), field.Type.Name())
			} else if field.Type.Kind() == reflect.Ptr {
				// Pointer dependency - check if we have the pointed-to type
				pointedType := field.Type.Elem()
				if dep, exists := activityMap[pointedType]; exists {
					// Set the pointer to the dependency
					fieldValue.Set(reflect.ValueOf(dep))
				}
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
