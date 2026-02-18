//go:build windows
// +build windows

package manager

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
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
	stateHandlers                []instance.StateHandlerEntry
	messageHandlers              []instance.MessageHandlerEntry
	openHandlers                 []instance.OpenHandlerEntry
	quitHandlers                 []instance.QuitHandlerEntry
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
	simStateHandlers           []instance.SimStateHandlerEntry
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
	flightLoadedHandlers        []instance.FlightLoadedHandlerEntry
	aircraftLoadedHandlers      []instance.FlightLoadedHandlerEntry
	flightPlanActivatedHandlers []instance.FlightLoadedHandlerEntry
	objectAddedHandlers         []instance.ObjectChangeHandlerEntry
	objectRemovedHandlers       []instance.ObjectChangeHandlerEntry

	crashedHandlers    []instance.CrashedHandlerEntry
	crashResetHandlers []instance.CrashResetHandlerEntry
	soundEventHandlers []instance.SoundEventHandlerEntry
	crashedEventID     uint32
	crashResetEventID  uint32
	soundEventID       uint32

	viewHandlers                  []instance.ViewHandlerEntry
	flightPlanDeactivatedHandlers []instance.FlightPlanDeactivatedHandlerEntry
	viewEventID                   uint32
	flightPlanDeactivatedEventID  uint32

	pauseHandlers      []instance.PauseHandlerEntry
	simRunningHandlers []instance.SimRunningHandlerEntry

	// Custom system events
	customSystemEvents map[string]*instance.CustomSystemEvent
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

// Handler function types that are part of the public Manager API

// FlightLoaded handler type
type FlightLoadedHandler func(filename string)

// Object change handler type (add/remove)
type ObjectChangeHandler func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE)

// Crashed handler type
type CrashedHandler func()

// CrashReset handler type
type CrashResetHandler func()

// Sound event handler type
type SoundEventHandler func(soundID uint32)

// View event handler type — called when the camera view changes
type ViewHandler func(viewID uint32)

// FlightPlanDeactivated handler type — called when the active flight plan is deactivated
type FlightPlanDeactivatedHandler func()

// PauseHandler is invoked when the simulator pause state changes
type PauseHandler func(paused bool)

// SimRunningHandler is invoked when the simulator running state changes
type SimRunningHandler func(running bool)

// CustomSystemEventHandler is invoked when a custom system event fires
type CustomSystemEventHandler func(eventName string, data uint32)
