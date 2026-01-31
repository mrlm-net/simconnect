//go:build windows
// +build windows

package manager

import (
	"errors"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNotConnected is returned when an operation requires an active connection
// but the manager is not currently connected to the simulator.
var ErrNotConnected = errors.New("manager: not connected to simulator")

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
	ConnectionState() ConnectionState

	// OnConnectionStateChange registers a callback to be invoked when connection state changes
	// Returns a unique id that can be used to remove the handler via RemoveConnectionStateChange.
	OnConnectionStateChange(handler ConnectionStateChangeHandler) string

	// OnMessage registers a callback to be invoked when a message is received.
	// Returns a unique id that can be used to remove the handler via RemoveMessage.
	// This allows handling events, data, and other messages while the manager
	// handles connection lifecycle automatically.
	OnMessage(handler MessageHandler) string

	// Subscribe creates a new message subscription that delivers messages to a channel.
	// The returned Subscription can be used to receive messages in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	Subscribe(id string, bufferSize int) Subscription

	// SubscribeWithFilter creates a new message subscription that only forwards
	// messages for which the provided filter function returns true.
	SubscribeWithFilter(id string, bufferSize int, filter func(engine.Message) bool) Subscription

	// SubscribeWithType creates a new message subscription that only forwards
	// messages whose `DwID` matches one of the provided SIMCONNECT_RECV_ID values.
	SubscribeWithType(id string, bufferSize int, recvIDs ...types.SIMCONNECT_RECV_ID) Subscription

	// GetSubscription returns an existing subscription by ID, or nil if not found.
	GetSubscription(id string) Subscription

	// SubscribeStateChange creates a new state change subscription that delivers state changes to a channel.
	// The returned StateSubscription can be used to receive state changes in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	SubscribeConnectionStateChange(id string, bufferSize int) ConnectionStateSubscription

	// GetStateSubscription returns an existing state subscription by ID, or nil if not found.
	GetConnectionStateSubscription(id string) ConnectionStateSubscription

	// SimState returns the current simulator state
	SimState() SimState

	// OnSimStateChange registers a callback to be invoked when simulator state changes.
	// Returns a unique id that can be used to remove the handler via RemoveSimStateChange.
	OnSimStateChange(handler SimStateChangeHandler) string

	// RemoveSimStateChange removes a previously registered simulator state change handler by id.
	// Returns an error if the id is unknown.
	RemoveSimStateChange(id string) error

	// SubscribeSimStateChange creates a new simulator state change subscription that delivers state changes to a channel.
	// The returned SimStateSubscription can be used to receive state changes in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	SubscribeSimStateChange(id string, bufferSize int) SimStateSubscription

	// GetSimStateSubscription returns an existing sim state subscription by ID, or nil if not found.
	GetSimStateSubscription(id string) SimStateSubscription

	// OnOpen registers a callback to be invoked when the simulator connection opens.
	// Returns a unique id that can be used to remove the handler via RemoveOpen.
	OnOpen(handler ConnectionOpenHandler) string

	// RemoveOpen removes a previously registered open handler by id.
	// Returns an error if the id is unknown.
	RemoveOpen(id string) error

	// SubscribeOnOpen creates a new connection open subscription that delivers open events to a channel.
	// The returned ConnectionOpenSubscription can be used to receive open events in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	SubscribeOnOpen(id string, bufferSize int) ConnectionOpenSubscription

	// GetOpenSubscription returns an existing open subscription by ID, or nil if not found.
	GetOpenSubscription(id string) ConnectionOpenSubscription

	// OnQuit registers a callback to be invoked when the simulator quits.
	// Returns a unique id that can be used to remove the handler via RemoveQuit.
	OnQuit(handler ConnectionQuitHandler) string

	// RemoveQuit removes a previously registered quit handler by id.
	// Returns an error if the id is unknown.
	RemoveQuit(id string) error

	// SubscribeOnQuit creates a new connection quit subscription that delivers quit events to a channel.
	// The returned ConnectionQuitSubscription can be used to receive quit events in an isolated goroutine.
	// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
	// The channel is buffered with the specified size.
	// Call Unsubscribe() when done to release resources.
	SubscribeOnQuit(id string, bufferSize int) ConnectionQuitSubscription

	// GetQuitSubscription returns an existing quit subscription by ID, or nil if not found.
	GetQuitSubscription(id string) ConnectionQuitSubscription

	// Typed system event subscriptions (filename/object events)
	SubscribeOnFlightLoaded(id string, bufferSize int) FilenameSubscription
	SubscribeOnAircraftLoaded(id string, bufferSize int) FilenameSubscription
	SubscribeOnFlightPlanActivated(id string, bufferSize int) FilenameSubscription
	SubscribeOnObjectAdded(id string, bufferSize int) ObjectSubscription
	SubscribeOnObjectRemoved(id string, bufferSize int) ObjectSubscription

	// Typed system event subscriptions (crash/sound events)
	SubscribeOnCrashed(id string, bufferSize int) Subscription
	SubscribeOnCrashReset(id string, bufferSize int) Subscription
	SubscribeOnSoundEvent(id string, bufferSize int) Subscription

	// Callback-style handlers for system events (convenience helpers)
	OnFlightLoaded(handler FlightLoadedHandler) string
	RemoveFlightLoaded(id string) error

	// Crash and sound event handlers
	OnCrashed(handler CrashedHandler) string
	RemoveCrashed(id string) error

	OnCrashReset(handler CrashResetHandler) string
	RemoveCrashReset(id string) error

	OnSoundEvent(handler SoundEventHandler) string
	RemoveSoundEvent(id string) error

	OnAircraftLoaded(handler FlightLoadedHandler) string
	RemoveAircraftLoaded(id string) error

	OnFlightPlanActivated(handler FlightLoadedHandler) string
	RemoveFlightPlanActivated(id string) error

	OnObjectAdded(handler ObjectChangeHandler) string
	RemoveObjectAdded(id string) error

	OnObjectRemoved(handler ObjectChangeHandler) string
	RemoveObjectRemoved(id string) error

	// Client returns the underlying engine client for direct API access.
	// Returns nil if not connected.
	Client() engine.Client

	// Dataset Registration Methods
	// These methods provide direct access to dataset operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// RegisterDataset registers a complete dataset definition with SimConnect.
	// This is a convenience method that iterates over all definitions in the dataset
	// and calls AddToDataDefinition for each one.
	// Returns ErrNotConnected if not connected to the simulator.
	RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error

	// AddToDataDefinition adds a single data definition to a definition group.
	// Returns ErrNotConnected if not connected to the simulator.
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error

	// RequestDataOnSimObject requests data for a specific simulation object.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error

	// RequestDataOnSimObjectType requests data for all objects of a specific type within a radius.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestDataOnSimObjectType(requestID uint32, definitionID uint32, dwRadiusMeters uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error

	// ClearDataDefinition clears all data definitions for a definition group.
	// Returns ErrNotConnected if not connected to the simulator.
	ClearDataDefinition(definitionID uint32) error

	// SetDataOnSimObject sets data on a simulation object.
	// Returns ErrNotConnected if not connected to the simulator.
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error

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

	// RemoveStateChange removes a previously registered state change handler by id.
	// Returns an error if the id is unknown.
	RemoveConnectionStateChange(id string) error

	// RemoveMessage removes a previously registered message handler by id.
	// Returns an error if the id is unknown.
	RemoveMessage(id string) error
}
