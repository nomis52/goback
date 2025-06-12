package activities

import (
	"context"
	"fmt"

	"github.com/nomis52/goback/ipmi"
	"github.com/nomis52/goback/orchestrator"
)

// PowerOnPBS manages the power state of the PBS host through IPMI
type PowerOnPBS struct {
	// Dependencies
	Controller *ipmi.IPMIController

	// Config
	Host string `config:"ipmi.host"`

	// Internal state
	initialized bool
	executed    bool
}

func (a *PowerOnPBS) Init() error {
	if a.Controller == nil {
		return fmt.Errorf("IPMI controller not injected")
	}
	if a.Host == "" {
		return fmt.Errorf("IPMI host not configured")
	}
	a.initialized = true
	return nil
}

func (a *PowerOnPBS) Run(ctx context.Context) (orchestrator.Result, error) {
	if !a.initialized {
		return orchestrator.NewFailureResult(), fmt.Errorf("not initialized")
	}

	// Check current power status
	status, err := a.Controller.Status()
	if err != nil {
		return orchestrator.NewFailureResult(), fmt.Errorf("failed to get power status: %w", err)
	}

	// If power is off, turn it on
	if status == ipmi.PowerStateOff {
		if err := a.Controller.PowerOn(); err != nil {
			return orchestrator.NewFailureResult(), fmt.Errorf("failed to power on PBS host: %w", err)
		}
	}

	a.executed = true
	return orchestrator.NewSuccessResult(), nil
}
