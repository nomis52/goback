package activities

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/orchestrator"
	"github.com/nomis52/goback/pbsclient"
)

const (
	pingCheckInterval = 5 * time.Second
)

// PowerOnPBS manages the power state of the PBS host through IPMI
type PowerOnPBS struct {
	// Dependencies
	Controller *ipmi.IPMIController
	PBSClient  *pbsclient.Client
	Logger     *slog.Logger

	// Config
	BootTimeout time.Duration `config:"pbs.boot_timeout"`
}

func (a *PowerOnPBS) Init() error {
	return nil
}

func (a *PowerOnPBS) Run(ctx context.Context) (orchestrator.Result, error) {
	// Check current power status
	status, err := a.Controller.Status()
	if err != nil {
		return orchestrator.NewFailureResult(), fmt.Errorf("failed to get power status: %w", err)
	}
	a.Logger.Debug("current power status", "status", status)

	// If power is off, turn it on
	if status == ipmi.PowerStateOff {
		if err := a.Controller.PowerOn(); err != nil {
			a.Logger.Error("failed to power on PBS host", "error", err)
			return orchestrator.NewFailureResult(), fmt.Errorf("failed to power on PBS host: %w", err)
		}
		a.Logger.Info("power on command sent successfully")
	} else {
		a.Logger.Info("PBS host is already powered on", "status", status)
	}

	// Wait for PBS to be available
	a.Logger.Info("waiting for PBS to become available", "timeout", a.BootTimeout)
	ticker := time.NewTicker(pingCheckInterval)
	defer ticker.Stop()

	timeout := time.After(a.BootTimeout)
	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return orchestrator.NewFailureResult(), fmt.Errorf("context cancelled while waiting for PBS: %w", ctx.Err())
		case <-timeout:
			return orchestrator.NewFailureResult(), fmt.Errorf("timed out waiting for PBS to become available after %v", a.BootTimeout)
		case <-ticker.C:
			attempts++
			_, err := a.PBSClient.Ping()
			if err == nil {
				a.Logger.Info("PBS is now available", "attempts", attempts)
				return orchestrator.NewSuccessResult(), nil
			}
			a.Logger.Debug("PBS not yet available", "attempt", attempts, "error", err)
		}
	}
}
