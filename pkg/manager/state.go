//go:build windows
// +build windows

package manager

// StateChange represents a connection state transition event
type StateChange struct {
	OldState ConnectionState
	NewState ConnectionState
}

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

// StateSubscription represents an active state change subscription that can be cancelled
type StateSubscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// StateChanges returns the channel for receiving state change events
	StateChanges() <-chan StateChange

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending state change delivery completes.
	Unsubscribe()
}
