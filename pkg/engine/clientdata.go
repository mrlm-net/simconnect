//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// CreateClientData registers a client data area with the given ID and size.
// dwSize must be between 1 and 8192 bytes; the SimConnect SDK returns an HRESULT error if exceeded.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_CreateClientData.htm
func (e *Engine) CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error {
	return e.api.CreateClientData(clientDataID, dwSize, flags)
}

// AddToClientDataDefinition adds a data field to a client data definition.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToClientDataDefinition.htm
func (e *Engine) AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error {
	return e.api.AddToClientDataDefinition(defineID, dwOffset, dwSizeOrType, epsilon, datumID)
}

// RequestClientData subscribes to client data area updates for the given definition.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestClientData.htm
func (e *Engine) RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	return e.api.RequestClientData(clientDataID, requestID, defineID, period, flags, origin, interval, limit)
}

// ClearClientDataDefinition removes all data definitions for the given client data definition ID.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearClientDataDefinition.htm
func (e *Engine) ClearClientDataDefinition(defineID uint32) error {
	return e.api.ClearClientDataDefinition(defineID)
}

// SetClientData writes data to a client data area.
// flags is a plain uint32 per ADR-B-01 — SimConnect.h does not define a typed enum for SetClientData flags.
// dwReserved must be 0.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetClientData.htm
func (e *Engine) SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	return e.api.SetClientData(clientDataID, defineID, flags, dwReserved, cbUnitSize, data)
}
