package workflow

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingObserver captures the sequence of states reported for each activity.
type recordingObserver struct {
	mu     sync.Mutex
	states map[string][]ActivityState
}

func newRecordingObserver() *recordingObserver {
	return &recordingObserver{states: make(map[string][]ActivityState)}
}

func (r *recordingObserver) observe(id ActivityID, result *Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[id.String()] = append(r.states[id.String()], result.State)
}

func (r *recordingObserver) forType(typeName string) []ActivityState {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, states := range r.states {
		if id[len(id)-len(typeName):] == typeName {
			out := make([]ActivityState, len(states))
			copy(out, states)
			return out
		}
	}
	return nil
}

func TestOrchestrator_ActivityStateObserver_Success(t *testing.T) {
	obs := newRecordingObserver()
	o := NewOrchestrator()
	o.SetActivityStateObserver(obs.observe)

	require.NoError(t, o.AddActivity(&PassActivity{}))
	require.NoError(t, o.Execute(context.Background()))

	// A successful step passes through Pending -> Running -> Completed.
	assert.Equal(t, []ActivityState{Pending, Running, Completed}, obs.forType("PassActivity"))
}

func TestOrchestrator_ActivityStateObserver_Skipped(t *testing.T) {
	obs := newRecordingObserver()
	o := NewOrchestrator()
	o.SetActivityStateObserver(obs.observe)

	// The dependent activity is skipped because its dependency fails; it must
	// never report Running (so a running gauge never sticks at 1 for it).
	require.NoError(t, o.AddActivity(&FailActivity{}, &DependentOnFailingActivity{}))
	require.Error(t, o.Execute(context.Background()))

	dependent := obs.forType("DependentOnFailingActivity")
	assert.Equal(t, []ActivityState{Pending, Skipped}, dependent)
	assert.NotContains(t, dependent, Running)
}

func TestOrchestrator_ActivityStateObserver_NotSet(t *testing.T) {
	// With no observer registered, Execute must not panic.
	o := NewOrchestrator()
	require.NoError(t, o.AddActivity(&PassActivity{}))
	require.NoError(t, o.Execute(context.Background()))
}
