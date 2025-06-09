package ipmi

import "testing"

func TestParsePowerState(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PowerState
	}{
		{
			name:     "on state",
			input:    "on",
			expected: PowerStateOn,
		},
		{
			name:     "off state",
			input:    "off",
			expected: PowerStateOff,
		},
		{
			name:     "soft-off state",
			input:    "soft-off",
			expected: PowerStateSoftOff,
		},
		{
			name:     "cycling state",
			input:    "cycling",
			expected: PowerStateCycling,
		},
		{
			name:     "fault state",
			input:    "fault",
			expected: PowerStateFault,
		},
		{
			name:     "unknown state",
			input:    "unknown",
			expected: PowerStateUnknown,
		},
		{
			name:     "uppercase input",
			input:    "ON",
			expected: PowerStateOn,
		},
		{
			name:     "mixed case input",
			input:    "SoFt-OfF",
			expected: PowerStateSoftOff,
		},
		{
			name:     "input with whitespace",
			input:    "  off  ",
			expected: PowerStateOff,
		},
		{
			name:     "input with leading/trailing whitespace",
			input:    "\t\n on \t\n",
			expected: PowerStateOn,
		},
		{
			name:     "empty string",
			input:    "",
			expected: PowerStateUnknown,
		},
		{
			name:     "invalid state",
			input:    "invalid",
			expected: PowerStateUnknown,
		},
		{
			name:     "random string",
			input:    "xyz123",
			expected: PowerStateUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePowerState(tt.input)
			if result != tt.expected {
				t.Errorf("ParsePowerState(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
