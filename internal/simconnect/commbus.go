//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// SubscribeToCommBusEvent subscribes to CommBus event eventName: each call
// of it (from JavaScript, WebAssembly or another client) arrives as a
// SIMCONNECT_RECV_COMM_BUS message with eventID. MSFS 2024 only.
func (sc *SimConnect) SubscribeToCommBusEvent(eventID uint32, eventName string) error {
	name, err := stringToBytePtr(eventName)
	if err != nil {
		return err
	}
	hresult, _, _ := sc.proc("SimConnect_SubscribeToCommBusEvent").Call(sc.getConnection(), uintptr(eventID), uintptr(unsafe.Pointer(name)))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToCommBusEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// UnsubscribeToCommBusEvent ends the subscription with eventID. MSFS 2024 only.
func (sc *SimConnect) UnsubscribeToCommBusEvent(eventID uint32) error {
	hresult, _, _ := sc.proc("SimConnect_UnsubscribeToCommBusEvent").Call(sc.getConnection(), uintptr(eventID))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeToCommBusEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// CallCommBusEvent calls CommBus event eventName with data (a string,
// usually JSON) for those in broadcastTo. MSFS 2024 only.
func (sc *SimConnect) CallCommBusEvent(eventName string, broadcastTo types.SIMCONNECT_COMM_BUS_BROADCAST_TO, data string) error {
	name, err := stringToBytePtr(eventName)
	if err != nil {
		return err
	}
	buf := append([]byte(data), 0) // NUL-terminated, its size with the NUL
	hresult, _, _ := sc.proc("SimConnect_CallCommBusEvent").Call(sc.getConnection(), uintptr(unsafe.Pointer(name)), uintptr(broadcastTo), uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_CallCommBusEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}
