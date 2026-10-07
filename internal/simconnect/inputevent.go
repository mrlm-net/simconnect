//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_EnumerateInputEvents.htm
func (sc *SimConnect) EnumerateInputEvents(requestID uint32) error {
	procedure := sc.proc("SimConnect_EnumerateInputEvents")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(requestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_EnumerateInputEvents failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_GetInputEvent.htm
func (sc *SimConnect) GetInputEvent(requestID uint32, hash uint64) error {
	procedure := sc.proc("SimConnect_GetInputEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(requestID),
		uintptr(hash),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_GetInputEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SetInputEvent.htm
// SimConnect_SetInputEvent(HANDLE, UINT64 Hash, DWORD cbUnitSize, void *Value):
// cbUnitSize is the size of the value, 8 for a double, the string's length
// plus its terminator for a string.
func (sc *SimConnect) SetInputEvent(hash uint64, cbUnitSize uint32, value unsafe.Pointer) error {
	procedure := sc.proc("SimConnect_SetInputEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(hash),
		uintptr(cbUnitSize),
		uintptr(value),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetInputEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SubscribeInputEvent.htm
func (sc *SimConnect) SubscribeInputEvent(hash uint64) error {
	procedure := sc.proc("SimConnect_SubscribeInputEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(hash),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeInputEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_UnsubscribeInputEvent.htm
func (sc *SimConnect) UnsubscribeInputEvent(hash uint64) error {
	procedure := sc.proc("SimConnect_UnsubscribeInputEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),
		uintptr(hash),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeInputEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
