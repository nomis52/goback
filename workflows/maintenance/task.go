package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/pbsclient"
)

const (
	// taskPollInterval is how often a running PBS task's status is checked.
	taskPollInterval = 10 * time.Second

	// statusUpdateInterval is how often the status line is refreshed with the
	// elapsed time while a task runs, so a multi-hour GC doesn't look stuck.
	statusUpdateInterval = 30 * time.Second
)

// runTask starts a PBS task with start and waits for it to finish, giving up after
// timeout. label names the task in status lines, logs and errors (e.g. "prune job
// goback-prune"). It returns the task's final status and how long it took. The
// error is non-nil when the task could not be started, did not finish in time, or
// finished unsuccessfully; a task that finished with warnings counts as success
// and is logged. Giving up on the wait does not stop the task on PBS.
func runTask(
	ctx context.Context,
	client *pbsclient.Client,
	statusLine *activity.StatusLine,
	logger *slog.Logger,
	label string,
	timeout time.Duration,
	start func(context.Context) (pbsclient.TaskID, error),
) (*pbsclient.TaskStatus, time.Duration, error) {
	statusLine.Set(fmt.Sprintf("starting %s", label))

	taskID, err := start(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to start %s: %w", label, err)
	}
	began := time.Now()
	logger.Info("PBS task started", "task", label, "upid", taskID, "timeout", timeout)
	statusLine.Set(fmt.Sprintf("%s running", label))

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Keep the status line showing how long the task has been running.
	stopUpdates := make(chan struct{})
	updatesDone := make(chan struct{})
	go func() {
		defer close(updatesDone)
		ticker := time.NewTicker(statusUpdateInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopUpdates:
				return
			case <-ticker.C:
				statusLine.Set(fmt.Sprintf("%s running for %s", label, time.Since(began).Round(time.Second)))
			}
		}
	}()

	status, err := client.WaitForTask(waitCtx, taskID, taskPollInterval)
	close(stopUpdates)
	<-updatesDone
	elapsed := time.Since(began)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, elapsed, fmt.Errorf("%s did not finish within %s; task %s keeps running on PBS", label, timeout, taskID)
		}
		return nil, elapsed, fmt.Errorf("waiting for %s (task %s): %w", label, taskID, err)
	}

	if !status.Succeeded() {
		return status, elapsed, fmt.Errorf("%s failed after %s: %s", label, elapsed.Round(time.Second), status.ExitStatus)
	}

	if status.HasWarnings() {
		logger.Warn("PBS task finished with warnings", "task", label, "upid", taskID, "exit_status", status.ExitStatus, "duration", elapsed)
	} else {
		logger.Info("PBS task finished", "task", label, "upid", taskID, "exit_status", status.ExitStatus, "duration", elapsed)
	}

	return status, elapsed, nil
}
