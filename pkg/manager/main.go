//go:build windows
// +build windows

package manager

import (
	"context"
	"fmt"
	"log/slog"
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

	ctx, cancel := context.WithCancel(config.Context)

	return &Instance{
		name:               name,
		config:             config,
		ctx:                ctx,
		cancel:             cancel,
		logger:             config.Logger,
		state:              StateDisconnected,
		stateHandlers:      []StateChangeHandler{},
		messageHandlers:    []MessageHandler{},
		subscriptions:      make(map[string]*subscription),
		stateSubscriptions: make(map[string]*stateSubscription),
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
	mu                 sync.RWMutex
	state              ConnectionState
	stateHandlers      []StateChangeHandler
	messageHandlers    []MessageHandler
	subscriptions      map[string]*subscription
	subsWg             sync.WaitGroup // WaitGroup for graceful shutdown of subscriptions
	stateSubscriptions map[string]*stateSubscription
	stateSubsWg        sync.WaitGroup // WaitGroup for graceful shutdown of state subscriptions

	// Current engine instance (recreated on each connection)
	engine *engine.Engine
}

// State returns the current connection state
func (m *Instance) State() ConnectionState {
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
	handlers := make([]StateChangeHandler, len(m.stateHandlers))
	copy(handlers, m.stateHandlers)
	stateSubs := make([]*stateSubscription, 0, len(m.stateSubscriptions))
	for _, sub := range m.stateSubscriptions {
		stateSubs = append(stateSubs, sub)
	}
	m.mu.Unlock()

	m.logger.Debug(fmt.Sprintf("[manager] State changed: %s -> %s", oldState, newState))

	// Notify handlers outside the lock
	for _, handler := range handlers {
		handler(oldState, newState)
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := StateChange{OldState: oldState, NewState: newState}
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

// OnStateChange registers a callback to be invoked when connection state changes
func (m *Instance) OnStateChange(handler StateChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateHandlers = append(m.stateHandlers, handler)
}

// OnMessage registers a callback to be invoked when a message is received
func (m *Instance) OnMessage(handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messageHandlers = append(m.messageHandlers, handler)
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
				continue
			}

			// Check for quit message
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_QUIT {
				m.logger.Debug("[manager] Received QUIT message from simulator")
				m.setState(StateDisconnected)
				m.mu.Lock()
				m.engine = nil
				m.mu.Unlock()
				return nil // Return nil to allow reconnection
			}

			// Forward message to registered handlers
			m.mu.RLock()
			handlers := make([]MessageHandler, len(m.messageHandlers))
			copy(handlers, m.messageHandlers)
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
	m.mu.Unlock()

	if eng != nil {
		if err := eng.Disconnect(); err != nil {
			m.logger.Error(fmt.Sprintf("[manager] Disconnect error: %v", err))
		}
	}
	m.setState(StateDisconnected)
}

// Stop gracefully shuts down the manager
func (m *Instance) Stop() error {
	m.logger.Info("[manager] Stopping manager")
	m.cancel() // This will trigger all subscription context watchers

	// Wait for all subscriptions to close with timeout
	m.logger.Debug("[manager] Waiting for subscriptions to close...")
	done := make(chan struct{})
	go func() {
		m.subsWg.Wait()
		m.stateSubsWg.Wait()
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
