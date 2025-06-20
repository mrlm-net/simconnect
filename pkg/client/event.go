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
