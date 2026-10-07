//go:build windows

package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// Start begins the connection lifecycle management
func (m *Instance) Start() error {
	m.logger.Debug("[manager] Starting connection lifecycle management")

	// Reconnection loop: reconnect is true once a connection has been made
	// and lost (ReconnectMaxRetries applies from then on)
	reconnect := false
	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("[manager] Context cancelled, stopping manager")
			m.setState(StateDisconnected)
			return m.ctx.Err()
		default:
		}

		err := m.runConnection(reconnect)
		reconnect = true
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
func (m *Instance) runConnection(reconnect bool) error {
	// Create a new engine instance for this connection
	m.mu.Lock()
	m.engine = m.newEngineLocked()
	m.mu.Unlock()

	// Attempt to connect with retry
	maxRetries := m.config.MaxRetries
	if reconnect && m.config.ReconnectMaxRetries >= 0 {
		maxRetries = m.config.ReconnectMaxRetries
	}
	if err := m.connectWithRetry(maxRetries); err != nil {
		m.mu.Lock()
		m.engine = nil
		m.mu.Unlock()
		m.fleet.SetClient(nil)
		return err
	}

	m.setState(StateConnected)
	m.logger.Debug("[manager] Connected to simulator")
	m.fleet.SetClient(m.engine)

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
				m.connectionLost()
				return nil // Return nil to allow reconnection
			}

			// Process message in separate method to ensure proper defer handling
			m.processMessage(msg)
		}
	}
}

// connectWithRetry attempts to connect to the simulator with fixed retry
// interval, at most maxRetries attempts (0 = unlimited)
func (m *Instance) connectWithRetry(maxRetries int) error {
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
		if maxRetries > 0 && attempts >= maxRetries {
			m.setState(StateDisconnected)
			return fmt.Errorf("max connection retries (%d) exceeded: %w", maxRetries, err)
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

// connectWithTimeout attempts a single connection with timeout. An attempt
// that times out keeps running (the DLL call cannot be cancelled): its
// engine is left to it and a new engine takes the next attempt, so the two
// never race on one engine, and a late success is disconnected rather
// than leaking a SimConnect handle (review #33).
func (m *Instance) connectWithTimeout(ctx context.Context) error {
	m.mu.RLock()
	eng := m.engine
	m.mu.RUnlock()
	if eng == nil {
		return ErrNotConnected
	}

	done := make(chan error, 1)

	go func() {
		done <- eng.Connect()
	}()

	select {
	case <-ctx.Done():
		m.mu.Lock()
		if m.engine == eng {
			m.engine = m.newEngineLocked()
		}
		m.mu.Unlock()
		go m.discardLateConnect(eng, done)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// lateConnector is the part of an engine discardLateConnect uses.
type lateConnector interface {
	Disconnect() error
}

// discardLateConnect waits for the result of an abandoned connection
// attempt on eng and disconnects it if it succeeded after all (review #33).
func (m *Instance) discardLateConnect(eng lateConnector, done <-chan error) {
	if err := <-done; err != nil {
		return
	}
	m.logger.Debug("[manager] A timed-out connection attempt succeeded late, disconnecting it")
	if err := eng.Disconnect(); err != nil {
		m.logger.Error("[manager] Disconnect of a late connection", "error", err)
	}
}

// newEngineLocked is a new engine for a connection, with the manager's
// context, the configured engine options and the manager's logger; it
// starts a new connection generation (connGen). m.mu must be held.
func (m *Instance) newEngineLocked() *engine.Engine {
	// Create engine options: start with manager's context, then add user options
	opts := []engine.Option{engine.WithContext(m.ctx)}
	opts = append(opts, m.config.EngineOptions...)
	// Manager's logger always takes precedence over any logger in EngineOptions
	if m.config.Logger != nil {
		opts = append(opts, engine.WithLogger(m.config.Logger))
	}
	m.connGen++
	return engine.New(m.name, opts...)
}

// connectionLost is the simulator gone (its stream closed, #405): what
// belonged to that connection is cleared as disconnect clears it — the
// camera request, the request registry — before the reconnect loop. What the
// application registered (custom system events with their subscriptions and
// handlers) is kept and subscribed again on the next connection with the
// same IDs (resubscribeCustomEvents), so their filters still match.
func (m *Instance) connectionLost() {
	m.setSimState(defaultSimState())
	m.mu.Lock()
	eng := m.engine
	if eng == nil {
		eng = m.quitEngine // the simulator quit first (QUIT)
	}
	m.engine, m.quitEngine = nil, nil
	m.cameraDataRequestPending = false
	m.mu.Unlock()
	// Its stream has ended, the dispatcher with it: the handle is closed
	// and the engine's goroutines waited for (the review of #405: it was
	// left open on every lost connection).
	if eng != nil {
		if err := eng.Disconnect(); err != nil {
			m.logger.Error("[manager] Disconnect after the connection was lost", "error", err)
		}
	}
	m.fleet.SetClient(nil)
	m.requestRegistry.Clear()
	m.setState(StateDisconnected)
}

// resubscribeCustomEvents subscribes the custom system events kept over a
// lost connection again on client, with their IDs (#405).
func (m *Instance) resubscribeCustomEvents(client systemEventSubscriber) {
	m.mu.Lock()
	gen := m.connGen
	evs := make([]instance.CustomSystemEvent, 0, len(m.customSystemEvents))
	for _, ce := range m.customSystemEvents {
		// Subscribed on this connection already (before its OPEN): not twice (review #32)
		if gen != 0 && ce.Conn == gen {
			continue
		}
		evs = append(evs, *ce)
	}
	m.mu.Unlock()
	for _, ce := range evs {
		if err := client.SubscribeToSystemEvent(ce.ID, ce.Name); err != nil {
			m.logger.Error("[manager] Failed to subscribe a custom system event again", "event", ce.Name, "error", err)
			continue
		}
		m.mu.Lock()
		if cur, ok := m.customSystemEvents[ce.Name]; ok && cur.ID == ce.ID {
			cur.Conn = gen
		}
		m.mu.Unlock()
	}
}

// systemEventSubscriber is the part of engine.Client resubscribeCustomEvents
// uses.
type systemEventSubscriber interface {
	SubscribeToSystemEvent(eventID uint32, eventName string) error
}

// disconnect gracefully disconnects from the simulator
func (m *Instance) disconnect() {
	m.mu.Lock()
	eng := m.engine
	if eng == nil {
		eng = m.quitEngine
	}
	m.engine, m.quitEngine = nil, nil
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

	// Stop clears the custom system events (a lost connection keeps them, #405)
	m.mu.Lock()
	m.customSystemEvents = make(map[string]*instance.CustomSystemEvent)
	m.customEventIDAlloc = CustomEventIDMin
	m.customEventSubs = make(map[string][]*subscription)
	m.mu.Unlock()
	m.userSubs.reset()

	// and the request registry.
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
		// the open and quit subscriptions too (review #35)
		m.openSubsWg.Wait()
		m.quitSubsWg.Wait()
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
