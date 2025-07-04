//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

// SimConnect_RequestResponseTimes requests the response times from the SimConnect server.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Debug/SimConnect_RequestResponseTimes.htm
func (e *Engine) SimConnect_RequestResponseTimes(count int) error {
	hresult, _, _ := SimConnect_RequestResponseTimes.Call(
		e.handle,       // hSimConnect (use handle directly, not getHandle())
		uintptr(count), // Count of response times data values written to respopnse
		0,              // Elapsed seconds
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestResponseTimes failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) SimConnect_GetLastSentPacketID(id uintptr) error {
	hresult, _, _ := SimConnect_GetLastSentPacketID.Call(
		e.handle, // hSimConnect (use handle directly, not getHandle())
		id,       // Pointer to the packet ID to be filled
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_GetLastSentPacketID failed: 0x%08X", uint32(hresult))
	}

	return nil
}
