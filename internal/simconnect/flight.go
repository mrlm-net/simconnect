//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Flights/SimConnect_FlightLoad.htm
func (sc *SimConnect) FlightLoad(flightFile string) error {
	szFlightFile, err := stringToBytePtr(flightFile)
	if err != nil {
		return fmt.Errorf("failed to convert flight file to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_FlightLoad")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szFlightFile)),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightLoad failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Flights/SimConnect_FlightSave.htm
func (sc *SimConnect) FlightSave(flightFile string, title string, description string) error {
	szFlightFile, err := stringToBytePtr(flightFile)
	if err != nil {
		return fmt.Errorf("failed to convert flight file to byte pointer: %w", err)
	}

	szTitle, err := stringToBytePtr(title)
	if err != nil {
		return fmt.Errorf("failed to convert title to byte pointer: %w", err)
	}

	szDescription, err := stringToBytePtr(description)
	if err != nil {
		return fmt.Errorf("failed to convert description to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_FlightSave")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szFlightFile)),
		uintptr(unsafe.Pointer(szTitle)),
		uintptr(unsafe.Pointer(szDescription)),
		0, // reserved - must be zero
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightSave failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Flights/SimConnect_FlightPlanLoad.htm
func (sc *SimConnect) FlightPlanLoad(flightPlanFile string) error {
	szFlightPlanFile, err := stringToBytePtr(flightPlanFile)
	if err != nil {
		return fmt.Errorf("failed to convert flight plan file to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_FlightPlanLoad")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szFlightPlanFile)),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightPlanLoad failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
