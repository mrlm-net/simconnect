//go:build windows
// +build windows

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
		simState:                     SimState{Camera: CameraStateUninitialized, Paused: false},
		simStateHandlers:             []simStateHandlerEntry{},
		simStateSubscriptions:        make(map[string]*simStateSubscription),
		cameraDefinitionID:           cameraDefinitionID,
		cameraRequestID:              cameraRequestID,
		pauseEventID:                 1000,
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
	simState                 SimState
	simStateHandlers         []simStateHandlerEntry
	simStateSubscriptions    map[string]*simStateSubscription
	simStateSubsWg           sync.WaitGroup // WaitGroup for graceful shutdown of simulator state subscriptions
	cameraDefinitionID       uint32
	cameraRequestID          uint32
	cameraDataRequestPending bool
	pauseEventID             uint32

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
	handlers := make([]ConnectionStateChangeHandler, len(m.stateHandlers))
	for i, e := range m.stateHandlers {
		handlers[i] = e.fn
	}
	stateSubs := make([]*connectionStateSubscription, 0, len(m.connectionStateSubscriptions))
	for _, sub := range m.connectionStateSubscriptions {
		stateSubs = append(stateSubs, sub)
	}
	m.mu.Unlock()

	m.logger.Debug(fmt.Sprintf("[manager] State changed: %s -> %s", oldState, newState))

	// Notify handlers outside the lock
	for _, handler := range handlers {
		handler(oldState, newState)
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := ConnectionStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed {
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
	handlers := make([]SimStateChangeHandler, len(m.simStateHandlers))
	for i, e := range m.simStateHandlers {
		handlers[i] = e.fn
	}
	stateSubs := make([]*simStateSubscription, 0, len(m.simStateSubscriptions))
	for _, sub := range m.simStateSubscriptions {
		stateSubs = append(stateSubs, sub)
	}
	m.mu.Unlock()

	m.logger.Debug(fmt.Sprintf("[manager] SimState changed: Camera %s -> %s", oldState.Camera, newState.Camera))

	// Notify handlers outside the lock
	for _, handler := range handlers {
		handler(oldState, newState)
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := SimStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed {
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
		m.logger.Debug(fmt.Sprintf("[manager] Registered state handler: %s", id))
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
				m.logger.Debug(fmt.Sprintf("[manager] Removed state handler: %s", id))
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
		m.logger.Debug(fmt.Sprintf("[manager] Registered message handler: %s", id))
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
				m.logger.Debug(fmt.Sprintf("[manager] Removed message handler: %s", id))
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
		m.logger.Debug(fmt.Sprintf("[manager] Registered open handler: %s", id))
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
				m.logger.Debug(fmt.Sprintf("[manager] Removed open handler: %s", id))
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
		m.logger.Debug(fmt.Sprintf("[manager] Registered quit handler: %s", id))
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
				m.logger.Debug(fmt.Sprintf("[manager] Removed quit handler: %s", id))
			}
			return nil
		}
	}
	return fmt.Errorf("quit handler not found: %s", id)
}

