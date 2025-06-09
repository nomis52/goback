package ipmi

import (
	"fmt"
	"os/exec"
	"strings"
)

const IPMI_TOOL = "ipmitool"

// PowerState represents the possible IPMI power states
type PowerState int

const (
	PowerStateUnknown PowerState = iota
	PowerStateOn
	PowerStateOff
	PowerStateSoftOff
	PowerStateCycling
	PowerStateFault
)

func (p PowerState) String() string {
	switch p {
	case PowerStateOn:
		return "on"
	case PowerStateOff:
		return "off"
	case PowerStateSoftOff:
		return "soft-off"
	case PowerStateCycling:
		return "cycling"
	case PowerStateFault:
		return "fault"
	default:
		return "unknown"
	}
}

// ParsePowerState converts a string to a PowerState enum
func ParsePowerState(state string) PowerState {
	state = strings.ToLower(strings.TrimSpace(state))
	switch state {
	case "on":
		return PowerStateOn
	case "off":
		return PowerStateOff
	case "soft-off":
		return PowerStateSoftOff
	case "cycling":
		return PowerStateCycling
	case "fault":
		return PowerStateFault
	default:
		return PowerStateUnknown
	}
}

// IPMIController manages IPMI operations
type IPMIController struct {
	host     string
	username string
	password string
}

// NewIPMIController creates a new IPMI controller
func NewIPMIController(host, username, password string) *IPMIController {
	return &IPMIController{
		host:     host,
		username: username,
		password: password,
	}
}

// Status returns the current status of the remote system
func (c *IPMIController) Status() (PowerState, error) {
	// Get chassis status
	output, err := c.runIPMICommand("chassis", "status")
	if err != nil {
		return PowerStateUnknown, fmt.Errorf("failed to get chassis status: %v", err)
	}

	// Parse power state from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "System Power") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return ParsePowerState(strings.TrimSpace(parts[1])), nil
			}
		}
	}
	return PowerStateUnknown, fmt.Errorf("failed to read status")
}

// PowerOn turns on the remote system
func (c *IPMIController) PowerOn() error {
	_, err := c.runIPMICommand("chassis", "power", "on")
	if err != nil {
		return fmt.Errorf("failed to power on system: %v", err)
	}
	return nil
}

// PowerOff turns off the remote system
func (c *IPMIController) PowerOff() error {
	_, err := c.runIPMICommand("chassis", "power", "off")
	if err != nil {
		return fmt.Errorf("failed to power off system: %v", err)
	}
	return nil
}

// Reset resets the remote system
func (c *IPMIController) Reset() error {
	_, err := c.runIPMICommand("chassis", "power", "reset")
	if err != nil {
		return fmt.Errorf("failed to reset system: %v", err)
	}
	return nil
}

// runIPMICommand executes an IPMI command with the configured credentials
func (c *IPMIController) runIPMICommand(args ...string) ([]byte, error) {
	cmdArgs := []string{"-H", c.host, "-U", c.username, "-P", c.password}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(IPMI_TOOL, cmdArgs...)
	return cmd.CombinedOutput()
}
