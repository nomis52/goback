package pbsclient

import "strings"

const (
	// taskStatusStopped is the status PBS reports for a task that has finished.
	taskStatusStopped = "stopped"

	// exitStatusOK is the exit status of a task that finished without problems.
	exitStatusOK = "OK"

	// exitStatusWarningsPrefix prefixes the exit status of a task that finished
	// but logged warnings, e.g. "WARNINGS: 2".
	exitStatusWarningsPrefix = "WARNINGS"
)

// TaskID identifies a PBS task (a UPID such as
// "UPID:pbs:00001234:0000ABCD:00000001:6900A1B2:prunejob:goback-prune:root@pam:").
type TaskID string

// TaskStatus is the status of a PBS task.
// See: GET /api2/json/nodes/{node}/tasks/{upid}/status
type TaskStatus struct {
	UPID       string `json:"upid"`
	Status     string `json:"status"`     // "running" or "stopped"
	ExitStatus string `json:"exitstatus"` // set once stopped: "OK", "WARNINGS: n", or an error message
	StartTime  int64  `json:"starttime"`
	EndTime    int64  `json:"endtime,omitempty"`
	WorkerType string `json:"worker_type"` // e.g. "prunejob", "garbage_collection"
	WorkerID   string `json:"worker_id"`   // e.g. the prune job ID or the datastore name
	User       string `json:"user"`
}

// Finished reports whether the task has stopped running.
func (s *TaskStatus) Finished() bool {
	return s.Status == taskStatusStopped
}

// Succeeded reports whether the task finished without failing. A task that
// finished with warnings counts as succeeded; use HasWarnings to tell the two
// apart.
func (s *TaskStatus) Succeeded() bool {
	return s.Finished() && (s.ExitStatus == exitStatusOK || s.HasWarnings())
}

// HasWarnings reports whether the task finished with warnings.
func (s *TaskStatus) HasWarnings() bool {
	return strings.HasPrefix(s.ExitStatus, exitStatusWarningsPrefix)
}

// taskResponse is the response from an endpoint that starts a task.
type taskResponse struct {
	Data string `json:"data"`
}
