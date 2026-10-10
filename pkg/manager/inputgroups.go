//go:build windows
// +build windows

package manager

// MapInputEventToClientEvent maps a key or joystick input to client events
// (see engine.MapInputEventToClientEvent). Returns ErrNotConnected if not
// connected to the simulator.
func (m *Instance) MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.MapInputEventToClientEvent(groupID, definition, downEventID, downValue, upEventID, upValue, maskable)
}

// SetInputGroupPriority sets an input group's priority. Returns
// ErrNotConnected if not connected to the simulator.
func (m *Instance) SetInputGroupPriority(groupID uint32, priority uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetInputGroupPriority(groupID, priority)
}

// SetInputGroupState turns an input group on (1) or off (0). Returns
// ErrNotConnected if not connected to the simulator.
func (m *Instance) SetInputGroupState(groupID uint32, state uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetInputGroupState(groupID, state)
}

// RemoveInputEvent removes an input from an input group. Returns
// ErrNotConnected if not connected to the simulator.
func (m *Instance) RemoveInputEvent(groupID uint32, definition string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RemoveInputEvent(groupID, definition)
}

// ClearInputGroup removes every input of an input group. Returns
// ErrNotConnected if not connected to the simulator.
func (m *Instance) ClearInputGroup(groupID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.ClearInputGroup(groupID)
}
