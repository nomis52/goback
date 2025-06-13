package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActivityID_Methods tests all ActivityID methods
func TestActivityID_Methods(t *testing.T) {
	// Test valid ActivityID
	id := ActivityID{
		Module: "github.com/nomis52/goback/activities",
		Type:   "PowerOnPBSActivity",
	}

	t.Run("String", func(t *testing.T) {
		expected := "github.com/nomis52/goback/activities.PowerOnPBSActivity"
		assert.Equal(t, expected, id.String())
	})

	t.Run("Key", func(t *testing.T) {
		// Key should currently be same as String
		assert.Equal(t, id.String(), id.Key())
	})

	t.Run("IsValid", func(t *testing.T) {
		assert.True(t, id.IsValid(), "ID with both fields should be valid")

		invalidID1 := ActivityID{Module: "", Type: "TestActivity"}
		assert.False(t, invalidID1.IsValid(), "ID with empty Module should be invalid")

		invalidID2 := ActivityID{Module: "test", Type: ""}
		assert.False(t, invalidID2.IsValid(), "ID with empty Type should be invalid")

		invalidID3 := ActivityID{}
		assert.False(t, invalidID3.IsValid(), "Empty ID should be invalid")
	})

	t.Run("Equal", func(t *testing.T) {
		sameID := ActivityID{
			Module: "github.com/nomis52/goback/activities",
			Type:   "PowerOnPBSActivity",
		}
		assert.True(t, id.Equal(sameID), "Identical IDs should be equal")

		differentModule := ActivityID{
			Module: "github.com/other/activities",
			Type:   "PowerOnPBSActivity",
		}
		assert.False(t, id.Equal(differentModule), "Different modules should not be equal")

		differentType := ActivityID{
			Module: "github.com/nomis52/goback/activities",
			Type:   "BackupTaskActivity",
		}
		assert.False(t, id.Equal(differentType), "Different types should not be equal")
	})

	t.Run("ShortString", func(t *testing.T) {
		expected := "activities.PowerOnPBSActivity"
		assert.Equal(t, expected, id.ShortString())

		// Test with single component module
		simpleID := ActivityID{Module: "main", Type: "TaskActivity"}
		assert.Equal(t, "main.TaskActivity", simpleID.ShortString())

		// Test with empty module
		emptyModuleID := ActivityID{Module: "", Type: "TaskActivity"}
		assert.Equal(t, "TaskActivity", emptyModuleID.ShortString())
	})
}

// TestActivityID_CollisionPrevention tests the collision prevention capabilities
func TestActivityID_CollisionPrevention(t *testing.T) {
	// Test that activities with same struct name but different modules are unique
	id1 := ActivityID{
		Module: "github.com/user/app/activities",
		Type:   "BackupTask",
	}
	
	id2 := ActivityID{
		Module: "github.com/vendor/lib/activities", 
		Type:   "BackupTask",
	}

	// Should be different despite same Type
	assert.False(t, id1.Equal(id2), "Activities with same Type but different Module should not be equal")
	assert.NotEqual(t, id1.String(), id2.String(), "String representations should be different")
	assert.NotEqual(t, id1.Key(), id2.Key(), "Keys should be different")

	// Test map key uniqueness
	activityMap := make(map[string]bool)
	activityMap[id1.Key()] = true
	activityMap[id2.Key()] = true
	
	assert.Len(t, activityMap, 2, "Should have 2 unique keys in map")
}

