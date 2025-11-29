//go:build windows
// +build windows

package simconnect

import (
	"fmt"
)

func (sc *SimConnect) Connect() error {
	szName, err := stringToBytePtr(sc.name)

	if err != nil {
		return fmt.Errorf("failed to convert client name to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_Open")

	hresult, _, _ := procedure.Call(
		sc.getConnectionPtr(), // phSimConnect - pointer to connection handle
		szName,                // szName
		0,                     // hWnd (NULL)
		0,                     // UserEventWin32
		0,                     // hEventHandle
		uintptr(0),            // ConfigIndex
	)

	if !isHRESULTSuccess(hresult) {
		// This needs to be handled properly, maybe with a custom error type.
		return fmt.Errorf("SimConnect_Open failed with HRESULT: 0x%08X", hresult)
	}

	// Verify handle was set or return an error
	handle := sc.getConnection()

	if handle == 0 {
		return fmt.Errorf("SimConnect_Open succeeded but handle is null")
	}

	return nil
}

func (sc *SimConnect) Disconnect() error {
	procedure := sc.library.LoadProcedure("SimConnect_Close")

	if sc.connection != 0 {
		hresult, _, _ := procedure.Call(
			sc.getConnection(), // hSimConnect
		)

		if !isHRESULTSuccess(hresult) {
			return fmt.Errorf("SimConnect_Close failed with HRESULT: 0x%08X", hresult)
		}
		sc.connection = 0
	}

	return nil
}
