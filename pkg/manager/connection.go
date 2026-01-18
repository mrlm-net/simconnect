//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/types"

// StateChange represents a connection state transition event
type ConnectionStateChange struct {
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

// ConnectionStateChangeHandler is a callback function invoked when connection state changes
type ConnectionStateChangeHandler func(oldState, newState ConnectionState)

// ConnectionStateSubscription represents an active state change subscription that can be cancelled
type ConnectionStateSubscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// StateChanges returns the channel for receiving state change events
	ConnectionStateChanges() <-chan ConnectionStateChange

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending state change delivery completes.
	Unsubscribe()
}

// ConnectionOpenHandler is a callback function invoked when connection opens
type ConnectionOpenHandler func(data types.ConnectionOpenData)

// ConnectionQuitHandler is a callback function invoked when connection quits
type ConnectionQuitHandler func(data types.ConnectionQuitData)

// ConnectionOpenSubscription represents an active connection open subscription that can be cancelled
type ConnectionOpenSubscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// Opens returns the channel for receiving connection open events
	Opens() <-chan types.ConnectionOpenData

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending change delivery completes.
	Unsubscribe()
}

// ConnectionQuitSubscription represents an active connection quit subscription that can be cancelled
type ConnectionQuitSubscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// Quits returns the channel for receiving connection quit events
	Quits() <-chan types.ConnectionQuitData

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending change delivery completes.
	Unsubscribe()
}
