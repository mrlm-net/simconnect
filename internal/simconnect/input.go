//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"
)

// unused is SIMCONNECT_UNUSED: no event (DWORD_MAX).
const unused = ^uint32(0)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_MapInputEventToClientEvent.htm
//
// upEventID 0xFFFFFFFF (SIMCONNECT_UNUSED) sends nothing on release.
func (sc *SimConnect) MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error {
	procedure := sc.proc("SimConnect_MapInputEventToClientEvent")
	defPtr, err := stringToBytePtr(definition)
	if err != nil {
		return fmt.Errorf("SimConnect_MapInputEventToClientEvent failed to convert definition: %w", err)
	}
	mask := uintptr(0)
	if maskable {
		mask = 1
	}
	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(groupID),
		uintptr(unsafe.Pointer(defPtr)),
		uintptr(downEventID),
		uintptr(downValue),
		uintptr(upEventID),
		uintptr(upValue),
		mask,
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_MapInputEventToClientEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetInputGroupPriority.htm
func (sc *SimConnect) SetInputGroupPriority(groupID uint32, priority uint32) error {
	return sc.call2("SimConnect_SetInputGroupPriority", groupID, priority)
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetInputGroupState.htm
func (sc *SimConnect) SetInputGroupState(groupID uint32, state uint32) error {
	return sc.call2("SimConnect_SetInputGroupState", groupID, state)
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RemoveInputEvent.htm
func (sc *SimConnect) RemoveInputEvent(groupID uint32, definition string) error {
	procedure := sc.proc("SimConnect_RemoveInputEvent")
	defPtr, err := stringToBytePtr(definition)
	if err != nil {
		return fmt.Errorf("SimConnect_RemoveInputEvent failed to convert definition: %w", err)
	}
	hresult, _, _ := procedure.Call(sc.getConnection(), uintptr(groupID), uintptr(unsafe.Pointer(defPtr)))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RemoveInputEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearInputGroup.htm
func (sc *SimConnect) ClearInputGroup(groupID uint32) error {
	procedure := sc.proc("SimConnect_ClearInputGroup")
	hresult, _, _ := procedure.Call(sc.getConnection(), uintptr(groupID))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearInputGroup failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// call2 calls a SimConnect function taking the handle and two DWORDs.
func (sc *SimConnect) call2(name string, a, b uint32) error {
	hresult, _, _ := sc.proc(name).Call(sc.getConnection(), uintptr(a), uintptr(b))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("%s failed with HRESULT: 0x%08X", name, uint32(hresult))
	}
	return nil
}
