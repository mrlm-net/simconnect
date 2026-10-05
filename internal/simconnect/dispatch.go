//go:build windows

package simconnect

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_GetNextDispatch.htm
func (sc *SimConnect) GetNextDispatch() (*types.SIMCONNECT_RECV, uint32, error) {
	var ppData uintptr
	var pcbData uint32

	procedure := sc.library.LoadProcedure("SimConnect_GetNextDispatch")

	hresult, _, _ := procedure.Call(
		sc.getConnection(),        // hSimConnect
		toUnsafePointer(&ppData),  // ppData
		toUnsafePointer(&pcbData), // pcbData
	)

	if !isHRESULTSuccess(hresult) {
		// Check for specific error codes
		switch uint32(hresult) {
		case types.E_FAIL:
			// E_FAIL often just means "no message available right now" - this is normal when polling
			return nil, 0, nil
		case types.E_ACCESSDENIED:
			return nil, 0, errors.New("SimConnect_GetNextDispatch failed: Access denied - check if SimConnect is properly connected")
		case types.E_HANDLE:
			return nil, 0, fmt.Errorf("%w: SimConnect_GetNextDispatch failed: Invalid handle - connection may be closed", ErrConnectionLost)
		case statusPipeDisconnected:
			return nil, 0, fmt.Errorf("%w: SimConnect_GetNextDispatch failed with HRESULT: 0x%08X (pipe disconnected)", ErrConnectionLost, uint32(hresult))
		default:
			return nil, 0, fmt.Errorf("SimConnect_GetNextDispatch failed with HRESULT: 0x%08X", uint32(hresult))
		}
	}

	if ppData == 0 {
		// No message available - this is normal behavior, not an error
		return nil, 0, nil
	}

	// Convert uintptr to SIMCONNECT_RECV pointer.
	// SAFETY: ppData is a pointer value written by the SimConnect DLL, pointing to DLL-managed memory.
	// The conversion is safe because the memory is not Go-managed and remains valid until next dispatch.
	//nolint:govet // ppData is from SimConnect DLL (C memory), not Go heap - conversion is safe
	return (*types.SIMCONNECT_RECV)(unsafe.Pointer(ppData)), pcbData, nil
}

// ErrConnectionLost: the simulator is gone (it quit or restarted): the
// connection's pipe is closed and nothing more will come on it.
var ErrConnectionLost = errors.New("simconnect: connection lost")

// statusPipeDisconnected is STATUS_PIPE_DISCONNECTED (0xC00000B0), what
// SimConnect_GetNextDispatch returns once the simulator has quit.
const statusPipeDisconnected = 0xC00000B0