// TestActivityID_EdgeCases tests edge cases and boundary conditions
func TestActivityID_EdgeCases(t *testing.T) {
	t.Run("EmptyFields", func(t *testing.T) {
		emptyID := ActivityID{}
		assert.Equal(t, ".", emptyID.String())
		assert.Equal(t, ".", emptyID.Key())
		assert.Equal(t, "", emptyID.ShortString())
		assert.False(t, emptyID.IsValid())
	})

	t.Run("OnlyModule", func(t *testing.T) {
		moduleOnlyID := ActivityID{Module: "github.com/test/module"}
		assert.Equal(t, "github.com/test/module.", moduleOnlyID.String())
		assert.Equal(t, "module.", moduleOnlyID.ShortString())
		assert.False(t, moduleOnlyID.IsValid())
	})

	t.Run("OnlyType", func(t *testing.T) {
		typeOnlyID := ActivityID{Type: "TestActivity"}
		assert.Equal(t, ".TestActivity", typeOnlyID.String())
		assert.Equal(t, "TestActivity", typeOnlyID.ShortString())
		assert.False(t, typeOnlyID.IsValid())
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		specialID := ActivityID{
			Module: "github.com/test-user/my_app/activities",
			Type:   "BackupTask_V2",
		}
		assert.True(t, specialID.IsValid())
		assert.Equal(t, "activities.BackupTask_V2", specialID.ShortString())
	})

	t.Run("DeepNestedModule", func(t *testing.T) {
		deepID := ActivityID{
			Module: "github.com/company/project/internal/services/backup/activities",
			Type:   "DeepTask",
		}
		assert.Equal(t, "activities.DeepTask", deepID.ShortString())
		assert.Contains(t, deepID.String(), "internal/services/backup/activities.DeepTask")
	})
}

// TestActivityID_RealWorldScenarios tests realistic usage scenarios
func TestActivityID_RealWorldScenarios(t *testing.T) {
	t.Run("PBSAutomationScenario", func(t *testing.T) {
		// Simulate real PBS automation activities
		powerOnID := ActivityID{
			Module: "github.com/nomis52/goback/activities",
			Type:   "PowerOnPBSActivity",
		}
		
		backupID := ActivityID{
			Module: "github.com/nomis52/goback/activities", 
			Type:   "RunProxmoxBackupActivity",
		}
		
		// Vendor activity with same name
		vendorPowerOnID := ActivityID{
			Module: "github.com/vendor/pbs-tools/activities",
			Type:   "PowerOnPBSActivity",
		}

		// All should be valid and unique
		assert.True(t, powerOnID.IsValid())
		assert.True(t, backupID.IsValid())
		assert.True(t, vendorPowerOnID.IsValid())
		
		assert.False(t, powerOnID.Equal(backupID))
		assert.False(t, powerOnID.Equal(vendorPowerOnID))
		assert.False(t, backupID.Equal(vendorPowerOnID))

		// Test in map to ensure uniqueness
		activityResults := make(map[string]string)
		activityResults[powerOnID.Key()] = "success"
		activityResults[backupID.Key()] = "success"
		activityResults[vendorPowerOnID.Key()] = "failure"
		
		assert.Len(t, activityResults, 3, "Should have 3 unique activities")
		assert.Equal(t, "success", activityResults[powerOnID.Key()])
		assert.Equal(t, "failure", activityResults[vendorPowerOnID.Key()])
	})

	t.Run("CommonNamingCollisions", func(t *testing.T) {
		// Test common activity names that would collide without full module paths
		commonNames := []string{"Task", "Job", "Activity", "Worker", "Service", "Manager"}
		modules := []string{
			"github.com/company1/app/tasks",
			"github.com/company2/service/jobs", 
			"github.com/vendor/lib/workers",
		}

		activityIDs := make([]ActivityID, 0)
		for _, module := range modules {
			for _, name := range commonNames {
				id := ActivityID{Module: module, Type: name + "Activity"}
				activityIDs = append(activityIDs, id)
			}
		}

		// Verify all are unique
		keySet := make(map[string]bool)
		for _, id := range activityIDs {
			assert.True(t, id.IsValid(), "Activity ID should be valid: %s", id.String())
			
			key := id.Key()
			assert.False(t, keySet[key], "Key should be unique: %s", key)
			keySet[key] = true
		}

		expectedCount := len(modules) * len(commonNames)
		assert.Len(t, keySet, expectedCount, "Should have %d unique activity IDs", expectedCount)
	})
}
