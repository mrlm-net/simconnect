//go:build windows
// +build windows

package simconnect

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
func (sc *SimConnect) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	// Convert Go string to null-terminated C string using syscall helper
	szState, err := stringToBytePtr(string(state))
	if err != nil {
		return fmt.Errorf("failed to convert state string to C string: %v", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_RequestSystemState")

	// Request the system state from SimConnect
	// Note: Use e.handle directly, not e.getHandle() which is for receiving handles
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(requestID), // Explicit cast
		szState,
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestSystemState failed for state '%s': 0x%08X", state, uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SubscribeToSystemEvent.htm
func (sc *SimConnect) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	szEventName, err := stringToBytePtr(eventName)
	if err != nil {
		return fmt.Errorf("failed to convert event name to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_SubscribeToSystemEvent")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(eventID),
		szEventName,
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToSystemEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}
