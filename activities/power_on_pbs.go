package activities

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/pbsclient"
	"github.com/nomis52/goback/statusreporter"
)

const (
	pingCheckInterval = 5 * time.Second
)

// PowerOnPBS manages the power state of the PBS host through IPMI
type PowerOnPBS struct {
	// Dependencies
	Controller     *ipmi.IPMIController
	PBSClient      *pbsclient.Client
	Logger         *slog.Logger
	StatusReporter *statusreporter.StatusReporter

	BootTimeout     time.Duration `config:"pbs.boot_timeout"`
	ServiceWaitTime time.Duration `config:"pbs.service_wait_time"`
}

func (a *PowerOnPBS) Init() error {
	return nil
}

func (a *PowerOnPBS) Execute(ctx context.Context) error {
	return RecordError(a, a.StatusReporter, func() error {
		a.StatusReporter.SetStatus(a, "checking PBS power status")

		// Check current power status
		status, err := a.Controller.Status()
		if err != nil {
			return fmt.Errorf("failed to get power status: %w", err)
		}
		a.Logger.Debug("current PBS power status", "status", status)

		// If power is off, turn it on
		if status == ipmi.PowerStateOff {
			a.StatusReporter.SetStatus(a, "sending IPMI power on command")
			if err := a.Controller.PowerOn(); err != nil {
				a.Logger.Error("failed to power on PBS host", "error", err)
				return fmt.Errorf("failed to power on PBS host: %w", err)
			}
		} else {
			a.Logger.Debug("PBS host is already powered on", "status", status)
			// Do an immediate ping check since we know it's powered on
			if _, err := a.PBSClient.Ping(); err == nil {
				a.StatusReporter.SetStatus(a, "PBS server is online")
				return nil // Success!
			}
		}

		// Wait for PBS to be available
		a.StatusReporter.SetStatus(a, "waiting for PBS server to become available")
		ticker := time.NewTicker(pingCheckInterval)
		defer ticker.Stop()

		timeout := time.After(a.BootTimeout)
		attempts := 0
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for PBS: %w", ctx.Err())
			case <-timeout:
				return fmt.Errorf("timed out waiting for PBS to become available after %v", a.BootTimeout)
			case <-ticker.C:
				attempts++
				_, err := a.PBSClient.Ping()
				if err == nil {
					a.StatusReporter.SetStatus(a, "PBS ping passed, waiting for PBS services to stabilize")
					a.Logger.Debug("PBS ping successful", "attempts", attempts, "wait_time", a.ServiceWaitTime)

					// Give PBS additional time for all services to fully start
					select {
					case <-ctx.Done():
						return fmt.Errorf("context cancelled while waiting for PBS services: %w", ctx.Err())
					case <-time.After(a.ServiceWaitTime):
						a.StatusReporter.SetStatus(a, "PBS server is online")
						return nil // Success!
					}
				}
				a.Logger.Debug("PBS not yet available", "attempt", attempts, "error", err)
			}
		}
	})
}
