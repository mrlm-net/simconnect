//go:build windows
// +build windows

package simconnect

import (
	"errors"
	"fmt"

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
			return nil, 0, errors.New("SimConnect_GetNextDispatch failed: Invalid handle - connection may be closed")
		default:
			return nil, 0, fmt.Errorf("SimConnect_GetNextDispatch failed with HRESULT: 0x%08X", uint32(hresult))
		}
	}

	if ppData == 0 {
		// No message available - this is normal behavior, not an error
		return nil, 0, nil
	}

	return toSIMCONNECT_RECV(ppData), pcbData, nil
}
