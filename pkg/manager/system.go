//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/types"

// RequestSystemState requests a system state value from the simulator.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestSystemState(requestID, state)
}

// SubscribeToSystemEvent subscribes to a SimConnect system event.
//
// WARNING: Do not use event IDs in the manager's reserved range (999,999,900 - 999,999,999).
// The manager uses these IDs internally for its own system event subscriptions.
// Use IDs from 1 to 999,999,899 for your own subscriptions.
// See pkg/manager/ids.go for the full ID allocation reference.
//
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SubscribeToSystemEvent(eventID, eventName)
}

// UnsubscribeFromSystemEvent unsubscribes from a SimConnect system event.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) UnsubscribeFromSystemEvent(eventID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.UnsubscribeFromSystemEvent(eventID)
}
