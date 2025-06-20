//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// RequestSystemState requests the current system state from SimConnect.
// requestID: Client-defined request ID for tracking the response
// state: System state to request (e.g., "AircraftLoaded", "DialogMode", "FlightLoaded", "FlightPlan", "Sim")
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
func (e *Engine) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	// Check if we have a valid connection handle
	if e.handle == 0 {
		return fmt.Errorf("SimConnect not connected - handle is null")
	}

	// Convert Go string to null-terminated C string using syscall helper
	szState, err := helpers.StringToBytePtr(string(state))
	if err != nil {
		return fmt.Errorf("failed to convert state string to C string: %v", err)
	}

	// Request the system state from SimConnect
	// Note: Use e.handle directly, not e.getHandle() which is for receiving handles
	hresult, _, _ := SimConnect_RequestSystemState.Call(
		e.handle,
		uintptr(uint32(requestID)), // Explicit cast
		szState,
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestSystemState failed for state '%s': 0x%08X", state, uint32(hresult))
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

// SubscribeToSystemEvent subscribes to a system event in SimConnect.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SubscribeToSystemEvent.htm
func (e *Engine) SubscribeToSystemEvent(id int, event string) error {
	// Convert Go string to null-terminated C string using syscall helper
	szSystemEventName, err := helpers.StringToBytePtr(event)
	if err != nil {
		return fmt.Errorf("failed to convert state string to C string: %v", err)
	}

	// Request the system state from SimConnect
	// Note: Use e.handle directly, not e.getHandle() which is for receiving handles
	hresult, _, _ := SimConnect_SubscribeToSystemEvent.Call(
		e.handle,
		uintptr(uint32(id)), // Explicit cast
		szSystemEventName,
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToSystemEvent failed for event '%s': 0x%08X", event, uint32(hresult))
	}

	return nil
}

func (e *Engine) UnsubscribeFromSystemEvent(id int) error {
	hresult, _, _ := SimConnect_UnsubscribeFromSystemEvent.Call(
		e.handle,
		uintptr(uint32(id)), // Explicit cast
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeFromSystemEvent failed: 0x%08X", uint32(hresult))
	}

	return nil
}
