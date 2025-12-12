//go:build windows
// +build windows

package manager

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

// Manager defines the interface for managing SimConnect connections with
// automatic lifecycle handling and reconnection support
// MessageHandler is a callback function invoked when a message is received from the simulator
type MessageHandler func(msg engine.Message)

// Subscription represents an active message subscription that can be cancelled
type Subscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// Messages returns the channel for receiving messages
	Messages() <-chan engine.Message

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending message delivery completes.
	Unsubscribe()
}

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

	// Subscribe creates a new message subscription that delivers messages to a channel.
	// The returned Subscription can be used to receive messages in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	Subscribe(id string, bufferSize int) Subscription

	// GetSubscription returns an existing subscription by ID, or nil if not found.
	GetSubscription(id string) Subscription

	// Client returns the underlying engine client for direct API access.
	// Returns nil if not connected.
	Client() engine.Client

	// Configuration getters

	// IsAutoReconnect returns whether automatic reconnection is enabled
	IsAutoReconnect() bool

	// RetryInterval returns the delay between connection attempts
	RetryInterval() time.Duration

	// ConnectionTimeout returns the timeout for each connection attempt
	ConnectionTimeout() time.Duration

	// ReconnectDelay returns the delay before reconnecting after disconnect
	ReconnectDelay() time.Duration

	// ShutdownTimeout returns the timeout for graceful shutdown of subscriptions
	ShutdownTimeout() time.Duration

	// MaxRetries returns the maximum number of connection retries (0 = unlimited)
	MaxRetries() int
}
