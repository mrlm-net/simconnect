//go:build windows

package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

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

// State returns the current connection state
func (m *Instance) ConnectionState() ConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// setState updates the connection state and notifies handlers
func (m *Instance) setState(newState ConnectionState) {
	m.mu.Lock()
	oldState := m.state
	if oldState == newState {
		m.mu.Unlock()
		return
	}
	m.state = newState
	// Reuse pre-allocated buffers
	if cap(m.stateHandlersBuf) < len(m.stateHandlers) {
		m.stateHandlersBuf = make([]ConnectionStateChangeHandler, len(m.stateHandlers))
	} else {
		m.stateHandlersBuf = m.stateHandlersBuf[:len(m.stateHandlers)]
	}
	for i, e := range m.stateHandlers {
		m.stateHandlersBuf[i] = e.fn
	}
	handlers := m.stateHandlersBuf

	if cap(m.stateSubsBuf) < len(m.connectionStateSubscriptions) {
		m.stateSubsBuf = make([]*connectionStateSubscription, 0, len(m.connectionStateSubscriptions))
	} else {
		m.stateSubsBuf = m.stateSubsBuf[:0]
	}
	for _, sub := range m.connectionStateSubscriptions {
		m.stateSubsBuf = append(m.stateSubsBuf, sub)
	}
	stateSubs := m.stateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] State changed", "old", oldState, "new", newState)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "ConnectionStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := ConnectionStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] State subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// setSimState updates the simulator state and notifies handlers
func (m *Instance) setSimState(newState SimState) {
	m.mu.Lock()
	oldState := m.simState
	if oldState.Equal(newState) {
		m.mu.Unlock()
		return
	}
	m.simState = newState
	// Reuse pre-allocated buffers
	if cap(m.simStateHandlersBuf) < len(m.simStateHandlers) {
		m.simStateHandlersBuf = make([]SimStateChangeHandler, len(m.simStateHandlers))
	} else {
		m.simStateHandlersBuf = m.simStateHandlersBuf[:len(m.simStateHandlers)]
	}
	for i, e := range m.simStateHandlers {
		m.simStateHandlersBuf[i] = e.fn
	}
	handlers := m.simStateHandlersBuf

	if cap(m.simStateSubsBuf) < len(m.simStateSubscriptions) {
		m.simStateSubsBuf = make([]*simStateSubscription, 0, len(m.simStateSubscriptions))
	} else {
		m.simStateSubsBuf = m.simStateSubsBuf[:0]
	}
	for _, sub := range m.simStateSubscriptions {
		m.simStateSubsBuf = append(m.simStateSubsBuf, sub)
	}
	stateSubs := m.simStateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] SimState changed", "oldCamera", oldState.Camera, "newCamera", newState.Camera)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "SimStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := SimStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] SimState subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// notifySimStateChange notifies handlers and subscriptions of a SimState change.
// This is a helper used by delta update paths where state is already modified in-place.
// The caller must have already updated m.simState and must NOT hold m.mu when calling this.
func (m *Instance) notifySimStateChange(oldState, newState SimState) {
	// Gather handlers and subscriptions under lock
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.simStateHandlersBuf) < len(m.simStateHandlers) {
		m.simStateHandlersBuf = make([]SimStateChangeHandler, len(m.simStateHandlers))
	} else {
		m.simStateHandlersBuf = m.simStateHandlersBuf[:len(m.simStateHandlers)]
	}
	for i, e := range m.simStateHandlers {
		m.simStateHandlersBuf[i] = e.fn
	}
	handlers := m.simStateHandlersBuf

	if cap(m.simStateSubsBuf) < len(m.simStateSubscriptions) {
		m.simStateSubsBuf = make([]*simStateSubscription, 0, len(m.simStateSubscriptions))
	} else {
		m.simStateSubsBuf = m.simStateSubsBuf[:0]
	}
	for _, sub := range m.simStateSubscriptions {
		m.simStateSubsBuf = append(m.simStateSubsBuf, sub)
	}
	stateSubs := m.simStateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] SimState changed", "oldCamera", oldState.Camera, "newCamera", newState.Camera)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "SimStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := SimStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] SimState subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// OnConnectionStateChange registers a callback to be invoked when connection state changes.
// Returns a unique id that can be used to remove the handler via RemoveConnectionStateChange.
func (m *Instance) OnConnectionStateChange(handler ConnectionStateChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.stateHandlers = append(m.stateHandlers, stateHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered state handler", "id", id)
	}
	return id
}

