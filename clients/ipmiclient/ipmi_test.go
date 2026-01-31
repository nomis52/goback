package ipmiclient

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		cmdErr        error
		expectedState PowerState
		expectError   bool
	}{
		{
			name:          "successful status check - power on",
			output:        "System Power         : on\n",
			expectedState: PowerStateOn,
			expectError:   false,
		},
		{
			name:          "successful status check - power off",
			output:        "System Power         : off\n",
			expectedState: PowerStateOff,
			expectError:   false,
		},
		{
			name:          "command fails",
			output:        "Connection timed out",
			cmdErr:        errors.New("exit status 1"),
			expectedState: PowerStateUnknown,
			expectError:   true,
		},
		{
			name:          "malformed output",
			output:        "Something unexpected\n",
			expectedState: PowerStateUnknown,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{output: []byte(tt.output), err: tt.cmdErr}
			controller := NewIPMIController("192.168.1.100",
				WithUsername("admin"),
				WithPassword("secret"),
				withCommandRunner(mock),
			)

			state, err := controller.Status()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedState, state)
			assert.Equal(t, "ipmitool", mock.lastName)
			assert.Equal(t, []string{"-H", "192.168.1.100", "-U", "admin", "-P", "secret", "chassis", "status"}, mock.lastArgs)
		})
	}
}

func TestPowerOn(t *testing.T) {
	tests := []struct {
		name        string
		cmdErr      error
		expectError bool
	}{
		{
			name:        "successful power on",
			expectError: false,
		},
		{
			name:        "command fails",
			cmdErr:      errors.New("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{err: tt.cmdErr}
			controller := NewIPMIController("192.168.1.100",
				WithUsername("admin"),
				WithPassword("secret"),
				withCommandRunner(mock),
			)

			err := controller.PowerOn()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to power on")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, "ipmitool", mock.lastName)
			assert.Equal(t, []string{"-H", "192.168.1.100", "-U", "admin", "-P", "secret", "chassis", "power", "on"}, mock.lastArgs)
		})
	}
}

func TestPowerOff(t *testing.T) {
	tests := []struct {
		name        string
		cmdErr      error
		expectError bool
	}{
		{
			name:        "successful graceful power off",
			expectError: false,
		},
		{
			name:        "command fails",
			cmdErr:      errors.New("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{err: tt.cmdErr}
			controller := NewIPMIController("192.168.1.100",
				WithUsername("admin"),
				WithPassword("secret"),
				withCommandRunner(mock),
			)

			err := controller.PowerOff()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to gracefully power off")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, "ipmitool", mock.lastName)
			assert.Equal(t, []string{"-H", "192.168.1.100", "-U", "admin", "-P", "secret", "chassis", "power", "soft"}, mock.lastArgs)
		})
	}
}

func TestPowerOffHard(t *testing.T) {
	tests := []struct {
		name        string
		cmdErr      error
		expectError bool
	}{
		{
			name:        "successful hard power off",
			expectError: false,
		},
		{
			name:        "command fails",
			cmdErr:      errors.New("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{err: tt.cmdErr}
			controller := NewIPMIController("192.168.1.100",
				WithUsername("admin"),
				WithPassword("secret"),
				withCommandRunner(mock),
			)

			err := controller.PowerOffHard()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to hard power off")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, "ipmitool", mock.lastName)
			assert.Equal(t, []string{"-H", "192.168.1.100", "-U", "admin", "-P", "secret", "chassis", "power", "off"}, mock.lastArgs)
		})
	}
}

func TestReset(t *testing.T) {
	tests := []struct {
		name        string
		cmdErr      error
		expectError bool
	}{
		{
			name:        "successful reset",
			expectError: false,
		},
		{
			name:        "command fails",
			cmdErr:      errors.New("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommandRunner{err: tt.cmdErr}
			controller := NewIPMIController("192.168.1.100",
				WithUsername("admin"),
				WithPassword("secret"),
				withCommandRunner(mock),
			)

			err := controller.Reset()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to reset")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, "ipmitool", mock.lastName)
			assert.Equal(t, []string{"-H", "192.168.1.100", "-U", "admin", "-P", "secret", "chassis", "power", "reset"}, mock.lastArgs)
		})
	}
}

func TestParseChassisStatus(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedState PowerState
		expectError   bool
	}{
		{
			name: "typical chassis status output - power on",
			output: `System Power         : on
Power Overload       : false
Power Interlock      : inactive
Main Power Fault     : false
Power Control Fault  : false
Power Restore Policy : always-off
Last Power Event     :
Chassis Intrusion    : inactive
Front-Panel Lockout  : inactive
Drive Fault          : false
Cooling/Fan Fault    : false`,
			expectedState: PowerStateOn,
			expectError:   false,
		},
		{
			name: "typical chassis status output - power off",
			output: `System Power         : off
Power Overload       : false
Power Interlock      : inactive
Main Power Fault     : false
Power Control Fault  : false
Power Restore Policy : always-off
Last Power Event     :
Chassis Intrusion    : inactive
Front-Panel Lockout  : inactive
Drive Fault          : false
Cooling/Fan Fault    : false`,
			expectedState: PowerStateOff,
			expectError:   false,
		},
		{
			name:          "minimal output - just system power line",
			output:        "System Power         : on\n",
			expectedState: PowerStateOn,
			expectError:   false,
		},
		{
			name:          "system power line without trailing newline",
			output:        "System Power: off",
			expectedState: PowerStateOff,
			expectError:   false,
		},
		{
			name:          "system power line with extra whitespace",
			output:        "  System Power  :  on  \n",
			expectedState: PowerStateOn,
			expectError:   false,
		},
		{
			name:          "soft-off state",
			output:        "System Power         : soft-off\n",
			expectedState: PowerStateSoftOff,
			expectError:   false,
		},
		{
			name:          "cycling state",
			output:        "System Power         : cycling\n",
			expectedState: PowerStateCycling,
			expectError:   false,
		},
		{
			name:          "fault state",
			output:        "System Power         : fault\n",
			expectedState: PowerStateFault,
			expectError:   false,
		},
		{
			name:          "unknown power state string",
			output:        "System Power         : something-unexpected\n",
			expectedState: PowerStateUnknown,
			expectError:   false,
		},
		{
			name:          "empty output",
			output:        "",
			expectedState: PowerStateUnknown,
			expectError:   true,
		},
		{
			name:          "no system power line",
			output:        "Power Overload       : false\nPower Interlock      : inactive\n",
			expectedState: PowerStateUnknown,
			expectError:   true,
		},
		{
			name:          "system power line without colon",
			output:        "System Power on\n",
			expectedState: PowerStateUnknown,
			expectError:   true,
		},
		{
			name:          "system power line with empty value",
			output:        "System Power:\n",
			expectedState: PowerStateUnknown,
			expectError:   false,
		},
		{
			name: "system power not at start of line",
			output: `Checking chassis status...
System Power         : on
Done.`,
			expectedState: PowerStateOn,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := parseChassisStatus([]byte(tt.output))

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedState, state)
		})
	}
}

// Test helpers

type mockCommandRunner struct {
	output    []byte
	err       error
	lastName  string
	lastArgs  []string
	callCount int
}

func (m *mockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	m.lastName = name
	m.lastArgs = args
	m.callCount++
	return m.output, m.err
}

func withCommandRunner(runner CommandRunner) Option {
	return func(c *IPMIController) {
		c.cmdRunner = runner
	}
}
