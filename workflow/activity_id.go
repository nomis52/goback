package workflow

import (
	"fmt"
	"reflect"
)

// ActivityID provides collision-proof identification for activities across modules.
// It combines the full import path with the struct name to ensure global uniqueness,
// solving the naming collision problem that occurs when multiple packages contain
// activities with identical struct names.
//
// Examples:
//   - ActivityID{Module: "github.com/user/app/activities", Type: "PowerOnPBS"}
//   - ActivityID{Module: "github.com/vendor/lib/activities", Type: "PowerOnPBS"}
//
// These represent completely different activities despite having the same Type name.
type ActivityID struct {
	// Module is the full import path of the package containing the activity.
	// This is obtained via reflect.TypeOf(activity).Elem().PkgPath().
	// Example: "github.com/nomis52/goback/workflows/backup"
	Module string

	// Type is the struct name of the activity.
	// This is obtained via reflect.TypeOf(activity).Elem().Name().
	// Example: "PowerOnPBS"
	Type string
}

// String returns a human-readable representation of the ActivityID.
// The format is "Module.Type", providing clear identification of the activity's
// origin and type.
//
// Example: "github.com/nomis52/goback/workflows/backup.PowerOnPBS"
func (id ActivityID) String() string {
	return fmt.Sprintf("%s.%s", id.Module, id.Type)
}

// Key returns a string suitable for use as a map key.
// Currently identical to String(), but provided as a separate method
// to allow for future optimization if needed.
func (id ActivityID) Key() string {
	return id.String()
}

// IsValid returns true if the ActivityID has both Module and Type populated.
// This is useful for validation when constructing ActivityIDs manually.
func (id ActivityID) IsValid() bool {
	return id.Module != "" && id.Type != ""
}

// Equal returns true if this ActivityID is identical to another ActivityID.
// Both Module and Type must match exactly.
func (id ActivityID) Equal(other ActivityID) bool {
	return id.Module == other.Module && id.Type == other.Type
}

// ShortString returns a shortened version of the ActivityID for display purposes.
// It includes only the last component of the module path plus the type.
// This is useful for logging and UI display where full paths would be too verbose.
//
// Example: "github.com/nomis52/goback/workflows/backup.PowerOnPBS" becomes "backup.PowerOnPBS"
func (id ActivityID) ShortString() string {
	if id.Module == "" {
		return id.Type
	}

	// Find the last component of the module path
	lastSlash := -1
	for i := len(id.Module) - 1; i >= 0; i-- {
		if id.Module[i] == '/' {
			lastSlash = i
			break
		}
	}

	var packageName string
	if lastSlash >= 0 && lastSlash < len(id.Module)-1 {
		packageName = id.Module[lastSlash+1:]
	} else {
		packageName = id.Module
	}

	return fmt.Sprintf("%s.%s", packageName, id.Type)
}

// GetActivityID returns the ActivityID for an activity.
// This is a helper function that activities can use to identify themselves
// when reporting status or performing other operations that require an ActivityID.
func GetActivityID(activity Activity) ActivityID {
	activityType := reflect.TypeOf(activity).Elem()
	return ActivityID{
		Module: activityType.PkgPath(),
		Type:   activityType.Name(),
	}
}
