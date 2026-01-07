// Package cron provides cron-based scheduling for triggering backup runs.
//
// The CronTrigger type wraps a Runnable and executes it according to a cron schedule.
// It is designed to be started once and run until the context is cancelled.
//
// Example usage:
//
//	trigger, err := cron.NewCronTrigger("0 2 * * *", runner, logger)
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

// Runnable is implemented by anything that can be triggered by the cron scheduler.
type Runnable interface {
	Run() error
}

// CronTrigger executes a Runnable according to a cron schedule.
type CronTrigger struct {
	spec     string
	schedule cron.Schedule
	runnable Runnable
	logger   *slog.Logger
}

// NewCronTrigger creates a new CronTrigger with the given cron specification.
// The spec follows standard cron format (5 fields: minute, hour, day, month, weekday).
// Returns ErrInvalidCronSpec if the specification cannot be parsed.
func NewCronTrigger(spec string, runnable Runnable, logger *slog.Logger) (*CronTrigger, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec)
	if err != nil {
		return nil, errors.Join(ErrInvalidCronSpec, err)
	}

	return &CronTrigger{
		spec:     spec,
		schedule: schedule,
		runnable: runnable,
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

// executeRun executes the runnable and logs the result.
func (ct *CronTrigger) executeRun() {
	ct.logger.Info("starting scheduled backup run")

	if err := ct.runnable.Run(); err != nil {
		ct.logger.Warn("scheduled run completed with error", "error", err)
	} else {
		ct.logger.Info("scheduled run completed successfully")
	}
}
