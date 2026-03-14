//go:build windows
// +build windows

package manager

// EnumerateInputEvents requests an enumeration of all input events registered
// in the simulator. Results are delivered as SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS messages.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) EnumerateInputEvents(requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.EnumerateInputEvents(requestID)
}

// GetInputEvent requests the current value of an input event identified by its hash.
// The result is delivered as a SIMCONNECT_RECV_GET_INPUT_EVENT message.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) GetInputEvent(requestID uint32, hash uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.GetInputEvent(requestID, hash)
}

// SetInputEventDouble sets a DOUBLE-typed input event value identified by its hash.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SetInputEventDouble(hash uint64, value float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetInputEventDouble(hash, value)
}

// SetInputEventString sets a STRING-typed input event value identified by its hash.
// Strings longer than 259 bytes are silently truncated to preserve the null terminator.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SetInputEventString(hash uint64, value string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetInputEventString(hash, value)
}

// SubscribeInputEvent subscribes to change notifications for the input event
// identified by the given hash. Updates are delivered as
// SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT messages.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SubscribeInputEvent(hash uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SubscribeInputEvent(hash)
}

// UnsubscribeInputEvent cancels the subscription for the input event
// identified by the given hash.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) UnsubscribeInputEvent(hash uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.UnsubscribeInputEvent(hash)
}
