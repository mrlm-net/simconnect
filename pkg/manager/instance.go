//go:build windows
// +build windows

package manager

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Instance implements the Manager interface
type Instance struct {
	name   string
	config *Config

	ctx    context.Context
	cancel context.CancelFunc

	logger *slog.Logger

	// Connection state
	mu    sync.RWMutex
	state ConnectionState
	// Handler entries store an id and the callback function so callers can
	// unregister using the id (similar to subscriptions).
	stateHandlers                []stateHandlerEntry
	messageHandlers              []messageHandlerEntry
	openHandlers                 []openHandlerEntry
	quitHandlers                 []quitHandlerEntry
	subscriptions                map[string]*subscription
	subsWg                       sync.WaitGroup // WaitGroup for graceful shutdown of subscriptions
	connectionStateSubscriptions map[string]*connectionStateSubscription
	connectionStateSubsWg        sync.WaitGroup // WaitGroup for graceful shutdown of connection state subscriptions
	openSubscriptions            map[string]*connectionOpenSubscription
	openSubsWg                   sync.WaitGroup // WaitGroup for graceful shutdown of open subscriptions
	quitSubscriptions            map[string]*connectionQuitSubscription
	quitSubsWg                   sync.WaitGroup // WaitGroup for graceful shutdown of quit subscriptions

	// Simulator state
	simState                   SimState
	simStateHandlers           []simStateHandlerEntry
	simStateSubscriptions      map[string]*simStateSubscription
	simStateSubsWg             sync.WaitGroup // WaitGroup for graceful shutdown of simulator state subscriptions
	cameraDefinitionID         uint32
	cameraRequestID            uint32
	cameraDataRequestPending   bool
	pauseEventID               uint32
	simEventID                 uint32
	flightLoadedEventID        uint32
	aircraftLoadedEventID      uint32
	objectAddedEventID         uint32
	objectRemovedEventID       uint32
	flightPlanActivatedEventID uint32

	// Event handlers
	flightLoadedHandlers        []flightLoadedHandlerEntry
	aircraftLoadedHandlers      []flightLoadedHandlerEntry
	flightPlanActivatedHandlers []flightLoadedHandlerEntry
	objectAddedHandlers         []objectChangeHandlerEntry
	objectRemovedHandlers       []objectChangeHandlerEntry

	crashedHandlers    []crashedHandlerEntry
	crashResetHandlers []crashResetHandlerEntry
	soundEventHandlers []soundEventHandlerEntry
	crashedEventID     uint32
	crashResetEventID  uint32
	soundEventID       uint32

	viewHandlers                  []viewHandlerEntry
	flightPlanDeactivatedHandlers []flightPlanDeactivatedHandlerEntry
	viewEventID                   uint32
	flightPlanDeactivatedEventID  uint32

	pauseHandlers      []pauseHandlerEntry
	simRunningHandlers []simRunningHandlerEntry

	// Custom system events
	customSystemEvents map[string]*customSystemEvent
	customEventIDAlloc uint32

	// Request tracking
	requestRegistry *RequestRegistry // Tracks active SimConnect requests for correlation with responses

	// Pre-allocated slices to reduce GC pressure in hot path (reused per message)
	handlersBuf []MessageHandler
	subsBuf     []*subscription

	// Pre-allocated buffers to reduce GC pressure (reused per notification)
	stateHandlersBuf                     []ConnectionStateChangeHandler
	simStateHandlersBuf                  []SimStateChangeHandler
	openHandlersBuf                      []ConnectionOpenHandler
	quitHandlersBuf                      []ConnectionQuitHandler
	crashedHandlersBuf                   []CrashedHandler
	crashResetHandlersBuf                []CrashResetHandler
	soundEventHandlersBuf                []SoundEventHandler
	viewHandlersBuf                      []ViewHandler
	flightPlanDeactivatedHandlersBuf     []FlightPlanDeactivatedHandler
	pauseHandlersBuf                     []PauseHandler
	simRunningHandlersBuf                []SimRunningHandler
	flightLoadedHandlersBuf              []FlightLoadedHandler
	objectChangeHandlersBuf              []ObjectChangeHandler
	stateSubsBuf                         []*connectionStateSubscription
	simStateSubsBuf                      []*simStateSubscription
	openSubsBuf                          []*connectionOpenSubscription
	quitSubsBuf                          []*connectionQuitSubscription

	// Current engine instance (recreated on each connection)
	engine *engine.Engine
}

// stateHandlerEntry stores a state change handler with an identifier
type stateHandlerEntry struct {
	id string
	fn ConnectionStateChangeHandler
}

// simStateHandlerEntry stores a simulator state change handler with an identifier
type simStateHandlerEntry struct {
	id string
	fn SimStateChangeHandler
}

// messageHandlerEntry stores a message handler with an identifier
type messageHandlerEntry struct {
	id string
	fn MessageHandler
}

// FlightLoaded handler type
type FlightLoadedHandler func(filename string)

type flightLoadedHandlerEntry struct {
	id string
	fn FlightLoadedHandler
}

// Object change handler type (add/remove)
type ObjectChangeHandler func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE)

type objectChangeHandlerEntry struct {
	id string
	fn ObjectChangeHandler
}

// Crashed handler type
type CrashedHandler func()

type crashedHandlerEntry struct {
	id string
	fn CrashedHandler
}

// CrashReset handler type
type CrashResetHandler func()

type crashResetHandlerEntry struct {
	id string
	fn CrashResetHandler
}

// Sound event handler type
type SoundEventHandler func(soundID uint32)

type soundEventHandlerEntry struct {
	id string
	fn SoundEventHandler
}

// View event handler type — called when the camera view changes
type ViewHandler func(viewID uint32)

type viewHandlerEntry struct {
	id string
	fn ViewHandler
}

// FlightPlanDeactivated handler type — called when the active flight plan is deactivated
type FlightPlanDeactivatedHandler func()

type flightPlanDeactivatedHandlerEntry struct {
	id string
	fn FlightPlanDeactivatedHandler
}

// PauseHandler is invoked when the simulator pause state changes
type PauseHandler func(paused bool)

type pauseHandlerEntry struct {
	id string
	fn PauseHandler
}

// SimRunningHandler is invoked when the simulator running state changes
type SimRunningHandler func(running bool)

type simRunningHandlerEntry struct {
	id string
	fn SimRunningHandler
}

// customSystemEvent tracks a user-registered custom system event
type customSystemEvent struct {
	name     string
	id       uint32
	handlers []customSystemEventHandlerEntry
}

type customSystemEventHandlerEntry struct {
	id string
	fn CustomSystemEventHandler
}

// CustomSystemEventHandler is invoked when a custom system event fires
type CustomSystemEventHandler func(eventName string, data uint32)

// openHandlerEntry stores a connection open handler with an identifier
type openHandlerEntry struct {
	id string
	fn ConnectionOpenHandler
}

// quitHandlerEntry stores a connection quit handler with an identifier
type quitHandlerEntry struct {
	id string
	fn ConnectionQuitHandler
}
