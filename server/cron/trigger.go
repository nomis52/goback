// Package cron provides cron-based scheduling for triggering backup runs.
//
// The CronTrigger type executes a callback according to a cron schedule.
// It is designed to be started once and run until the context is cancelled.
//
// For managing multiple triggers with different workflows and schedules, use CronTriggerManager.
//
// Example usage:
//
//	callback := func() error {
//	    return runner.Run([]string{"backup", "poweroff"})
//	}
//	trigger, err := cron.NewCronTrigger("0 2 * * *", callback, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	trigger.Start(ctx)  // Returns immediately, runs in background
//	<-ctx.Done()        // Wait for shutdown signal
package cron

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// ErrInvalidCronSpec is returned when the cron specification cannot be parsed.
var ErrInvalidCronSpec = errors.New("invalid cron spec")

// CronTrigger executes a callback according to a cron schedule.
type CronTrigger struct {
	spec     string
	schedule cron.Schedule
	callback func() error
	logger   *slog.Logger
}

// NewCronTrigger creates a new CronTrigger with the given cron specification and callback.
// The spec follows standard cron format (5 fields: minute, hour, day, month, weekday).
// The callback is executed each time the trigger fires.
// Returns ErrInvalidCronSpec if the specification cannot be parsed.
func NewCronTrigger(spec string, callback func() error, logger *slog.Logger) (*CronTrigger, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec)
	if err != nil {
		return nil, errors.Join(ErrInvalidCronSpec, err)
	}

	return &CronTrigger{
		spec:     spec,
		schedule: schedule,
		callback: callback,
		logger:   logger,
	}, nil
}

// Start launches a goroutine that triggers runs according to the cron schedule.
// Returns immediately. The goroutine exits when ctx is cancelled.
func (ct *CronTrigger) Start(ctx context.Context) {
	go ct.loop(ctx)
}

// NextRun returns the next scheduled run time from now.
func (ct *CronTrigger) NextRun() time.Time {
	return ct.schedule.Next(time.Now())
}

// loop is the main scheduling loop that runs in a goroutine.
func (ct *CronTrigger) loop(ctx context.Context) {
	for {
		nextRun := ct.schedule.Next(time.Now())
		waitDuration := time.Until(nextRun)

		ct.logger.Debug("waiting for next scheduled run",
			"next_run", nextRun,
			"wait_duration", waitDuration,
		)

		select {
		case <-ctx.Done():
			ct.logger.Info("cron trigger shutting down")
			return
		case <-time.After(waitDuration):
			ct.executeRun()
		}
	}
}

// executeRun executes the callback and logs the result.
func (ct *CronTrigger) executeRun() {
	ct.logger.Info("starting scheduled run")

	if err := ct.callback(); err != nil {
		ct.logger.Warn("scheduled run completed with error", "error", err)
	} else {
		ct.logger.Info("scheduled run completed successfully")
	}
}
