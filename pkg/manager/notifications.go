//go:build windows
// +build windows

package manager

// AddClientEventToNotificationGroup adds a client event to a notification group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AddClientEventToNotificationGroup(groupID, eventID, mask)
}

// ClearNotificationGroup clears all events from a notification group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) ClearNotificationGroup(groupID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.ClearNotificationGroup(groupID)
}

// RequestNotificationGroup requests a notification group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestNotificationGroup(groupID, dwReserved, flags)
}

// SetNotificationGroupPriority sets the priority of a notification group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SetNotificationGroupPriority(groupID uint32, priority uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetNotificationGroupPriority(groupID, priority)
}
