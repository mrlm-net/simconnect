//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AICreateSimulatedObject.htm
func (sc *SimConnect) AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AICreateSimulatedObject")
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		uintptr(unsafe.Pointer(&initPos)),
		uintptr(RequestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateSimulatedObject failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AIReleaseControl.htm
func (sc *SimConnect) AIReleaseControl(objectID uint32, requestID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_AIReleaseControl")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		uintptr(requestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AIReleaseControl failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AIRemoveObject.htm
func (sc *SimConnect) AIRemoveObject(objectID uint32, requestID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_AIRemoveObject")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		uintptr(requestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AIRemoveObject failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_EnumerateSimObjectsAndLiveries.htm
func (sc *SimConnect) EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	procedure := sc.library.LoadProcedure("SimConnect_EnumerateSimObjectsAndLiveries")
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(requestID),
		uintptr(objectType),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_EnumerateSimObjectsAndLiveries failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateSimulatedObject_EX1.htm
func (sc *SimConnect) AICreateSimulatedObjectEX1(szContainerTitle string, szLivery string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}
	szLiveryPtr, err := stringToBytePtr(szLivery)
	if err != nil {
		return fmt.Errorf("failed to convert livery to byte pointer: %w", err)
	}
	procedure := sc.library.LoadProcedure("SimConnect_AICreateSimulatedObject_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szLiveryPtr,
		uintptr(unsafe.Pointer(&initPos)),
		uintptr(RequestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateSimulatedObject_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}
