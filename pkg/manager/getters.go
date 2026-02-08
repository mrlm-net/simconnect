//go:build windows

package manager

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

// State returns the current connection state
func (m *Instance) ConnectionState() ConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SimState returns the current simulator state
func (m *Instance) SimState() SimState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.simState
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
