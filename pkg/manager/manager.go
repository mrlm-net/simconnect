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
	// Optional SubscriptionOption parameters can be provided to configure drop notifications.
	Subscribe(id string, bufferSize int, opts ...SubscriptionOption) Subscription

	// SubscribeWithFilter creates a new message subscription that only forwards
	// messages for which the provided filter function returns true.
	// Optional SubscriptionOption parameters can be provided to configure drop notifications.
	SubscribeWithFilter(id string, bufferSize int, filter func(engine.Message) bool, opts ...SubscriptionOption) Subscription

	// SubscribeWithType creates a new message subscription that only forwards
	// messages whose `DwID` matches one of the provided SIMCONNECT_RECV_ID values.
	// Optional SubscriptionOption parameters can be provided to configure drop notifications.
	SubscribeWithType(id string, bufferSize int, recvIDs []types.SIMCONNECT_RECV_ID, opts ...SubscriptionOption) Subscription

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
	SubscribeOnView(id string, bufferSize int) Subscription
	SubscribeOnFlightPlanDeactivated(id string, bufferSize int) Subscription

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

	OnView(handler ViewHandler) string
	RemoveView(id string) error

	OnFlightPlanDeactivated(handler FlightPlanDeactivatedHandler) string
	RemoveFlightPlanDeactivated(id string) error

	OnAircraftLoaded(handler FlightLoadedHandler) string
	RemoveAircraftLoaded(id string) error

	OnFlightPlanActivated(handler FlightLoadedHandler) string
	RemoveFlightPlanActivated(id string) error

	OnObjectAdded(handler ObjectChangeHandler) string
	RemoveObjectAdded(id string) error

	OnObjectRemoved(handler ObjectChangeHandler) string
	RemoveObjectRemoved(id string) error

	// Pause event dual API
	OnPause(handler PauseHandler) string
	RemovePause(id string) error
	SubscribeOnPause(id string, bufferSize int) Subscription

	// Sim running event dual API
	OnSimRunning(handler SimRunningHandler) string
	RemoveSimRunning(id string) error
	SubscribeOnSimRunning(id string, bufferSize int) Subscription

	// Custom system event API
	SubscribeToCustomSystemEvent(eventName string, bufferSize int) (Subscription, error)
	UnsubscribeFromCustomSystemEvent(eventName string) error
	OnCustomSystemEvent(eventName string, handler CustomSystemEventHandler) (string, error)
	RemoveCustomSystemEvent(eventName string, handlerID string) error

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

	// Event Emission Methods
	// These methods provide direct access to event operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// MapClientEventToSimEvent maps a client event ID to a SimConnect event name.
	// Returns ErrNotConnected if not connected to the simulator.
	MapClientEventToSimEvent(eventID uint32, eventName string) error

	// RemoveClientEvent removes a client event from a notification group.
	// Returns ErrNotConnected if not connected to the simulator.
	RemoveClientEvent(groupID uint32, eventID uint32) error

	// TransmitClientEvent transmits a client event to the simulator.
	// Returns ErrNotConnected if not connected to the simulator.
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error

	// TransmitClientEventEx1 transmits a client event with extended data to the simulator.
	// Returns ErrNotConnected if not connected to the simulator.
	TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error

	// MapClientDataNameToID maps a client data name to a client data ID.
	// Returns ErrNotConnected if not connected to the simulator.
	MapClientDataNameToID(clientDataName string, clientDataID uint32) error

	// Notification Group Methods
	// These methods provide direct access to notification group operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// AddClientEventToNotificationGroup adds a client event to a notification group.
	// Returns ErrNotConnected if not connected to the simulator.
	AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error

	// ClearNotificationGroup clears all events from a notification group.
	// Returns ErrNotConnected if not connected to the simulator.
	ClearNotificationGroup(groupID uint32) error

	// RequestNotificationGroup requests a notification group.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error

	// SetNotificationGroupPriority sets the priority of a notification group.
	// Returns ErrNotConnected if not connected to the simulator.
	SetNotificationGroupPriority(groupID uint32, priority uint32) error

	// System State Methods
	// These methods provide direct access to system state operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// RequestSystemState requests a system state value from the simulator.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error

	// SubscribeToSystemEvent subscribes to a SimConnect system event.
	// WARNING: Do not use event IDs in the manager's reserved range (999,999,850 - 999,999,999).
	// Use IDs from 1 to 999,999,849 for your own subscriptions.
	// Returns ErrNotConnected if not connected to the simulator.
	SubscribeToSystemEvent(eventID uint32, eventName string) error

	// UnsubscribeFromSystemEvent unsubscribes from a SimConnect system event.
	// Returns ErrNotConnected if not connected to the simulator.
	UnsubscribeFromSystemEvent(eventID uint32) error

	// Facility Methods
	// These methods provide direct access to facility operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// RegisterFacilityDataset registers a complete facility dataset definition with SimConnect.
	// Returns ErrNotConnected if not connected to the simulator.
	RegisterFacilityDataset(definitionID uint32, dataset *datasets.FacilityDataSet) error

	// AddToFacilityDefinition adds a field to a facility data definition.
	// Returns ErrNotConnected if not connected to the simulator.
	AddToFacilityDefinition(definitionID uint32, fieldName string) error

	// AddFacilityDataDefinitionFilter adds a filter to a facility data definition.
	// Returns ErrNotConnected if not connected to the simulator.
	AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error

	// ClearAllFacilityDataDefinitionFilters clears all filters from a facility data definition.
	// Returns ErrNotConnected if not connected to the simulator.
	ClearAllFacilityDataDefinitionFilters(definitionID uint32) error

	// RequestFacilitiesList requests a list of facilities of the specified type.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error

	// RequestFacilitiesListEX1 requests a list of facilities of the specified type (extended version).
	// Returns ErrNotConnected if not connected to the simulator.
	RequestFacilitiesListEX1(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error

	// RequestFacilityData requests facility data for a specific ICAO code and region.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error

	// RequestFacilityDataEX1 requests facility data with a facility type filter (extended version).
	// Returns ErrNotConnected if not connected to the simulator.
	RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error

	// RequestJetwayData requests jetway data for an airport.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error

	// SubscribeToFacilities subscribes to facility list updates of the specified type.
	// Returns ErrNotConnected if not connected to the simulator.
	SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error

	// SubscribeToFacilitiesEX1 subscribes to facility list updates with separate in-range and out-of-range request IDs.
	// Returns ErrNotConnected if not connected to the simulator.
	SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error

	// UnsubscribeToFacilitiesEX1 unsubscribes from facility list updates.
	// Returns ErrNotConnected if not connected to the simulator.
	UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error

	// RequestAllFacilities requests all facilities of the specified type.
	// Returns ErrNotConnected if not connected to the simulator.
	RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error

	// AI Traffic Methods
	// These methods provide direct access to AI traffic operations without needing
	// to call Client() first. They return ErrNotConnected if not connected.

	// AICreateParkedATCAircraft creates a parked ATC aircraft at an airport.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error

	// AISetAircraftFlightPlan assigns a flight plan to an AI aircraft.
	// Returns ErrNotConnected if not connected to the simulator.
	AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error

	// AICreateEnrouteATCAircraft creates an enroute ATC aircraft along a flight plan.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error

	// AICreateNonATCAircraft creates a non-ATC aircraft at a specific position.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error

	// AICreateSimulatedObject creates a simulated object at a specific position.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error

	// AIReleaseControl releases control of an AI object back to the simulator.
	// Returns ErrNotConnected if not connected to the simulator.
	AIReleaseControl(objectID uint32, requestID uint32) error

	// AIRemoveObject removes an AI object from the simulation.
	// Returns ErrNotConnected if not connected to the simulator.
	AIRemoveObject(objectID uint32, requestID uint32) error

	// EnumerateSimObjectsAndLiveries enumerates available sim objects and their liveries.
	// Returns ErrNotConnected if not connected to the simulator.
	EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error

	// AICreateEnrouteATCAircraftEX1 creates an enroute ATC aircraft with livery selection.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error

	// AICreateNonATCAircraftEX1 creates a non-ATC aircraft with livery selection.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error

	// AICreateParkedATCAircraftEX1 creates a parked ATC aircraft with livery selection.
	// Returns ErrNotConnected if not connected to the simulator.
	AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error

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

	// SimStatePeriod returns the configured SimState data request period
	SimStatePeriod() types.SIMCONNECT_PERIOD

	// RemoveStateChange removes a previously registered state change handler by id.
	// Returns an error if the id is unknown.
	RemoveConnectionStateChange(id string) error

	// RemoveMessage removes a previously registered message handler by id.
	// Returns an error if the id is unknown.
	RemoveMessage(id string) error
}
