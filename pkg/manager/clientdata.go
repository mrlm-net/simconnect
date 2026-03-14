//go:build windows
// +build windows

package manager

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// CreateClientData registers a client data area with the given ID and size.
// dwSize must be between 1 and 8192 bytes; the SimConnect SDK returns an HRESULT error if exceeded.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.CreateClientData(clientDataID, dwSize, flags)
}

// AddToClientDataDefinition adds a data field to a client data definition.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AddToClientDataDefinition(defineID, dwOffset, dwSizeOrType, epsilon, datumID)
}

// ClearClientDataDefinition removes all data definitions for the given client data definition ID.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) ClearClientDataDefinition(defineID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.ClearClientDataDefinition(defineID)
}

// RequestClientData subscribes to client data area updates for the given definition.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestClientData(clientDataID, requestID, defineID, period, flags, origin, interval, limit)
}

// SetClientData writes data to a client data area.
// flags is a plain uint32 per ADR-B-01 — SimConnect.h does not define a typed enum for SetClientData flags.
// dwReserved must be 0.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetClientData(clientDataID, defineID, flags, dwReserved, cbUnitSize, data)
}
