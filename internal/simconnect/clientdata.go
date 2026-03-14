//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_CreateClientData.htm
func (sc *SimConnect) CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error {
	procedure := sc.library.LoadProcedure("SimConnect_CreateClientData")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // HANDLE hSimConnect
		uintptr(clientDataID),
		uintptr(dwSize),
		uintptr(flags),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_CreateClientData failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToClientDataDefinition.htm
func (sc *SimConnect) AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_AddToClientDataDefinition")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // HANDLE hSimConnect
		uintptr(defineID),
		uintptr(dwOffset),
		uintptr(dwSizeOrType),
		uintptr(*(*uint32)(unsafe.Pointer(&epsilon))), // float32 reinterpret to uintptr
		uintptr(datumID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToClientDataDefinition failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestClientData.htm
func (sc *SimConnect) RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestClientData")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // HANDLE hSimConnect
		uintptr(clientDataID),
		uintptr(requestID),
		uintptr(defineID),
		uintptr(period),
		uintptr(flags),
		uintptr(origin),
		uintptr(interval),
		uintptr(limit),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestClientData failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearClientDataDefinition.htm
func (sc *SimConnect) ClearClientDataDefinition(defineID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_ClearClientDataDefinition")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // HANDLE hSimConnect
		uintptr(defineID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearClientDataDefinition failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetClientData.htm
// Note: Flags is a plain uint32 per ADR-B-01 — SimConnect.h does not define a typed enum for SetClientData flags.
func (sc *SimConnect) SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	procedure := sc.library.LoadProcedure("SimConnect_SetClientData")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // HANDLE hSimConnect
		uintptr(clientDataID),
		uintptr(defineID),
		uintptr(flags),
		uintptr(dwReserved),
		uintptr(cbUnitSize),
		uintptr(data),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetClientData failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
