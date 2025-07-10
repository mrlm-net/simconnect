//go:build windows
// +build windows

package client

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ...existing code...

// EnumerateSimObjectsAndLiveries requests the list of spawnable SimObjects and their liveries.
// requestID: Client-defined request ID for tracking the response.
// simObjectType: The type of SimObjects to enumerate (see types.SIMCONNECT_SIMOBJECT_TYPE).
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_EnumerateSimObjectsAndLiveries.htm
func (e *Engine) EnumerateSimObjectsAndLiveries(requestID uint32, simObjectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	hresult, _, _ := SimConnect_EnumerateSimObjectsAndLiveries.Call(
		e.handle,               // hSimConnect
		uintptr(requestID),     // RequestID
		uintptr(simObjectType), // Type
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_EnumerateSimObjectsAndLiveries failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateSimulatedObject creates an AI controlled simulated object (non-aircraft).
// containerTitle: The title of the object (from aircraft.cfg or SimObject).
// initPos: The initial position of the object.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateSimulatedObject.htm
func (e *Engine) AICreateSimulatedObject(containerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, requestID uint32) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	hresult, _, _ := SimConnect_AICreateSimulatedObject.Call(
		e.handle,                          // hSimConnect
		titlePtr,                          // szContainerTitle
		uintptr(unsafe.Pointer(&initPos)), // InitPos
		uintptr(requestID),                // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateSimulatedObject failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateSimulatedObject_EX1 creates an AI controlled simulated object (modular or legacy).
// containerTitle: The title of the object (from aircraft.cfg or SimObject).
// livery: The livery name or folder (can be empty for default livery).
// initPos: The initial position of the object.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateSimulatedObject_EX1.htm
func (e *Engine) AICreateSimulatedObject_EX1(containerTitle, livery string, initPos types.SIMCONNECT_DATA_INITPOSITION, requestID uint32) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	liveryPtr, err := helpers.StringToBytePtr(livery)
	if err != nil {
		return fmt.Errorf("invalid livery: %w", err)
	}
	hresult, _, _ := SimConnect_AICreateSimulatedObject_EX1.Call(
		e.handle,                          // hSimConnect
		titlePtr,                          // szContainerTitle
		liveryPtr,                         // szLivery
		uintptr(unsafe.Pointer(&initPos)), // InitPos
		uintptr(requestID),                // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateSimulatedObject_EX1 failed: 0x%08X", uint32(hresult))
	}
	return nil
}