// setOpen invokes all registered open handlers and sends to subscriptions
func (m *Instance) setOpen(data types.ConnectionOpenData) {
	m.mu.Lock()
	handlers := make([]ConnectionOpenHandler, len(m.openHandlers))
	for i, e := range m.openHandlers {
		handlers[i] = e.fn
	}
	openSubs := make([]*connectionOpenSubscription, 0, len(m.openSubscriptions))
	for _, sub := range m.openSubscriptions {
		openSubs = append(openSubs, sub)
	}
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection opened")

	// Notify handlers outside the lock
	for _, handler := range handlers {
		handler(data)
	}

	// Forward open event to subscriptions (non-blocking)
	for _, sub := range openSubs {
		sub.closeMu.Lock()
		if !sub.closed {
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
	handlers := make([]ConnectionQuitHandler, len(m.quitHandlers))
	for i, e := range m.quitHandlers {
		handlers[i] = e.fn
	}
	quitSubs := make([]*connectionQuitSubscription, 0, len(m.quitSubscriptions))
	for _, sub := range m.quitSubscriptions {
		quitSubs = append(quitSubs, sub)
	}
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection quit")

	// Notify handlers outside the lock
	for _, handler := range handlers {
		handler(data)
	}

	// Forward quit event to subscriptions (non-blocking)
	for _, sub := range quitSubs {
		sub.closeMu.Lock()
		if !sub.closed {
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
		m.logger.Debug(fmt.Sprintf("[manager] Registered SimState handler: %s", id))
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
				m.logger.Debug(fmt.Sprintf("[manager] Removed SimState handler: %s", id))
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
			m.logger.Debug(fmt.Sprintf("[manager] Connection ended with error: %v", err))
			return err
		}

		// Simulator disconnected (err == nil) - check if we should reconnect
		if !m.config.AutoReconnect {
			m.logger.Debug("[manager] Auto-reconnect disabled, stopping manager")
			return nil
		}

		m.setState(StateReconnecting)
		m.logger.Debug(fmt.Sprintf("[manager] Waiting %v before reconnecting...", m.config.ReconnectDelay))

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
	// (excluding any Context or Logger options as manager controls these)
	opts := []engine.Option{engine.WithContext(m.ctx)}
	for _, eo := range m.config.EngineOptions {
		// Heuristic: try to detect if option is a Context or Logger wrapper by
		// inspecting the underlying function name. This is best-effort and
		// only intended to warn users that such options will be overridden.
		if eo != nil {
			pc := reflect.ValueOf(eo).Pointer()
			if fn := runtime.FuncForPC(pc); fn != nil {
				name := fn.Name()
				if strings.Contains(name, "WithContext") {
					m.logger.Warn("[manager] EngineOptions contains WithContext; this will be overridden by manager context.")
				}
				if strings.Contains(name, "WithLogger") {
					m.logger.Warn("[manager] EngineOptions contains WithLogger; this will be overridden by manager logger.")
				}
			}
		}
		opts = append(opts, eo)
	}
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
				m.setSimState(SimState{Camera: CameraStateUninitialized, Substate: CameraSubstateUninitialized, Paused: false})
				m.setState(StateDisconnected)
				m.mu.Lock()
				m.engine = nil
				m.mu.Unlock()
				return nil // Return nil to allow reconnection
			}

			if msg.Err != nil {
				m.logger.Error(fmt.Sprintf("[manager] Stream error: %v", msg.Err))
				continue
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
					m.setSimState(SimState{Camera: CameraStateUninitialized, Substate: CameraSubstateUninitialized, Paused: false})

					// Subscribe to pause events
					if err := client.SubscribeToSystemEvent(m.pauseEventID, "Pause"); err != nil {
						m.logger.Error(fmt.Sprintf("[manager] Failed to subscribe to Pause event: %v", err))
					}

					// Define camera data structure
					if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0); err != nil {
						m.logger.Error(fmt.Sprintf("[manager] Failed to add CAMERA STATE definition: %v", err))
					}
					if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1); err != nil {
						m.logger.Error(fmt.Sprintf("[manager] Failed to add CAMERA SUBSTATE definition: %v", err))
					}

					// Request camera data with period matching heartbeat configuration
					period := types.SIMCONNECT_PERIOD_SECOND
					if err := client.RequestDataOnSimObject(m.cameraRequestID, m.cameraDefinitionID, types.SIMCONNECT_OBJECT_ID_USER, period, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
						m.logger.Error(fmt.Sprintf("[manager] Failed to request camera data: %v", err))
					} else {
						m.mu.Lock()
						m.cameraDataRequestPending = true
						m.mu.Unlock()
						m.logger.Debug("[manager] Camera data request submitted")
					}
				}
				continue
			}

			// Check for quit message
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_QUIT {
				m.logger.Debug("[manager] Received QUIT message from simulator")
				quitData := types.ConnectionQuitData{}
				m.setQuit(quitData)
				m.setSimState(SimState{Camera: CameraStateUninitialized, Substate: CameraSubstateUninitialized, Paused: false})
				m.setState(StateDisconnected)
				m.mu.Lock()
				m.engine = nil
				m.mu.Unlock()
				return nil // Return nil to allow reconnection
			}

			// Handle pause event
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT {
				eventMsg := msg.AsEvent()
				if eventMsg.UEventID == types.DWORD(m.pauseEventID) {
					newPausedState := eventMsg.DwData == 1

					m.mu.RLock()
					oldPausedState := m.simState.Paused
					m.mu.RUnlock()

					if oldPausedState != newPausedState {
						newSimState := SimState{Camera: m.simState.Camera, Substate: m.simState.Substate, Paused: newPausedState}
						m.setSimState(newSimState)
					}
				}
			}

			// Handle camera state data
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
				simObjMsg := msg.AsSimObjectData()
				if uint32(simObjMsg.DwRequestID) == m.cameraRequestID && uint32(simObjMsg.DwDefineID) == m.cameraDefinitionID {
					// Extract camera state and substate from data
					cameraData := engine.CastDataAs[cameraDataStruct](&simObjMsg.DwData)
					newCameraState := CameraState(cameraData.CameraState)
					newCameraSubstate := CameraSubstate(cameraData.CameraSubstate)

					m.mu.RLock()
					oldCameraState := m.simState.Camera
					oldCameraSubstate := m.simState.Substate
					oldPausedState := m.simState.Paused
					m.mu.RUnlock()

					if oldCameraState != newCameraState || oldCameraSubstate != newCameraSubstate {
						newSimState := SimState{Camera: newCameraState, Substate: newCameraSubstate, Paused: oldPausedState}
						m.setSimState(newSimState)
					}
				}
			}

			// Forward message to registered handlers
			m.mu.RLock()
			handlers := make([]MessageHandler, len(m.messageHandlers))
			for i, e := range m.messageHandlers {
				handlers[i] = e.fn
			}
			subs := make([]*subscription, 0, len(m.subscriptions))
			for _, sub := range m.subscriptions {
				subs = append(subs, sub)
			}
			m.mu.RUnlock()

			for _, handler := range handlers {
				handler(msg)
			}

			// Forward message to subscriptions (non-blocking)
			for _, sub := range subs {
				// fast-path: skip closed subscriptions
				sub.closeMu.Lock()
				closed := sub.closed
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
								m.logger.Error(fmt.Sprintf("[manager] Subscription filter panic: %v", r))
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
				if !sub.closed {
					select {
					case sub.ch <- msg:
					default:
						// Channel full, skip message to avoid blocking
						m.logger.Debug("[manager] Subscription channel full, dropping message")
					}
				}
				sub.closeMu.Unlock()
			}
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

		m.logger.Debug(fmt.Sprintf("[manager] Connection attempt %d failed: %v, retrying in %v...", attempts, err, m.config.RetryInterval))

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
				m.logger.Error(fmt.Sprintf("[manager] Failed to clear camera data definition: %v", err))
			}
		}

		if err := eng.Disconnect(); err != nil {
			m.logger.Error(fmt.Sprintf("[manager] Disconnect error: %v", err))
		}
	}
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
		m.logger.Warn(fmt.Sprintf("[manager] Shutdown timeout (%v) exceeded, some subscriptions may not have closed gracefully", m.config.ShutdownTimeout))
	}

	m.disconnect()
	return nil
}
