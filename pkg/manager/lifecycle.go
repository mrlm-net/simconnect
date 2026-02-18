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

	// Clear custom system events on disconnect
	m.mu.Lock()
	m.customSystemEvents = make(map[string]*instance.CustomSystemEvent)
	m.customEventIDAlloc = CustomEventIDMin
	m.mu.Unlock()

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
