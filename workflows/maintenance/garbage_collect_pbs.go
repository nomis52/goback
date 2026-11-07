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

var ErrMissingDatastore = errors.New("missing maintenance config: datastore is not set")

const (
	metricGCLastSuccess = "gc_last_success"
	metricGCLastSeconds = "gc_last_duration_seconds"
	metricGCFailure     = "gc_failure"
)

// GarbageCollectPBS runs garbage collection on a PBS datastore and waits for it to
// finish. Pruning only removes snapshot indexes; GC is what frees the chunks that
// no remaining snapshot references. It runs after PowerOnPBS so PBS is guaranteed
// to be available.
type GarbageCollectPBS struct {
	// Dependencies
	PBSClient  *pbsclient.Client
	Logger     *slog.Logger
	StatusLine *activity.StatusLine
	Registry   metrics.Registry
	_          *poweron.PowerOnPBS // Unnamed dependency: run after PBS is powered on

	// Configuration
	Token       string        `config:"maintenance.token"`
	Datastore   string        `config:"maintenance.datastore"`
	TaskTimeout time.Duration `config:"maintenance.task_timeout"`

	// Metrics (initialized in Init)
	lastSuccessGauge  metrics.GaugeVec
	lastDurationGauge metrics.GaugeVec
	failureCounter    metrics.CounterVec
}

func (a *GarbageCollectPBS) Init() error {
	var err error
	a.lastSuccessGauge, err = a.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricGCLastSuccess,
		Help: "Unix timestamp of the last successful PBS garbage collection",
	}, []string{"datastore"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricGCLastSuccess, err)
	}

	a.lastDurationGauge, err = a.Registry.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricGCLastSeconds,
		Help: "Duration in seconds of the last successful PBS garbage collection",
	}, []string{"datastore"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricGCLastSeconds, err)
	}

	a.failureCounter, err = a.Registry.NewCounterVec(prometheus.CounterOpts{
		Name: metricGCFailure,
		Help: "Count of PBS garbage collections that failed or timed out",
	}, []string{"datastore"})
	if err != nil {
		return fmt.Errorf("creating %s metric: %w", metricGCFailure, err)
	}

	if a.Datastore == "" {
		return ErrMissingDatastore
	}
	if a.Token == "" {
		return ErrMissingToken
	}
	if a.TaskTimeout <= 0 {
		return fmt.Errorf("maintenance task_timeout must be positive, got %v", a.TaskTimeout)
	}

	return nil
}

func (a *GarbageCollectPBS) Execute(ctx context.Context) error {
	return activity.CaptureError(a.StatusLine, func() error {
		label := fmt.Sprintf("garbage collection on %s", a.Datastore)
		labels := prometheus.Labels{"datastore": a.Datastore}

		status, elapsed, err := runTask(ctx, a.PBSClient, a.StatusLine, a.Logger, label, a.TaskTimeout,
			func(ctx context.Context) (pbsclient.TaskID, error) {
				return a.PBSClient.StartGarbageCollection(ctx, a.Datastore)
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
