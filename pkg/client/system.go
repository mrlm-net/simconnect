//go:build windows
// +build windows

package client

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// RequestSystemState requests the current system state from SimConnect.
// requestID: Client-defined request ID for tracking the response
// state: System state to request (e.g., "AircraftLoaded", "DialogMode", "FlightLoaded", "FlightPlan", "Sim")
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
func (e *Engine) RequestSystemState(requestID uint32, state string) error {
	// Convert Go string to null-terminated C string
	cState := append([]byte(state), 0) // Add null terminator

	// Request the system state from SimConnect
	hresult, _, _ := SimConnect_RequestSystemState.Call(
		e.getHandle(),                       // hSimConnect
		uintptr(requestID),                  // RequestID (client-defined)
		uintptr(unsafe.Pointer(&cState[0])), // szState (null-terminated string)
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestSystemState failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RequestSystemStateAircraftLoaded requests the path of the last loaded aircraft file
func (e *Engine) RequestSystemStateAircraftLoaded(requestID uint32) error {
	return e.RequestSystemState(requestID, types.SIMCONNECT_SYSTEM_STATE_AIRCRAFT_LOADED)
}

// RequestSystemStateDialogMode requests whether the simulation is in dialog mode
func (e *Engine) RequestSystemStateDialogMode(requestID uint32) error {
	return e.RequestSystemState(requestID, types.SIMCONNECT_SYSTEM_STATE_DIALOG_MODE)
}

// RequestSystemStateFlightLoaded requests the path of the last loaded flight
func (e *Engine) RequestSystemStateFlightLoaded(requestID uint32) error {
	return e.RequestSystemState(requestID, types.SIMCONNECT_SYSTEM_STATE_FLIGHT_LOADED)
}

// RequestSystemStateFlightPlan requests the path of the active flight plan
func (e *Engine) RequestSystemStateFlightPlan(requestID uint32) error {
	return e.RequestSystemState(requestID, types.SIMCONNECT_SYSTEM_STATE_FLIGHT_PLAN)
}

// RequestSystemStateSim requests the current simulation state
func (e *Engine) RequestSystemStateSim(requestID uint32) error {
	return e.RequestSystemState(requestID, types.SIMCONNECT_SYSTEM_STATE_SIM)
}
