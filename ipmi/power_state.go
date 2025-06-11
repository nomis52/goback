package ipmi

import "strings"

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
