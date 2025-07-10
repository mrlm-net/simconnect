//go:build windows
// +build windows

package client

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

// ExecuteAction executes a simulator action by its name with optional parameters.
// requestID: ID for tracking the request in callbacks.
// actionName: the SimConnect action/event string.
// paramValues: optional parameters as a byte slice (can be nil).
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/General/SimConnect_ExecuteAction.htm
func (e *Engine) ExecuteAction(requestID uint32, actionName string, paramValues []byte) error {
	actionPtr, err := syscall.BytePtrFromString(actionName)
	if err != nil {
		return fmt.Errorf("invalid action name: %w", err)
	}
	var paramPtr unsafe.Pointer
	var paramSize uint32
	if len(paramValues) > 0 {
		paramPtr = unsafe.Pointer(&paramValues[0])
		paramSize = uint32(len(paramValues))
	} else {
		paramPtr = nil
		paramSize = 0
	}
	hresult, _, _ := SimConnect_ExecuteAction.Call(
		e.handle,                           // hSimConnect
		uintptr(requestID),                 // cbRequestID
		uintptr(unsafe.Pointer(actionPtr)), // szActionID
		uintptr(paramSize),                 // cbUnitSize
		uintptr(paramPtr),                  // pParamValues
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ExecuteAction failed for '%s': 0x%08X", actionName, uint32(hresult))
	}

	return nil
}
