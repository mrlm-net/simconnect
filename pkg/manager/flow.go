//go:build windows
// +build windows

package manager

// SubscribeToFlowEvent subscribes to all simulator flow events.
// Once subscribed, SIMCONNECT_RECV_FLOW_EVENT messages are delivered to
// all active message subscriptions and OnMessage handlers.
//
// Important: there is no automatic re-subscription on reconnect — it is the
// caller's responsibility to call SubscribeToFlowEvent again after a
// reconnection event if persistent flow event delivery is required.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SubscribeToFlowEvent() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SubscribeToFlowEvent()
}

// UnsubscribeFromFlowEvent cancels the active flow event subscription.
// After this call, SIMCONNECT_RECV_FLOW_EVENT messages will no longer be delivered.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) UnsubscribeFromFlowEvent() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.UnsubscribeFromFlowEvent()
}
