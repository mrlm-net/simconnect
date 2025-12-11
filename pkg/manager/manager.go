//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/engine"

// Manager defines the interface for managing SimConnect connections with
// automatic lifecycle handling and reconnection support
// MessageHandler is a callback function invoked when a message is received from the simulator
type MessageHandler func(msg engine.Message)

type Manager interface {
	// Start begins the connection lifecycle management.
	// It will attempt to connect to the simulator and automatically
	// reconnect if the connection is lost (when AutoReconnect is enabled).
	// This method blocks until the context is cancelled or Stop is called.
	Start() error

	// Stop gracefully shuts down the manager and disconnects from the simulator
	Stop() error

	// State returns the current connection state
	State() ConnectionState

	// OnStateChange registers a callback to be invoked when connection state changes
	OnStateChange(handler StateChangeHandler)

	// OnMessage registers a callback to be invoked when a message is received.
	// This allows handling events, data, and other messages while the manager
	// handles connection lifecycle automatically.
	OnMessage(handler MessageHandler)

	// Client returns the underlying engine client for direct API access.
	// Returns nil if not connected.
	Client() engine.Client
}
