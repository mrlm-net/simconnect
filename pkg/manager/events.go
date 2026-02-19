//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/types"

// MapClientEventToSimEvent maps a client event ID to a SimConnect event name.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) MapClientEventToSimEvent(eventID uint32, eventName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.MapClientEventToSimEvent(eventID, eventName)
}

// RemoveClientEvent removes a client event from a notification group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RemoveClientEvent(groupID uint32, eventID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RemoveClientEvent(groupID, eventID)
}

// TransmitClientEvent transmits a client event to the simulator.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.TransmitClientEvent(objectID, eventID, data, groupID, flags)
}

// TransmitClientEventEx1 transmits a client event with extended data to the simulator.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.TransmitClientEventEx1(objectID, eventID, groupID, flags, data)
}

// MapClientDataNameToID maps a client data name to a client data ID.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) MapClientDataNameToID(clientDataName string, clientDataID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.MapClientDataNameToID(clientDataName, clientDataID)
}
