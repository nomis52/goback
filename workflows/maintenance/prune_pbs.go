package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nomis52/goback/activity"
	"github.com/nomis52/goback/clients/pbsclient"
	"github.com/nomis52/goback/metrics"
	"github.com/nomis52/goback/workflows/poweron"
)

var (
	ErrMissingPruneJob = errors.New("missing maintenance config: prune_job is not set")
	ErrMissingToken    = errors.New("missing maintenance config: token is not set")
)

const (
	metricPruneLastSuccess = "prune_last_success"
	metricPruneLastSeconds = "prune_last_duration_seconds"
	metricPruneFailure     = "prune_failure"
)

// PrunePBS runs a prune job that is configured on PBS and waits for it to finish.
// It runs after PowerOnPBS so PBS is guaranteed to be available.
//
// The job is started with the PBS API's manual-run call, which ignores the job's
// disable flag. Keeping the job disabled in PBS is the intended setup: retention
// stays in PBS config, and only goback decides when the prune happens.
type PrunePBS struct {
	// Dependencies
	PBSClient  *pbsclient.Client
	Logger     *slog.Logger
	StatusLine *activity.StatusLine
	Registry   metrics.Registry
	_          *poweron.PowerOnPBS // Unnamed dependency: run after PBS is powered on

	// Configuration
	Token       string        `config:"maintenance.token"`
	PruneJob    string        `config:"maintenance.prune_job"`
	TaskTimeout time.Duration `config:"maintenance.task_timeout"`

	// Metrics (initialized in Init)
	lastSuccessGauge  metrics.GaugeVec
	lastDurationGauge metrics.GaugeVec
	failureCounter    metrics.CounterVec
}

func (a *PrunePBS) Init() error {
	var err error
	a.lastSuccessGauge, err = a.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricPruneLastSuccess,
		Help: "Unix timestamp of the last successful PBS prune job run",
	}, []string{"job"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricPruneLastSuccess, err)
	}

	a.lastDurationGauge, err = a.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricPruneLastSeconds,
		Help: "Duration in seconds of the last successful PBS prune job run",
	}, []string{"job"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricPruneLastSeconds, err)
	}

	a.failureCounter, err = a.Registry.NewCounterVec(prometheus.CounterOpts{
		Name: metricPruneFailure,
		Help: "Count of PBS prune job runs that failed or timed out",
	}, []string{"job"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricPruneFailure, err)
	}

	if a.PruneJob == "" {
		return ErrMissingPruneJob
	}
	if a.Token == "" {
		return ErrMissingToken
	}
	if a.TaskTimeout <= 0 {
		return fmt.Errorf("maintenance task_timeout must be positive, got %v", a.TaskTimeout)
	}

	return nil
}

func (a *PrunePBS) Execute(ctx context.Context) error {
	return activity.CaptureError(a.StatusLine, func() error {
		label := fmt.Sprintf("prune job %s", a.PruneJob)
		labels := prometheus.Labels{"job": a.PruneJob}

		status, elapsed, err := runTask(ctx, a.PBSClient, a.StatusLine, a.Logger, label, a.TaskTimeout,
			func(ctx context.Context) (pbsclient.TaskID, error) {
				return a.PBSClient.RunPruneJob(ctx, a.PruneJob)
			})
		if err != nil {
			a.failureCounter.With(labels).Inc()
			return err
		}

		a.lastSuccessGauge.With(labels).Set(float64(time.Now().Unix()))
		a.lastDurationGauge.With(labels).Set(elapsed.Seconds())

		a.StatusLine.Set(fmt.Sprintf("%s finished in %s: %s", label, elapsed.Round(time.Second), status.ExitStatus))
		return nil
	})
}
