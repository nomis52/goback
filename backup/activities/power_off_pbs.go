// Package activities provides orchestration activities for the PBS backup automation system.
//
// PowerOffPBS Activity:
//
// The PowerOffPBS activity handles the graceful shutdown of the Proxmox Backup Server (PBS)
// after backup operations are complete. It implements a clean two-stage IPMI-based shutdown:
//
// 1. Graceful IPMI Shutdown:
//   - Uses IPMI "chassis power soft" to send ACPI shutdown signal to PBS
//   - Monitors IPMI power status for shutdown completion
//   - This is equivalent to pressing the power button gently
//
// 2. Hard IPMI Power-off (if timeout):
//   - If graceful shutdown doesn't complete within timeout, forces hard power-off
//   - Uses IPMI "chassis power off" for immediate shutdown
//   - Equivalent to holding the power button or pulling the plug
//
// Monitoring Logic:
//   - After graceful shutdown command, continuously checks IPMI power status
//   - If IPMI reports "off", shutdown is complete (success)
//   - If timeout expires and still "on", forces hard power-off
//
// Advantages over SSH approach:
//   - No network connectivity dependencies
//   - No SSH configuration or authentication required
//   - Works even if OS is unresponsive or SSH service is down
//   - Single interface for all power management operations
//   - Hardware-level reliability through BMC
//
// Configuration Requirements:
//   - timeouts.shutdown_timeout: Maximum time to wait for graceful shutdown
//   - IPMI controller configuration (host, username, password)
//
// Dependencies:
//   - Must run after BackupDirs and BackupVMs activities complete
//   - Requires IPMI controller for all power operations
//   - No SSH or PBS client dependencies needed
//
// Error Handling:
//   - IPMI graceful shutdown failures trigger immediate hard power-off
//   - Shutdown timeout triggers hard power-off
//   - Already powered-off systems are handled gracefully
//   - All operations use hardware-level IPMI commands
package activities

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomis52/goback/clients/ipmiclient"
	"github.com/nomis52/goback/statusreporter"
)

const (
	shutdownCheckInterval = 5 * time.Second
)

// PowerOffPBS manages the graceful shutdown of the PBS host via IPMI commands.
//
// This activity ensures that the PBS server is gracefully powered down after
// backup operations complete, reducing power consumption and wear on the hardware.
// It depends on BackupDirs and BackupVMs activities to ensure it only runs
// after all backup operations have finished.
//
// The shutdown process uses pure IPMI commands for maximum reliability,
// with graceful ACPI shutdown as primary method and hard power-off as fallback.
type PowerOffPBS struct {
	// Dependencies
	Controller     *ipmiclient.IPMIController
	Logger         *slog.Logger
	StatusReporter *statusreporter.StatusReporter

	// Activity dependencies - these ensure PowerOffPBS runs after backup activities complete
	_ *BackupDirs
	_ *BackupVMs

	// Configuration
	ShutdownTimeout time.Duration `config:"pbs.shutdown_timeout"`
}

// Init initializes the PowerOffPBS activity.
func (a *PowerOffPBS) Init() error {
	return nil
}

// Execute performs the PBS shutdown process using pure IPMI commands.
//
// The execution follows this sequence:
//  1. Check if PBS is already powered off (early return if so)
//  2. Send IPMI graceful shutdown command ("chassis power soft")
//  3. Monitor IPMI power status until shutdown completes or timeout
//  4. Fall back to IPMI hard power-off if graceful shutdown times out
//
// This approach provides maximum reliability by using only hardware-level
// IPMI commands, eliminating network and SSH dependencies.
func (a *PowerOffPBS) Execute(ctx context.Context) error {
	return RecordError(a, a.StatusReporter, func() error {
		a.StatusReporter.SetStatus(a, "checking PBS power status")

		// Check current power status first
		status, err := a.Controller.Status()
		if err != nil {
			a.Logger.Warn("failed to get initial power status", "error", err)
		} else if status == ipmiclient.PowerStateOff {
			a.StatusReporter.SetStatus(a, "PBS server already powered off")
			return nil
		}

		// Attempt graceful shutdown via IPMI ACPI signal
		a.StatusReporter.SetStatus(a, "sending graceful shutdown signal")
		if err := a.gracefulIPMIShutdown(); err != nil {
			a.Logger.Warn("graceful IPMI shutdown failed, falling back to hard power-off", "error", err)
			a.StatusReporter.SetStatus(a, "forcing hard power off")
			return a.hardIPMIPowerOff()
		}

		// Wait for system to shutdown by monitoring IPMI power status
		a.StatusReporter.SetStatus(a, "waiting for PBS server to shut down")
		if err := a.waitForShutdownViaIPMI(ctx); err != nil {
			a.Logger.Warn("graceful shutdown timed out, forcing hard power-off via IPMI", "error", err)
			a.StatusReporter.SetStatus(a, "forcing hard power off")
			return a.hardIPMIPowerOff()
		}

		a.StatusReporter.SetStatus(a, "PBS server powered off")
		return nil
	})
}

// gracefulIPMIShutdown sends a graceful shutdown signal via IPMI ACPI
func (a *PowerOffPBS) gracefulIPMIShutdown() error {
	if err := a.Controller.PowerOff(); err != nil {
		return fmt.Errorf("failed to send IPMI graceful shutdown signal: %w", err)
	}

	return nil
}

// waitForShutdownViaIPMI monitors IPMI power status until PBS shuts down or timeout
func (a *PowerOffPBS) waitForShutdownViaIPMI(ctx context.Context) error {
	a.Logger.Debug("monitoring PBS shutdown via IPMI power status", "timeout", a.ShutdownTimeout)

	ticker := time.NewTicker(shutdownCheckInterval)
	defer ticker.Stop()

	timeout := time.After(a.ShutdownTimeout)
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for PBS shutdown: %w", ctx.Err())
		case <-timeout:
			return fmt.Errorf("timed out waiting for PBS to shutdown after %v", a.ShutdownTimeout)
		case <-ticker.C:
			attempts++

			// Check IPMI power status
			status, err := a.Controller.Status()
			if err != nil {
				a.Logger.Debug("IPMI status check failed", "attempt", attempts, "error", err)
				continue // Keep trying
			}

			a.Logger.Debug("IPMI power status check", "status", status, "attempt", attempts)

			// Check if PBS has powered off
			if status == ipmiclient.PowerStateOff {
				a.Logger.Debug("PBS shutdown completed successfully", "attempts", attempts)
				return nil
			}

			// Log current status for debugging
			a.Logger.Debug("PBS still powered on, continuing to wait", "status", status, "attempt", attempts)
		}
	}
}

// hardIPMIPowerOff performs an immediate hard power off via IPMI
func (a *PowerOffPBS) hardIPMIPowerOff() error {
	a.Logger.Warn("performing hard power-off via IPMI")

	if err := a.Controller.PowerOffHard(); err != nil {
		return fmt.Errorf("failed to hard power-off PBS host via IPMI: %w", err)
	}

	return nil
}
