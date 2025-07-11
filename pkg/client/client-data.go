//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// MapClientDataNameToID associates an ID with a named client data area.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_MapClientDataNameToID.htm
func (e *Engine) MapClientDataNameToID(clientDataName string, clientDataID int) error {
	// Convert strings to C-style for SimConnect
	clientDataNamePtr, err := helpers.StringToBytePtr(clientDataName)
	if err != nil {
		return fmt.Errorf("invalid client data name: %v", err)
	}

	hresult, _, _ := SimConnect_MapClientDataNameToID.Call(
		e.handle,                      // hSimConnect
		clientDataNamePtr,             // szClientDataName
		uintptr(uint32(clientDataID)), // ClientDataID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_MapClientDataNameToID failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// CreateClientData requests the creation of a reserved data area for this client.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_CreateClientData.htm
func (e *Engine) CreateClientData(clientDataID int, size int, flags int) error {
	hresult, _, _ := SimConnect_CreateClientData.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(clientDataID)), // ClientDataID
		uintptr(uint32(size)),         // dwSize
		uintptr(uint32(flags)),        // Flags
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_CreateClientData failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// AddToClientDataDefinition adds an offset and a size in bytes, or a type, to a client data definition.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToClientDataDefinition.htm
func (e *Engine) AddToClientDataDefinition(defineID int, offset int, sizeOrType int, epsilon float32, datumID int) error {
	hresult, _, _ := SimConnect_AddToClientDataDefinition.Call(
		e.handle,                    // hSimConnect
		uintptr(uint32(defineID)),   // DefineID
		uintptr(uint32(offset)),     // dwOffset
		uintptr(uint32(sizeOrType)), // dwSizeOrType
		uintptr(epsilon),            // fEpsilon
		uintptr(uint32(datumID)),    // DatumID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToClientDataDefinition failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// ClearClientDataDefinition clears the definition of the specified client data.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearClientDataDefinition.htm
func (e *Engine) ClearClientDataDefinition(defineID int) error {
	hresult, _, _ := SimConnect_ClearClientDataDefinition.Call(
		e.handle,                  // hSimConnect
		uintptr(uint32(defineID)), // DefineID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearClientDataDefinition failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RequestClientData requests that the data in an area created by another client be sent to this client.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestClientData.htm
func (e *Engine) RequestClientData(clientDataID int, requestID int, defineID int, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags int, origin int, interval int, limit int) error {
	hresult, _, _ := SimConnect_RequestClientData.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(clientDataID)), // ClientDataID
		uintptr(uint32(requestID)),    // RequestID
		uintptr(uint32(defineID)),     // DefineID
		uintptr(uint32(period)),       // Period
		uintptr(uint32(flags)),        // Flags
		uintptr(uint32(origin)),       // Origin
		uintptr(uint32(interval)),     // Interval
		uintptr(uint32(limit)),        // Limit
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestClientData failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// SetClientData writes one or more units of data to a client data area.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SetClientData.htm
func (e *Engine) SetClientData(clientDataID int, defineID int, flags int, reserved int, unitSize int, data uintptr) error {
	hresult, _, _ := SimConnect_SetClientData.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(clientDataID)), // ClientDataID
		uintptr(uint32(defineID)),     // DefineID
		uintptr(uint32(flags)),        // Flags
		uintptr(uint32(reserved)),     // dwReserved
		uintptr(uint32(unitSize)),     // cbUnitSize
		data,                          // pDataSet
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetClientData failed: 0x%08X", uint32(hresult))
	}

	return nil
}
