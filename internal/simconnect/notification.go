//go:build windows
// +build windows

package simconnect

import "fmt"

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddClientEventToNotificationGroup.htm
func (sc *SimConnect) AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error {
	procedure := sc.library.LoadProcedure("SimConnect_AddClientEventToNotificationGroup")

	var maskable uint32
	if mask {
		maskable = 1
	} else {
		maskable = 0
	}

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(groupID),
		uintptr(eventID),
		uintptr(maskable),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AISetAircraftFlightPlan failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearNotificationGroup.htm
func (sc *SimConnect) ClearNotificationGroup(groupID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_ClearNotificationGroup")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(groupID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearNotificationGroup failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestNotificationGroup.htm
func (sc *SimConnect) RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestNotificationGroup")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(groupID),
		uintptr(dwReserved),
		uintptr(flags),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestNotificationGroup failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
