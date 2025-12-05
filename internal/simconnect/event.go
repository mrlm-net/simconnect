//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_MapClientEventToSimEvent.htm
func (sc *SimConnect) MapClientEventToSimEvent(eventID uint32, eventName string) error {
	procedure := sc.library.LoadProcedure("SimConnect_MapClientEventToSimEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(eventID),
		uintptr(unsafe.Pointer(syscall.StringBytePtr(eventName))),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_MapClientEventToSimEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RemoveClientEvent.htm
func (sc *SimConnect) RemoveClientEvent(groupID uint32, eventID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_RemoveClientEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(groupID),
		uintptr(eventID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RemoveClientEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_TransmitClientEvent.htm
func (sc *SimConnect) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	procedure := sc.library.LoadProcedure("SimConnect_TransmitClientEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		uintptr(eventID),
		uintptr(data),
		uintptr(groupID),
		uintptr(flags),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_TransmitClientEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_TransmitClientEvent_EX1.htm
func (sc *SimConnect) TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_TransmitClientEvent_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		uintptr(eventID),
		uintptr(groupID),
		uintptr(flags),
		uintptr(data[0]),
		uintptr(data[1]),
		uintptr(data[2]),
		uintptr(data[3]),
		uintptr(data[4]),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_TransmitClientEventEx1 failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_MapClientDataNameToID.htm
func (sc *SimConnect) MapClientDataNameToID(clientDataName string, clientDataID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_MapClientDataNameToID")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(syscall.StringBytePtr(clientDataName))),
		uintptr(clientDataID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_MapClientDataNameToID failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
