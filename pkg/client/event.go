//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

// TransmitClientEvent transmits a client event to the SimConnect server.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_TransmitClientEvent.htm
func (e *Engine) TransmitClientEvent(object int, event int, data int, group int) error {
	hresult, _, _ := SimConnect_TransmitClientEvent.Call(
		e.handle,
		uintptr(uint32(object)),
		uintptr(uint32(event)),
		uintptr(uint32(data)),
		uintptr(uint32(group)),
		uintptr(0), // Flags, set to 0 for default behavior
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_TransmitClientEvent failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// MapClientEventToSimEvent maps a client event to a SimConnect event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_MapClientEventToSimEvent.htm
func (e *Engine) MapClientEventToSimEvent(id int, event string) error {
	// Convert Go string to null-terminated C string using syscall helper
	szSystemEventName, err := helpers.StringToBytePtr(event)
	if err != nil {
		return fmt.Errorf("failed to convert state string to C string: %v", err)
	}
	hresult, _, _ := SimConnect_MapClientEventToSimEvent.Call(
		e.handle,
		uintptr(uint32(id)),
		szSystemEventName,
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_MapClientEventToSimEvent failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// AddClientEventToNotificationGroup adds a client event to a notification group.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddClientEventToNotificationGroup.htm
func (e *Engine) AddClientEventToNotificationGroup(group int, event int) error {
	hresult, _, _ := SimConnect_AddClientEventToNotificationGroup.Call(
		e.handle,
		uintptr(uint32(group)),
		uintptr(uint32(event)),
		uintptr(0),
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddClientEventToNotificationGroup failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// SetNotificationGroupPriority sets the priority of a notification group.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_SetNotificationGroupPriority.htm
func (e *Engine) SetNotificationGroupPriority(group int, priority int) error {
	hresult, _, _ := SimConnect_SetNotificationGroupPriority.Call(
		e.handle,
		uintptr(uint32(group)),
		uintptr(uint32(priority)),
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetNotificationGroupPriority failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RemoveClientEvent removes a client defined event from a notification group.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RemoveClientEvent.htm
func (e *Engine) RemoveClientEvent(group int, event int) error {
	hresult, _, _ := SimConnect_RemoveClientEvent.Call(
		e.handle,
		uintptr(uint32(group)),
		uintptr(uint32(event)),
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RemoveClientEvent failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// TransmitClientEvent_EX1 transmits a client event with up to five event parameters to the SimConnect server.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_TransmitClientEvent_EX1.htm
func (e *Engine) TransmitClientEvent_EX1(object int, event int, group int, flags int, param0, param1, param2, param3, param4 int) error {
	hresult, _, _ := SimConnect_TransmitClientEvent_EX1.Call(
		e.handle,
		uintptr(uint32(object)),
		uintptr(uint32(event)),
		uintptr(uint32(group)),
		uintptr(uint32(flags)),
		uintptr(uint32(param0)),
		uintptr(uint32(param1)),
		uintptr(uint32(param2)),
		uintptr(uint32(param3)),
		uintptr(uint32(param4)),
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_TransmitClientEvent_EX1 failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// ClearNotificationGroup removes all the client defined events from a notification group.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_ClearNotificationGroup.htm
func (e *Engine) ClearNotificationGroup(group int) error {
	hresult, _, _ := SimConnect_ClearNotificationGroup.Call(
		e.handle,
		uintptr(uint32(group)),
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearNotificationGroup failed: 0x%08X", uint32(hresult))
	}

	return nil
}

// RequestNotificationGroup requests events from a notification group when the simulation is in Dialog Mode.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestNotificationGroup.htm
func (e *Engine) RequestNotificationGroup(group int, reserved int) error {
	hresult, _, _ := SimConnect_RequestNotificationGroup.Call(
		e.handle,
		uintptr(uint32(group)),
		uintptr(uint32(reserved)), // Reserved, should be 0
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestNotificationGroup failed: 0x%08X", uint32(hresult))
	}

	return nil
}
