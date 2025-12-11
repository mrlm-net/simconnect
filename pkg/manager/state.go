//go:build windows
// +build windows

package manager

// ConnectionState represents the current state of the connection
type ConnectionState int

const (
	// StateDisconnected indicates no active connection to the simulator
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates a connection attempt is in progress
	StateConnecting
	// StateConnected indicates an active connection to the simulator
	StateConnected
	// StateAvailable indicates the connection is fully ready (RECV_OPEN received)
	StateAvailable
	// StateReconnecting indicates a reconnection attempt is in progress after disconnect
	StateReconnecting
)

// String returns a human-readable representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateAvailable:
		return "Available"
	case StateReconnecting:
		return "Reconnecting"
	default:
		return "Unknown"
	}
}

// StateChangeHandler is a callback function invoked when connection state changes
type StateChangeHandler func(oldState, newState ConnectionState)
