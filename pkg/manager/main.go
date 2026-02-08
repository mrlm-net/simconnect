//go:build windows

package manager

import (
	"context"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// defaultSimState returns a new SimState with all fields initialized to their default/uninitialized values.
// This centralizes the initialization to avoid duplication and ensure consistency.
func defaultSimState() SimState {
	return SimState{
		Camera:                   CameraStateUninitialized,
		Substate:                 CameraSubstateUninitialized,
		Paused:                   false,
		SimRunning:               false,
		SimulationRate:           0,
		SimulationTime:           0,
		LocalTime:                0,
		ZuluTime:                 0,
		IsInVR:                   false,
		IsUsingMotionControllers: false,
		IsUsingJoystickThrottle:  false,
		IsInRTC:                  false,
		IsAvatar:                 false,
		IsAircraft:               false,
		Crashed:                  false,
		CrashReset:               false,
		Sound:                    0,
		LocalDay:                 0,
		LocalMonth:               0,
		LocalYear:                0,
		ZuluDay:                  0,
		ZuluMonth:                0,
		ZuluYear:                 0,
		Realism:                  0,
		VisualModelRadius:        0,
		SimDisabled:              false,
		RealismCrashDetection:    false,
		RealismCrashWithOthers:   false,
		TrackIREnabled:           false,
		UserInputEnabled:         false,
		SimOnGround:              false,
		AmbientTemperature:       0,
		AmbientPressure:          0,
		AmbientWindVelocity:      0,
		AmbientWindDirection:     0,
		AmbientVisibility:        0,
		AmbientInCloud:           false,
		AmbientPrecipState:       0,
		BarometerPressure:        0,
		SeaLevelPressure:         0,
		GroundAltitude:           0,
		MagVar:                   0,
		SurfaceType:              0,
	}
}

// safeCallHandler executes a handler function with panic recovery.
// If the handler panics, the panic is logged and execution continues.
func safeCallHandler(logger *slog.Logger, name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[manager] Handler panic recovered", "handler", name, "panic", r)
		}
	}()
	fn()
}

// New creates a new Manager instance with the given application name and options
func New(name string, opts ...Option) Manager {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	// If caller did not provide a logger, construct one using the configured
	// LogLevel so manager logs reflect the requested verbosity.
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	}

	// Validate EngineOptions once at construction time
	for _, eo := range config.EngineOptions {
		if eo != nil {
			pc := reflect.ValueOf(eo).Pointer()
			if fn := runtime.FuncForPC(pc); fn != nil {
				name := fn.Name()
				if strings.Contains(name, "WithContext") {
					config.Logger.Warn("[manager] EngineOptions contains WithContext, will be overridden by manager context")
				}
				if strings.Contains(name, "WithLogger") {
					config.Logger.Warn("[manager] EngineOptions contains WithLogger, will be overridden by manager logger")
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(config.Context)

	return &Instance{
		name:                         name,
		config:                       config,
		ctx:                          ctx,
		cancel:                       cancel,
		logger:                       config.Logger,
		state:                        StateDisconnected,
		stateHandlers:                []stateHandlerEntry{},
		messageHandlers:              []messageHandlerEntry{},
		openHandlers:                 []openHandlerEntry{},
		quitHandlers:                 []quitHandlerEntry{},
		subscriptions:                make(map[string]*subscription),
		connectionStateSubscriptions: make(map[string]*connectionStateSubscription),
		openSubscriptions:            make(map[string]*connectionOpenSubscription),
		quitSubscriptions:            make(map[string]*connectionQuitSubscription),
		simState:                     defaultSimState(),
		simStateHandlers:             []simStateHandlerEntry{},
		simStateSubscriptions:        make(map[string]*simStateSubscription),
		cameraDefinitionID:           CameraDefinitionID,
		cameraRequestID:              CameraRequestID,
		pauseEventID:                 PauseEventID,
		simEventID:                   SimEventID,
		flightLoadedEventID:          FlightLoadedEventID,
		aircraftLoadedEventID:        AircraftLoadedEventID,
		objectAddedEventID:           ObjectAddedEventID,
		objectRemovedEventID:         ObjectRemovedEventID,
		flightPlanActivatedEventID:   FlightPlanActivatedEventID,
		crashedEventID:               CrashedEventID,
		crashResetEventID:            CrashResetEventID,
		soundEventID:                 SoundEventID,
		crashedHandlers:              []crashedHandlerEntry{},
		crashResetHandlers:           []crashResetHandlerEntry{},
		soundEventHandlers:           []soundEventHandlerEntry{},
		requestRegistry:              NewRequestRegistry(),
	}
}

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

	// Request tracking
	requestRegistry *RequestRegistry // Tracks active SimConnect requests for correlation with responses

	// Pre-allocated slices to reduce GC pressure in hot path (reused per message)
	handlersBuf []MessageHandler
	subsBuf     []*subscription

	// Pre-allocated buffers to reduce GC pressure (reused per notification)
	stateHandlersBuf        []ConnectionStateChangeHandler
	simStateHandlersBuf     []SimStateChangeHandler
	openHandlersBuf         []ConnectionOpenHandler
	quitHandlersBuf         []ConnectionQuitHandler
	crashedHandlersBuf      []CrashedHandler
	crashResetHandlersBuf   []CrashResetHandler
	soundEventHandlersBuf   []SoundEventHandler
	flightLoadedHandlersBuf []FlightLoadedHandler
	objectChangeHandlersBuf []ObjectChangeHandler
	stateSubsBuf            []*connectionStateSubscription
	simStateSubsBuf         []*simStateSubscription
	openSubsBuf             []*connectionOpenSubscription
	quitSubsBuf             []*connectionQuitSubscription

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
