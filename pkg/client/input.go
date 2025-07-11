//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

// ClearInputGroup removes all the input events from a specified input group object.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearInputGroup.htm
func (e *Engine) ClearInputGroup(inputGroupID int) error {
	hresult, _, _ := SimConnect_ClearInputGroup.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(inputGroupID)), // InputGroupID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearInputGroup failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RequestReservedKey requests a specific keyboard TAB-key combination applies only to this client.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestReservedKey.htm
func (e *Engine) RequestReservedKey(eventID int, keyChoice1 string, keyChoice2 string, keyChoice3 string) error {
	// Convert strings to C-style for SimConnect
	keyChoice1Ptr, err := helpers.StringToBytePtr(keyChoice1)
	if err != nil {
		return fmt.Errorf("invalid key choice 1: %v", err)
	}

	keyChoice2Ptr, err := helpers.StringToBytePtr(keyChoice2)
	if err != nil {
		return fmt.Errorf("invalid key choice 2: %v", err)
	}

	keyChoice3Ptr, err := helpers.StringToBytePtr(keyChoice3)
	if err != nil {
		return fmt.Errorf("invalid key choice 3: %v", err)
	}

	hresult, _, _ := SimConnect_RequestReservedKey.Call(
		e.handle,                 // hSimConnect
		uintptr(uint32(eventID)), // EventID
		keyChoice1Ptr,            // szKeyChoice1
		keyChoice2Ptr,            // szKeyChoice2
		keyChoice3Ptr,            // szKeyChoice3
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestReservedKey failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// SetInputGroupPriority sets the priority for a specified input group object.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SetInputGroupPriority.htm
func (e *Engine) SetInputGroupPriority(inputGroupID int, priority int) error {
	hresult, _, _ := SimConnect_SetInputGroupPriority.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(inputGroupID)), // InputGroupID
		uintptr(uint32(priority)),     // uPriority
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetInputGroupPriority failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// SetInputGroupState turns requests for input event information from the server on and off.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SetInputGroupState.htm
func (e *Engine) SetInputGroupState(inputGroupID int, state int) error {
	hresult, _, _ := SimConnect_SetInputGroupState.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(inputGroupID)), // InputGroupID
		uintptr(uint32(state)),        // dwState
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetInputGroupState failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RemoveInputEvent removes an input event from a specified input group object.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RemoveInputEvent.htm
func (e *Engine) RemoveInputEvent(inputGroupID int, inputID int) error {
	hresult, _, _ := SimConnect_RemoveInputEvent.Call(
		e.handle,                      // hSimConnect
		uintptr(uint32(inputGroupID)), // InputGroupID
		uintptr(uint32(inputID)),      // InputID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RemoveInputEvent failed: 0x%08X", uint32(hresult))
	}

	return nil
}