// RemoveConnectionStateChange removes a previously registered connection state change handler by id.
func (m *Instance) RemoveConnectionStateChange(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.stateHandlers {
		if e.id == id {
			m.stateHandlers = append(m.stateHandlers[:i], m.stateHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed state handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("state handler not found: %s", id)
}

// OnMessage registers a callback to be invoked when a message is received.
// Returns a unique id that can be used to remove the handler via RemoveMessage.
func (m *Instance) OnMessage(handler MessageHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.messageHandlers = append(m.messageHandlers, messageHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered message handler", "id", id)
	}
	return id
}

// RemoveMessage removes a previously registered message handler by id.
func (m *Instance) RemoveMessage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.messageHandlers {
		if e.id == id {
			m.messageHandlers = append(m.messageHandlers[:i], m.messageHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed message handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("message handler not found: %s", id)
}

// OnOpen registers a callback to be invoked when the simulator connection opens.
// Returns a unique id that can be used to remove the handler via RemoveOpen.
func (m *Instance) OnOpen(handler ConnectionOpenHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.openHandlers = append(m.openHandlers, openHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered open handler", "id", id)
	}
	return id
}

// RemoveOpen removes a previously registered open handler by id.
func (m *Instance) RemoveOpen(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.openHandlers {
		if e.id == id {
			m.openHandlers = append(m.openHandlers[:i], m.openHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed open handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("open handler not found: %s", id)
}

// OnQuit registers a callback to be invoked when the simulator quits.
// Returns a unique id that can be used to remove the handler via RemoveQuit.
func (m *Instance) OnQuit(handler ConnectionQuitHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.quitHandlers = append(m.quitHandlers, quitHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered quit handler", "id", id)
	}
	return id
}

// RemoveQuit removes a previously registered quit handler by id.
func (m *Instance) RemoveQuit(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.quitHandlers {
		if e.id == id {
			m.quitHandlers = append(m.quitHandlers[:i], m.quitHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed quit handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("quit handler not found: %s", id)
}

// OnFlightLoaded registers a callback invoked when a FlightLoaded system event arrives.
func (m *Instance) OnFlightLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightLoadedHandlers = append(m.flightLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightLoaded handler", "id", id)
	}
	return id
}

// RemoveFlightLoaded removes a previously registered FlightLoaded handler.
func (m *Instance) RemoveFlightLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightLoadedHandlers {
		if e.id == id {
			m.flightLoadedHandlers = append(m.flightLoadedHandlers[:i], m.flightLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightLoaded handler not found: %s", id)
}

// OnAircraftLoaded registers a callback invoked when an AircraftLoaded system event arrives.
func (m *Instance) OnAircraftLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered AircraftLoaded handler", "id", id)
	}
	return id
}

// RemoveAircraftLoaded removes a previously registered AircraftLoaded handler.
func (m *Instance) RemoveAircraftLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.aircraftLoadedHandlers {
		if e.id == id {
			m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers[:i], m.aircraftLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed AircraftLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("AircraftLoaded handler not found: %s", id)
}

// OnFlightPlanActivated registers a callback invoked when a FlightPlanActivated system event arrives.
func (m *Instance) OnFlightPlanActivated(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightPlanActivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanActivated removes a previously registered FlightPlanActivated handler.
func (m *Instance) RemoveFlightPlanActivated(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightPlanActivatedHandlers {
		if e.id == id {
			m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers[:i], m.flightPlanActivatedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightPlanActivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanActivated handler not found: %s", id)
}

// OnObjectAdded registers a callback invoked when an ObjectAdded system event arrives.
func (m *Instance) OnObjectAdded(handler ObjectChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.objectAddedHandlers = append(m.objectAddedHandlers, objectChangeHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered ObjectAdded handler", "id", id)
	}
	return id
}

// RemoveObjectAdded removes a previously registered ObjectAdded handler.
func (m *Instance) RemoveObjectAdded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.objectAddedHandlers {
		if e.id == id {
			m.objectAddedHandlers = append(m.objectAddedHandlers[:i], m.objectAddedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed ObjectAdded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectAdded handler not found: %s", id)
}

// OnObjectRemoved registers a callback invoked when an ObjectRemoved system event arrives.
func (m *Instance) OnObjectRemoved(handler ObjectChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.objectRemovedHandlers = append(m.objectRemovedHandlers, objectChangeHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered ObjectRemoved handler", "id", id)
	}
	return id
}

// RemoveObjectRemoved removes a previously registered ObjectRemoved handler.
func (m *Instance) RemoveObjectRemoved(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.objectRemovedHandlers {
		if e.id == id {
			m.objectRemovedHandlers = append(m.objectRemovedHandlers[:i], m.objectRemovedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed ObjectRemoved handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectRemoved handler not found: %s", id)
}

// SubscribeOnCrashed returns a subscription that receives raw engine.Message for Crashed events
func (m *Instance) SubscribeOnCrashed(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.crashedEventID)
	}
	return m.SubscribeWithFilter(id+"-crashed", bufferSize, filter)
}

// SubscribeOnCrashReset returns a subscription for CrashReset events
func (m *Instance) SubscribeOnCrashReset(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.crashResetEventID)
	}
	return m.SubscribeWithFilter(id+"-crashreset", bufferSize, filter)
}

// SubscribeOnSoundEvent returns a subscription for Sound events
func (m *Instance) SubscribeOnSoundEvent(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.soundEventID)
	}
	return m.SubscribeWithFilter(id+"-sound", bufferSize, filter)
}

// OnCrashed registers a callback invoked when a Crashed system event arrives.
func (m *Instance) OnCrashed(handler CrashedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.crashedHandlers = append(m.crashedHandlers, crashedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered Crashed handler", "id", id)
	}
	return id
}

// RemoveCrashed removes a previously registered Crashed handler.
func (m *Instance) RemoveCrashed(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.crashedHandlers {
		if e.id == id {
			m.crashedHandlers = append(m.crashedHandlers[:i], m.crashedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed Crashed handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("Crashed handler not found: %s", id)
}

// OnCrashReset registers a callback invoked when a CrashReset system event arrives.
func (m *Instance) OnCrashReset(handler CrashResetHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.crashResetHandlers = append(m.crashResetHandlers, crashResetHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered CrashReset handler", "id", id)
	}
	return id
}

// RemoveCrashReset removes a previously registered CrashReset handler.
func (m *Instance) RemoveCrashReset(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.crashResetHandlers {
		if e.id == id {
			m.crashResetHandlers = append(m.crashResetHandlers[:i], m.crashResetHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed CrashReset handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("CrashReset handler not found: %s", id)
}

// OnSoundEvent registers a callback invoked when a Sound event arrives.
func (m *Instance) OnSoundEvent(handler SoundEventHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.soundEventHandlers = append(m.soundEventHandlers, soundEventHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered SoundEvent handler", "id", id)
	}
	return id
}

// RemoveSoundEvent removes a previously registered Sound event handler.
func (m *Instance) RemoveSoundEvent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.soundEventHandlers {
		if e.id == id {
			m.soundEventHandlers = append(m.soundEventHandlers[:i], m.soundEventHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed SoundEvent handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SoundEvent handler not found: %s", id)
}

// setOpen invokes all registered open handlers and sends to subscriptions
func (m *Instance) setOpen(data types.ConnectionOpenData) {
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.openHandlersBuf) < len(m.openHandlers) {
		m.openHandlersBuf = make([]ConnectionOpenHandler, len(m.openHandlers))
	} else {
		m.openHandlersBuf = m.openHandlersBuf[:len(m.openHandlers)]
	}
	for i, e := range m.openHandlers {
		m.openHandlersBuf[i] = e.fn
	}
	handlers := m.openHandlersBuf

	if cap(m.openSubsBuf) < len(m.openSubscriptions) {
		m.openSubsBuf = make([]*connectionOpenSubscription, 0, len(m.openSubscriptions))
	} else {
		m.openSubsBuf = m.openSubsBuf[:0]
	}
	for _, sub := range m.openSubscriptions {
		m.openSubsBuf = append(m.openSubsBuf, sub)
	}
	openSubs := m.openSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection opened")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCallHandler(m.logger, "ConnectionOpenHandler", func() {
			h(d)
		})
	}

	// Forward open event to subscriptions (non-blocking)
	for _, sub := range openSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- data:
			default:
				// Channel full, skip event to avoid blocking
				m.logger.Debug("[manager] Open subscription channel full, dropping open event")
			}
		}
		sub.closeMu.Unlock()
	}
}

// setQuit invokes all registered quit handlers and sends to subscriptions
func (m *Instance) setQuit(data types.ConnectionQuitData) {
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.quitHandlersBuf) < len(m.quitHandlers) {
		m.quitHandlersBuf = make([]ConnectionQuitHandler, len(m.quitHandlers))
	} else {
		m.quitHandlersBuf = m.quitHandlersBuf[:len(m.quitHandlers)]
	}
	for i, e := range m.quitHandlers {
		m.quitHandlersBuf[i] = e.fn
	}
	handlers := m.quitHandlersBuf

	if cap(m.quitSubsBuf) < len(m.quitSubscriptions) {
		m.quitSubsBuf = make([]*connectionQuitSubscription, 0, len(m.quitSubscriptions))
	} else {
		m.quitSubsBuf = m.quitSubsBuf[:0]
	}
	for _, sub := range m.quitSubscriptions {
		m.quitSubsBuf = append(m.quitSubsBuf, sub)
	}
	quitSubs := m.quitSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection quit")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCallHandler(m.logger, "ConnectionQuitHandler", func() {
			h(d)
		})
	}

	// Forward quit event to subscriptions (non-blocking)
	for _, sub := range quitSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- data:
			default:
				// Channel full, skip event to avoid blocking
				m.logger.Debug("[manager] Quit subscription channel full, dropping quit event")
			}
		}
		sub.closeMu.Unlock()
	}
}

// SimState returns the current simulator state
func (m *Instance) SimState() SimState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.simState
}

// OnSimStateChange registers a callback to be invoked when simulator state changes.
// Returns a unique id that can be used to remove the handler via RemoveSimStateChange.
func (m *Instance) OnSimStateChange(handler SimStateChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.simStateHandlers = append(m.simStateHandlers, simStateHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered SimState handler", "id", id)
	}
	return id
}

// RemoveSimStateChange removes a previously registered simulator state change handler by id.
func (m *Instance) RemoveSimStateChange(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.simStateHandlers {
		if e.id == id {
			m.simStateHandlers = append(m.simStateHandlers[:i], m.simStateHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed SimState handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SimState handler not found: %s", id)
}

// Client returns the underlying engine client for direct API access
func (m *Instance) Client() engine.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.engine
}

// IsAutoReconnect returns whether automatic reconnection is enabled
func (m *Instance) IsAutoReconnect() bool {
	return m.config.AutoReconnect
}

// RetryInterval returns the delay between connection attempts
func (m *Instance) RetryInterval() time.Duration {
	return m.config.RetryInterval
}

// ConnectionTimeout returns the timeout for each connection attempt
func (m *Instance) ConnectionTimeout() time.Duration {
	return m.config.ConnectionTimeout
}

// ReconnectDelay returns the delay before reconnecting after disconnect
func (m *Instance) ReconnectDelay() time.Duration {
	return m.config.ReconnectDelay
}

// ShutdownTimeout returns the timeout for graceful shutdown of subscriptions
func (m *Instance) ShutdownTimeout() time.Duration {
	return m.config.ShutdownTimeout
}

// MaxRetries returns the maximum number of connection retries (0 = unlimited)
func (m *Instance) MaxRetries() int {
	return m.config.MaxRetries
}

// Start begins the connection lifecycle management
func (m *Instance) Start() error {
	m.logger.Debug("[manager] Starting connection lifecycle management")

	// Reconnection loop
	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("[manager] Context cancelled, stopping manager")
			m.setState(StateDisconnected)
			return m.ctx.Err()
		default:
		}

		err := m.runConnection()
		if err != nil {
			// Context cancelled - exit completely
			m.logger.Debug("[manager] Connection ended with error", "error", err)
			return err
		}

		// Simulator disconnected (err == nil) - check if we should reconnect
		if !m.config.AutoReconnect {
			m.logger.Debug("[manager] Auto-reconnect disabled, stopping manager")
			return nil
		}

		m.setState(StateReconnecting)
		m.logger.Debug("[manager] Waiting before reconnecting", "delay", m.config.ReconnectDelay)

		select {
		case <-m.ctx.Done():
			m.logger.Debug("[manager] Shutdown requested, not reconnecting")
			m.setState(StateDisconnected)
			return m.ctx.Err()
		case <-time.After(m.config.ReconnectDelay):
			m.logger.Debug("[manager] Attempting to reconnect...")
		}
	}
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func (m *Instance) runConnection() error {
	// Create engine options: start with manager's context, then add user options
	opts := []engine.Option{engine.WithContext(m.ctx)}
	opts = append(opts, m.config.EngineOptions...)
	// Manager's logger always takes precedence over any logger in EngineOptions
	if m.config.Logger != nil {
		opts = append(opts, engine.WithLogger(m.config.Logger))
	}

	// Create a new engine instance for this connection
	m.mu.Lock()
	m.engine = engine.New(m.name, opts...)
	m.mu.Unlock()

	// Attempt to connect with retry
	if err := m.connectWithRetry(); err != nil {
		m.mu.Lock()
		m.engine = nil
		m.mu.Unlock()
		return err
	}

	m.setState(StateConnected)
	m.logger.Debug("[manager] Connected to simulator")

	// Process messages until disconnection or cancellation
	stream := m.engine.Stream()
	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("[manager] Context cancelled, disconnecting...")
			m.disconnect()
			return m.ctx.Err()

		case msg, ok := <-stream:
			if !ok {
				// Stream closed (simulator disconnected)
				m.logger.Debug("[manager] Stream closed (simulator disconnected)")
				m.setSimState(defaultSimState())
				m.setState(StateDisconnected)
				m.mu.Lock()
				m.engine = nil
				m.mu.Unlock()
				return nil // Return nil to allow reconnection
			}

			// Process message in separate method to ensure proper defer handling
			m.processMessage(msg)
		}
	}
}

// connectWithRetry attempts to connect to the simulator with fixed retry interval
func (m *Instance) connectWithRetry() error {
	m.setState(StateConnecting)

	attempts := 0

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("[manager] Cancelled while waiting for simulator")
			m.setState(StateDisconnected)
			return m.ctx.Err()
		default:
		}

		// Create a timeout context for this connection attempt
		connectCtx, cancel := context.WithTimeout(m.ctx, m.config.ConnectionTimeout)
		err := m.connectWithTimeout(connectCtx)
		cancel()

		if err == nil {
			return nil // Connected successfully
		}

		attempts++
		if m.config.MaxRetries > 0 && attempts >= m.config.MaxRetries {
			m.setState(StateDisconnected)
			return fmt.Errorf("max connection retries (%d) exceeded: %w", m.config.MaxRetries, err)
		}

		m.logger.Debug("[manager] Connection attempt failed, retrying", "attempt", attempts, "error", err, "retryInterval", m.config.RetryInterval)

		select {
		case <-m.ctx.Done():
			m.setState(StateDisconnected)
			return m.ctx.Err()
		case <-time.After(m.config.RetryInterval):
		}
	}
}

// connectWithTimeout attempts a single connection with timeout
func (m *Instance) connectWithTimeout(ctx context.Context) error {
	done := make(chan error, 1)

	go func() {
		done <- m.engine.Connect()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// disconnect gracefully disconnects from the simulator
func (m *Instance) disconnect() {
	m.mu.Lock()
	eng := m.engine
	m.engine = nil
	cameraRequestPending := m.cameraDataRequestPending
	m.cameraDataRequestPending = false
	m.mu.Unlock()

	if eng != nil {
		// Clear camera data definition if it was requested
		if cameraRequestPending {
			if err := eng.ClearDataDefinition(m.cameraDefinitionID); err != nil {
				m.logger.Error("[manager] Failed to clear camera data definition", "error", err)
			}
		}

		if err := eng.Disconnect(); err != nil {
			m.logger.Error("[manager] Disconnect error", "error", err)
		}
	}

	// Clean up request registry on disconnect
	m.requestRegistry.Clear()

	m.setState(StateDisconnected)
}

// Stop gracefully shuts down the manager
func (m *Instance) Stop() error {
	m.logger.Debug("[manager] Stopping manager")
	m.cancel() // This will trigger all subscription context watchers

	// Wait for all subscriptions to close with timeout
	m.logger.Debug("[manager] Waiting for subscriptions to close...")
	done := make(chan struct{})
	go func() {
		m.subsWg.Wait()
		m.connectionStateSubsWg.Wait()
		m.simStateSubsWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Debug("[manager] All subscriptions closed")
	case <-time.After(m.config.ShutdownTimeout):
		m.logger.Warn("[manager] Shutdown timeout exceeded, some subscriptions may not have closed gracefully", "timeout", m.config.ShutdownTimeout)
	}

	m.disconnect()
	return nil
}

// processMessage handles a single message from the simulator.
// This method ensures defer msg.Release() fires at the end of each message processing,
// not at the end of the entire connection loop.
func (m *Instance) processMessage(msg engine.Message) {
	// Release the message buffer back to pool when done processing
	// This is critical for memory efficiency under high message load
	defer msg.Release()

	if msg.Err != nil {
		m.logger.Error("[manager] Stream error", "error", msg.Err)
		return
	}

	// Check for connection ready (OPEN) message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
		m.logger.Debug("[manager] Received OPEN message, connection is now available")
		m.setState(StateAvailable)

		// Extract version information from OPEN message
		openMsg := msg.AsOpen()
		if openMsg != nil {
			appName := engine.BytesToString(openMsg.SzApplicationName[:])
			openData := types.ConnectionOpenData{
				ApplicationName:         appName,
				ApplicationVersionMajor: uint32(openMsg.DwApplicationVersionMajor),
				ApplicationVersionMinor: uint32(openMsg.DwApplicationVersionMinor),
				ApplicationBuildMajor:   uint32(openMsg.DwApplicationBuildMajor),
				ApplicationBuildMinor:   uint32(openMsg.DwApplicationBuildMinor),
				SimConnectVersionMajor:  uint32(openMsg.DwSimConnectVersionMajor),
				SimConnectVersionMinor:  uint32(openMsg.DwSimConnectVersionMinor),
				SimConnectBuildMajor:    uint32(openMsg.DwSimConnectBuildMajor),
				SimConnectBuildMinor:    uint32(openMsg.DwSimConnectBuildMinor),
			}
			m.setOpen(openData)
		}

		// Initialize simulator state and request camera data
		m.mu.Lock()
		client := m.engine
		m.mu.Unlock()

		if client != nil {
			// Set initial SimState
			m.setSimState(defaultSimState())

			// Subscribe to pause events
			// Register manager ID for tracking, but subscribe with actual SimConnect event ID 1000
			m.requestRegistry.Register(m.pauseEventID, RequestTypeEvent, "Pause Event Subscription")
			if err := client.SubscribeToSystemEvent(m.pauseEventID, "Pause"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Pause event", "error", err)
			}

			// Subscribe to sim events
			// Register manager ID for tracking, but subscribe with actual SimConnect event ID 1001
			m.requestRegistry.Register(m.simEventID, RequestTypeEvent, "Sim Event Subscription")
			if err := client.SubscribeToSystemEvent(m.simEventID, "Sim"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Sim event", "error", err)
			}

			// Subscribe to additional system events
			m.requestRegistry.Register(m.flightLoadedEventID, RequestTypeEvent, "FlightLoaded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.flightLoadedEventID, "FlightLoaded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to FlightLoaded event", "error", err)
			}

			m.requestRegistry.Register(m.aircraftLoadedEventID, RequestTypeEvent, "AircraftLoaded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.aircraftLoadedEventID, "AircraftLoaded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to AircraftLoaded event", "error", err)
			}

			m.requestRegistry.Register(m.flightPlanActivatedEventID, RequestTypeEvent, "FlightPlanActivated Event Subscription")
			if err := client.SubscribeToSystemEvent(m.flightPlanActivatedEventID, "FlightPlanActivated"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to FlightPlanActivated event", "error", err)
			}

			m.requestRegistry.Register(m.objectAddedEventID, RequestTypeEvent, "ObjectAdded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.objectAddedEventID, "ObjectAdded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to ObjectAdded event", "error", err)
			}

			m.requestRegistry.Register(m.objectRemovedEventID, RequestTypeEvent, "ObjectRemoved Event Subscription")
			if err := client.SubscribeToSystemEvent(m.objectRemovedEventID, "ObjectRemoved"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to ObjectRemoved event", "error", err)
			}

			// (Position change events removed)

			// Define camera data structure
			m.requestRegistry.Register(m.cameraDefinitionID, RequestTypeDataDefinition, "Camera State and Substate Definition")
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0); err != nil {
				m.logger.Error("[manager] Failed to add CAMERA STATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1); err != nil {
				m.logger.Error("[manager] Failed to add CAMERA SUBSTATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIMULATION RATE", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2); err != nil {
				m.logger.Error("[manager] Failed to add SIMULATION RATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIMULATION TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3); err != nil {
				m.logger.Error("[manager] Failed to add SIMULATION TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5); err != nil {
				m.logger.Error("[manager] Failed to add ZULU TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS IN VR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 6); err != nil {
				m.logger.Error("[manager] Failed to add IS IN VR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS USING MOTION CONTROLLERS", "", types.SIMCONNECT_DATATYPE_INT32, 0, 7); err != nil {
				m.logger.Error("[manager] Failed to add IS USING MOTION CONTROLLERS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS USING JOYSTICK THROTTLE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 8); err != nil {
				m.logger.Error("[manager] Failed to add IS USING JOYSTICK THROTTLE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS IN RTC", "", types.SIMCONNECT_DATATYPE_INT32, 0, 9); err != nil {
				m.logger.Error("[manager] Failed to add IS IN RTC definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS AVATAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 10); err != nil {
				m.logger.Error("[manager] Failed to add IS AVATAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS AIRCRAFT", "", types.SIMCONNECT_DATATYPE_INT32, 0, 11); err != nil {
				m.logger.Error("[manager] Failed to add IS AIRCRAFT definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL DAY OF MONTH", "", types.SIMCONNECT_DATATYPE_INT32, 0, 12); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL DAY OF MONTH definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL MONTH OF YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 13); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL MONTH OF YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 14); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU DAY OF MONTH", "", types.SIMCONNECT_DATATYPE_INT32, 0, 15); err != nil {
				m.logger.Error("[manager] Failed to add ZULU DAY OF MONTH definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU MONTH OF YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 16); err != nil {
				m.logger.Error("[manager] Failed to add ZULU MONTH OF YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 17); err != nil {
				m.logger.Error("[manager] Failed to add ZULU YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 18); err != nil {
				m.logger.Error("[manager] Failed to add REALISM definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "VISUAL MODEL RADIUS", "meters", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 19); err != nil {
				m.logger.Error("[manager] Failed to add VISUAL MODEL RADIUS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIM DISABLED", "", types.SIMCONNECT_DATATYPE_INT32, 0, 20); err != nil {
				m.logger.Error("[manager] Failed to add SIM DISABLED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM CRASH DETECTION", "", types.SIMCONNECT_DATATYPE_INT32, 0, 21); err != nil {
				m.logger.Error("[manager] Failed to add REALISM CRASH DETECTION definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM CRASH WITH OTHERS", "", types.SIMCONNECT_DATATYPE_INT32, 0, 22); err != nil {
				m.logger.Error("[manager] Failed to add REALISM CRASH WITH OTHERS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "TRACK IR ENABLE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 23); err != nil {
				m.logger.Error("[manager] Failed to add TRACK IR ENABLE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "USER INPUT ENABLED", "", types.SIMCONNECT_DATATYPE_INT32, 0, 24); err != nil {
				m.logger.Error("[manager] Failed to add USER INPUT ENABLED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIM ON GROUND", "", types.SIMCONNECT_DATATYPE_INT32, 0, 25); err != nil {
				m.logger.Error("[manager] Failed to add SIM ON GROUND definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT TEMPERATURE", "Celsius", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 26); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT TEMPERATURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT PRESSURE", "inHg", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 27); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT WIND VELOCITY", "Knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 28); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT WIND VELOCITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT WIND DIRECTION", "Degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 29); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT WIND DIRECTION definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT VISIBILITY", "Meters", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 30); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT VISIBILITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT IN CLOUD", "", types.SIMCONNECT_DATATYPE_INT32, 0, 31); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT IN CLOUD definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT PRECIP STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 32); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT PRECIP STATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "BAROMETER PRESSURE", "Millibars", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 33); err != nil {
				m.logger.Error("[manager] Failed to add BAROMETER PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SEA LEVEL PRESSURE", "Millibars", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 34); err != nil {
				m.logger.Error("[manager] Failed to add SEA LEVEL PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "GROUND ALTITUDE", "Feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 35); err != nil {
				m.logger.Error("[manager] Failed to add GROUND ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "MAGVAR", "Degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 36); err != nil {
				m.logger.Error("[manager] Failed to add MAGVAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SURFACE TYPE", "Enum", types.SIMCONNECT_DATATYPE_INT32, 0, 37); err != nil {
				m.logger.Error("[manager] Failed to add SURFACE TYPE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 38); err != nil {
				m.logger.Error("[manager] Failed to add PLANE LATITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 39); err != nil {
				m.logger.Error("[manager] Failed to add PLANE LONGITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 40); err != nil {
				m.logger.Error("[manager] Failed to add PLANE ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "INDICATED ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 41); err != nil {
				m.logger.Error("[manager] Failed to add INDICATED ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 42); err != nil {
				m.logger.Error("[manager] Failed to add PLANE HEADING DEGREES TRUE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 43); err != nil {
				m.logger.Error("[manager] Failed to add PLANE HEADING DEGREES MAGNETIC definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 44); err != nil {
				m.logger.Error("[manager] Failed to add PLANE PITCH DEGREES definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 45); err != nil {
				m.logger.Error("[manager] Failed to add PLANE BANK DEGREES definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 46); err != nil {
				m.logger.Error("[manager] Failed to add GROUND VELOCITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 47); err != nil {
				m.logger.Error("[manager] Failed to add AIRSPEED INDICATED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 48); err != nil {
				m.logger.Error("[manager] Failed to add AIRSPEED TRUE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 49); err != nil {
				m.logger.Error("[manager] Failed to add VERTICAL SPEED definition", "error", err)
			}

			// Request camera data with period matching heartbeat configuration
			period := types.SIMCONNECT_PERIOD_SIM_FRAME
			m.requestRegistry.Register(m.cameraRequestID, RequestTypeDataRequest, "Camera State Data Request")
			if err := client.RequestDataOnSimObject(m.cameraRequestID, m.cameraDefinitionID, types.SIMCONNECT_OBJECT_ID_USER, period, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
				m.logger.Error("[manager] Failed to request camera data", "error", err)
			} else {
				m.mu.Lock()
				m.cameraDataRequestPending = true
				m.mu.Unlock()
				m.logger.Debug("[manager] Camera data request submitted")
			}
		}
		return
	}

	// Check for quit message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_QUIT {
		m.logger.Debug("[manager] Received QUIT message from simulator")
		quitData := types.ConnectionQuitData{}
		m.setQuit(quitData)
		m.setSimState(defaultSimState())
		m.setState(StateDisconnected)
		m.mu.Lock()
		m.engine = nil
		m.mu.Unlock()
		return
	}

	// Handle pause and sim events
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT {
		eventMsg := msg.AsEvent()
		switch eventMsg.UEventID {
		case types.DWORD(m.pauseEventID):
			// Handle pause event
			newPausedState := eventMsg.DwData == 1

			m.mu.Lock()
			if m.simState.Paused != newPausedState {
				oldState := m.simState
				m.simState.Paused = newPausedState
				newState := m.simState
				m.mu.Unlock()
				m.notifySimStateChange(oldState, newState)
			} else {
				m.mu.Unlock()
			}

		case types.DWORD(m.simEventID):
			// Handle sim running event
			newSimRunningState := eventMsg.DwData == 1

			m.mu.Lock()
			if m.simState.SimRunning != newSimRunningState {
				oldState := m.simState
				m.simState.SimRunning = newSimRunningState
				newState := m.simState
				m.mu.Unlock()
				m.notifySimStateChange(oldState, newState)
			} else {
				m.mu.Unlock()
			}

		case types.DWORD(m.crashedEventID):
			// Handle crashed event
			newCrashed := eventMsg.DwData == 1

			m.mu.Lock()
			if m.simState.Crashed != newCrashed {
				oldState := m.simState
				m.simState.Crashed = newCrashed
				newState := m.simState

				// Copy handlers under lock using pre-allocated buffer
				if cap(m.crashedHandlersBuf) < len(m.crashedHandlers) {
					m.crashedHandlersBuf = make([]CrashedHandler, len(m.crashedHandlers))
				} else {
					m.crashedHandlersBuf = m.crashedHandlersBuf[:len(m.crashedHandlers)]
				}
				for i, e := range m.crashedHandlers {
					m.crashedHandlersBuf[i] = e.fn
				}
				hs := m.crashedHandlersBuf
				m.mu.Unlock()

				m.notifySimStateChange(oldState, newState)

				// Invoke handlers outside lock with panic recovery
				for _, h := range hs {
					handler := h // capture for closure
					safeCallHandler(m.logger, "CrashedHandler", func() {
						handler()
					})
				}
			} else {
				m.mu.Unlock()
			}

		case types.DWORD(m.crashResetEventID):
			// Handle crash reset event
			newReset := eventMsg.DwData == 1

			m.mu.Lock()
			if m.simState.CrashReset != newReset {
				oldState := m.simState
				m.simState.CrashReset = newReset
				newState := m.simState

				// Copy handlers under lock using pre-allocated buffer
				if cap(m.crashResetHandlersBuf) < len(m.crashResetHandlers) {
					m.crashResetHandlersBuf = make([]CrashResetHandler, len(m.crashResetHandlers))
				} else {
					m.crashResetHandlersBuf = m.crashResetHandlersBuf[:len(m.crashResetHandlers)]
				}
				for i, e := range m.crashResetHandlers {
					m.crashResetHandlersBuf[i] = e.fn
				}
				hs := m.crashResetHandlersBuf
				m.mu.Unlock()

				m.notifySimStateChange(oldState, newState)

				// Invoke handlers outside lock with panic recovery
				for _, h := range hs {
					handler := h // capture for closure
					safeCallHandler(m.logger, "CrashResetHandler", func() {
						handler()
					})
				}
			} else {
				m.mu.Unlock()
			}

		case types.DWORD(m.soundEventID):
			// Handle sound event
			newSound := uint32(eventMsg.DwData)

			m.mu.Lock()
			if m.simState.Sound != newSound {
				oldState := m.simState
				m.simState.Sound = newSound
				newState := m.simState

				// Copy handlers under lock using pre-allocated buffer
				if cap(m.soundEventHandlersBuf) < len(m.soundEventHandlers) {
					m.soundEventHandlersBuf = make([]SoundEventHandler, len(m.soundEventHandlers))
				} else {
					m.soundEventHandlersBuf = m.soundEventHandlersBuf[:len(m.soundEventHandlers)]
				}
				for i, e := range m.soundEventHandlers {
					m.soundEventHandlersBuf[i] = e.fn
				}
				hs := m.soundEventHandlersBuf
				m.mu.Unlock()

				m.notifySimStateChange(oldState, newState)

				// Invoke handlers outside lock with panic recovery
				for _, h := range hs {
					handler := h      // capture for closure
					sound := newSound // capture for closure
					safeCallHandler(m.logger, "SoundEventHandler", func() {
						handler(sound)
					})
				}
			} else {
				m.mu.Unlock()
			}
		}
		// (Position change event handling removed)
	}

	// Handle filename events (FlightLoaded, AircraftLoaded, FlightPlanActivated)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_FILENAME {
		fnameMsg := msg.AsEventFilename()
		if fnameMsg != nil {
			name := engine.BytesToString(fnameMsg.SzFileName[:])
			if fnameMsg.UEventID == types.DWORD(m.flightLoadedEventID) {
				m.logger.Debug("[manager] FlightLoaded event", "filename", name)
				// Invoke registered FlightLoaded handlers with panic recovery
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.flightLoadedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightLoadedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightLoadedHandlers)]
				}
				for i, e := range m.flightLoadedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "FlightLoadedHandler", func() {
						handler(n)
					})
				}
			}

			if fnameMsg.UEventID == types.DWORD(m.aircraftLoadedEventID) {
				m.logger.Debug("[manager] AircraftLoaded event", "filename", name)
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.aircraftLoadedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.aircraftLoadedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.aircraftLoadedHandlers)]
				}
				for i, e := range m.aircraftLoadedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "AircraftLoadedHandler", func() {
						handler(n)
					})
				}
			}

			if fnameMsg.UEventID == types.DWORD(m.flightPlanActivatedEventID) {
				m.logger.Debug("[manager] FlightPlanActivated event", "filename", name)
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.flightPlanActivatedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightPlanActivatedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightPlanActivatedHandlers)]
				}
				for i, e := range m.flightPlanActivatedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "FlightPlanActivatedHandler", func() {
						handler(n)
					})
				}
			}
		}
	}

	// Handle object add/remove events (ObjectAdded, ObjectRemoved)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE {
		objMsg := msg.AsEventObjectAddRemove()
		if objMsg != nil {
			if objMsg.UEventID == types.DWORD(m.objectAddedEventID) {
				m.logger.Debug("[manager] ObjectAdded event", "id", objMsg.DwData, "type", objMsg.EObjType)
				// Invoke object added handlers with panic recovery
				m.mu.RLock()
				if cap(m.objectChangeHandlersBuf) < len(m.objectAddedHandlers) {
					m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectAddedHandlers))
				} else {
					m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectAddedHandlers)]
				}
				for i, e := range m.objectAddedHandlers {
					m.objectChangeHandlersBuf[i] = e.fn
				}
				hs := m.objectChangeHandlersBuf
				m.mu.RUnlock()
				objID := uint32(objMsg.DwData)
				objType := objMsg.EObjType
				for _, h := range hs {
					handler := h // capture for closure
					safeCallHandler(m.logger, "ObjectAddedHandler", func() {
						handler(objID, objType)
					})
				}
			}
			if objMsg.UEventID == types.DWORD(m.objectRemovedEventID) {
				m.logger.Debug("[manager] ObjectRemoved event", "id", objMsg.DwData, "type", objMsg.EObjType)
				m.mu.RLock()
				if cap(m.objectChangeHandlersBuf) < len(m.objectRemovedHandlers) {
					m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectRemovedHandlers))
				} else {
					m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectRemovedHandlers)]
				}
				for i, e := range m.objectRemovedHandlers {
					m.objectChangeHandlersBuf[i] = e.fn
				}
				hs := m.objectChangeHandlersBuf
				m.mu.RUnlock()
				objID := uint32(objMsg.DwData)
				objType := objMsg.EObjType
				for _, h := range hs {
					handler := h // capture for closure
					safeCallHandler(m.logger, "ObjectRemovedHandler", func() {
						handler(objID, objType)
					})
				}
			}
		}
	}

	// Handle camera state data
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		simObjMsg := msg.AsSimObjectData()
		if uint32(simObjMsg.DwRequestID) == m.cameraRequestID && uint32(simObjMsg.DwDefineID) == m.cameraDefinitionID {
			// Extract camera state and substate from data
			stateData := engine.CastDataAs[simStateDataStruct](&simObjMsg.DwData)

			// Build new state from sim state data (no lock needed)
			newState := SimState{
				Camera:                   CameraState(stateData.CameraState),
				Substate:                 CameraSubstate(stateData.CameraSubstate),
				SimulationRate:           stateData.SimulationRate,
				SimulationTime:           stateData.SimulationTime,
				LocalTime:                stateData.LocalTime,
				ZuluTime:                 stateData.ZuluTime,
				IsInVR:                   stateData.IsInVR == 1,
				IsUsingMotionControllers: stateData.IsUsingMotionControllers == 1,
				IsUsingJoystickThrottle:  stateData.IsUsingJoystickThrottle == 1,
				IsInRTC:                  stateData.IsInRTC == 1,
				IsAvatar:                 stateData.IsAvatar == 1,
				IsAircraft:               stateData.IsAircraft == 1,
				LocalDay:                 int(stateData.LocalDay),
				LocalMonth:               int(stateData.LocalMonth),
				LocalYear:                int(stateData.LocalYear),
				ZuluDay:                  int(stateData.ZuluDay),
				ZuluMonth:                int(stateData.ZuluMonth),
				ZuluYear:                 int(stateData.ZuluYear),
				Realism:                  stateData.Realism,
				VisualModelRadius:        stateData.VisualModelRadius,
				SimDisabled:              stateData.SimDisabled == 1,
				RealismCrashDetection:    stateData.RealismCrashDetection == 1,
				RealismCrashWithOthers:   stateData.RealismCrashWithOthers == 1,
				TrackIREnabled:           stateData.TrackIREnabled == 1,
				UserInputEnabled:         stateData.UserInputEnabled == 1,
				SimOnGround:              stateData.SimOnGround == 1,
				AmbientTemperature:       stateData.AmbientTemperature,
				AmbientPressure:          stateData.AmbientPressure,
				AmbientWindVelocity:      stateData.AmbientWindVelocity,
				AmbientWindDirection:     stateData.AmbientWindDirection,
				AmbientVisibility:        stateData.AmbientVisibility,
				AmbientInCloud:           stateData.AmbientInCloud == 1,
				AmbientPrecipState:       uint32(stateData.AmbientPrecipState),
				BarometerPressure:        stateData.BarometerPressure,
				SeaLevelPressure:         stateData.SeaLevelPressure,
				GroundAltitude:           stateData.GroundAltitude,
				MagVar:                   stateData.MagVar,
				SurfaceType:              uint32(stateData.SurfaceType),
				Latitude:                 stateData.Latitude,
				Longitude:                stateData.Longitude,
				Altitude:                 stateData.Altitude,
				IndicatedAltitude:        stateData.IndicatedAltitude,
				TrueHeading:              stateData.TrueHeading,
				MagneticHeading:          stateData.MagneticHeading,
				Pitch:                    stateData.Pitch,
				Bank:                     stateData.Bank,
				GroundSpeed:              stateData.GroundSpeed,
				IndicatedAirspeed:        stateData.IndicatedAirspeed,
				TrueAirspeed:             stateData.TrueAirspeed,
				VerticalSpeed:            stateData.VerticalSpeed,
			}

			// Short lock: preserve event-driven fields and compare
			m.mu.Lock()
			newState.Paused = m.simState.Paused
			newState.SimRunning = m.simState.SimRunning
			newState.Crashed = m.simState.Crashed
			newState.CrashReset = m.simState.CrashReset
			newState.Sound = m.simState.Sound

			if m.simState == newState {
				m.mu.Unlock()
				return
			}
			oldState := m.simState
			m.simState = newState
			m.mu.Unlock()

			m.notifySimStateChange(oldState, newState)
		}
	}

	// Forward message to registered handlers
	m.mu.RLock()
	// Reuse pre-allocated slices, grow if necessary
	if cap(m.handlersBuf) < len(m.messageHandlers) {
		m.handlersBuf = make([]MessageHandler, len(m.messageHandlers))
	} else {
		m.handlersBuf = m.handlersBuf[:len(m.messageHandlers)]
	}
	for i, e := range m.messageHandlers {
		m.handlersBuf[i] = e.fn
	}
	if cap(m.subsBuf) < len(m.subscriptions) {
		m.subsBuf = make([]*subscription, 0, len(m.subscriptions))
	} else {
		m.subsBuf = m.subsBuf[:0]
	}
	for _, sub := range m.subscriptions {
		m.subsBuf = append(m.subsBuf, sub)
	}
	m.mu.RUnlock()

	for _, handler := range m.handlersBuf {
		h := handler   // capture for closure
		message := msg // capture for closure
		safeCallHandler(m.logger, "MessageHandler", func() {
			h(message)
		})
	}

	// Forward message to subscriptions (non-blocking)
	for _, sub := range m.subsBuf {
		// fast-path: skip closed subscriptions
		sub.closeMu.Lock()
		closed := sub.closed.Load()
		sub.closeMu.Unlock()
		if closed {
			continue
		}

		// Determine whether this subscription should receive the message
		allowed := true
		if sub.filter != nil {
			// Protect against panics in user-provided filters
			func() {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("[manager] Subscription filter panic", "panic", r)
						allowed = false
					}
				}()
				allowed = sub.filter(msg)
			}()
		} else if len(sub.allowedTypes) > 0 {
			_, ok := sub.allowedTypes[types.SIMCONNECT_RECV_ID(msg.DwID)]
			allowed = ok
		}

		if !allowed {
			continue
		}

		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- msg:
			default:
				// Channel full, skip message to avoid blocking
				if sub.onDrop != nil {
					// Protect dispatch loop from user callback panics
					func() {
						defer func() {
							if r := recover(); r != nil {
								m.logger.Error("[manager] OnDrop callback panicked", "panic", r)
							}
						}()
						sub.onDrop(1)
					}()
				}
				m.logger.Debug("[manager] Subscription channel full, dropping message")
			}
		}
		sub.closeMu.Unlock()
	}
}
