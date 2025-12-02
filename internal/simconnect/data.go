//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObject.htm
func (sc *SimConnect) RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestDataOnSimObject")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(requestID),
		uintptr(definitionID),
		uintptr(objectID),
		uintptr(period),
		uintptr(flags),
		uintptr(origin),
		uintptr(interval),
		uintptr(limit),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestDataOnSimObject failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObjectType.htm
func (sc *SimConnect) RequestDataOnSimObjectType(requestID uint32, definitionID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestDataOnSimObjectType")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(requestID),
		uintptr(definitionID),
		uintptr(objectType),
		uintptr(period),
		uintptr(flags),
		uintptr(origin),
		uintptr(interval),
		uintptr(limit),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestDataOnSimObjectType failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToDataDefinition.htm
func (sc *SimConnect) AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error {
	szDatumName, err := stringToBytePtr(datumName)
	if err != nil {
		return fmt.Errorf("failed to convert datum name to byte pointer: %w", err)
	}

	szUnitsName, err := stringToBytePtr(unitsName)
	if err != nil {
		return fmt.Errorf("failed to convert units name to byte pointer: %w", err)
	}
	procedure := sc.library.LoadProcedure("SimConnect_AddToDataDefinition")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		szDatumName,
		szUnitsName,
		uintptr(datumType),
		uintptr(*(*uint32)(unsafe.Pointer(&epsilon))), // float32 to uintptr conversion
		uintptr(datumID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToDataDefinition failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearDataDefinition.htm
func (sc *SimConnect) ClearDataDefinition(definitionID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_ClearDataDefinition")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearDataDefinition failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SetDataOnSimObject.htm
func (sc *SimConnect) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, data unsafe.Pointer, dataSize uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_SetDataOnSimObject")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		uintptr(objectID),
		uintptr(flags),
		uintptr(dataSize),
		uintptr(data),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetDataOnSimObject failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
