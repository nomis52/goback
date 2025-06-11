package ipmi

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

const IPMI_TOOL = "ipmitool"

// Option is a function that configures an IPMIController
type Option func(*IPMIController)

// WithLogger sets a custom logger for the IPMIController
func WithLogger(logger *slog.Logger) Option {
	return func(c *IPMIController) {
		c.logger = logger
	}
}

// WithUsername sets the username for the IPMIController
func WithUsername(username string) Option {
	return func(c *IPMIController) {
		c.username = username
	}
}

// WithPassword sets the password for the IPMIController
func WithPassword(password string) Option {
	return func(c *IPMIController) {
		c.password = password
	}
}

// IPMIController manages IPMI operations
type IPMIController struct {
	host     string
	username string
	password string
	logger   *slog.Logger
}

// NewIPMIController creates a new IPMI controller
func NewIPMIController(host string, opts ...Option) *IPMIController {
	c := &IPMIController{
		host:   host,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Status returns the current status of the remote system
func (c *IPMIController) Status() (PowerState, error) {
	output, err := c.runIPMICommand("chassis", "status")
	if err != nil {
		return PowerStateUnknown, fmt.Errorf("failed to get chassis status: %v", err)
	}

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
	output, err := cmd.CombinedOutput()

	// Log command details (with redacted password)
	logArgs := []string{"-H", c.host, "-U", c.username, "-P", "[REDACTED]"}
	logArgs = append(logArgs, args...)

	if err != nil {
		c.logger.Error("IPMI command failed",
			"command", IPMI_TOOL,
			"args", logArgs,
			"error", err,
			"output", string(output))
		return output, err
	}

	c.logger.Debug("IPMI command succeeded",
		"command", IPMI_TOOL,
		"args", logArgs,
		"output", string(output))

	return output, nil
}
