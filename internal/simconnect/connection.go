//go:build windows

package simconnect

import (
	"fmt"
	"unsafe"
)

func (sc *SimConnect) Connect() error {
	szName, err := stringToBytePtr(sc.name)

	if err != nil {
		return fmt.Errorf("failed to convert client name to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_Open")

	// The DLL writes the handle into a local, stored under the lock after:
	// writing it straight into sc.connection raced with readers.
	var handle uintptr
	hresult, _, _ := procedure.Call(
		uintptr(unsafe.Pointer(&handle)), // phSimConnect - pointer to connection handle
		uintptr(unsafe.Pointer(szName)),  // szName
		0,                                // hWnd (NULL)
		0,                                // UserEventWin32
		0,                                // hEventHandle
		uintptr(0),                       // ConfigIndex
	)

	if !isHRESULTSuccess(hresult) {
		// This needs to be handled properly, maybe with a custom error type.
		return fmt.Errorf("SimConnect_Open failed with HRESULT: 0x%08X", hresult)
	}

	// Verify handle was set or return an error
	if handle == 0 {
		return fmt.Errorf("SimConnect_Open succeeded but handle is null")
	}

	sc.sync.Lock()
	sc.connection = handle
	sc.sync.Unlock()

	return nil
}

// Disconnect closes the connection. It holds the lock across
// SimConnect_Close, so it waits for calls in flight and no call starts on
// the handle being closed.
func (sc *SimConnect) Disconnect() error {
	procedure := sc.library.LoadProcedure("SimConnect_Close")

	sc.sync.Lock()
	defer sc.sync.Unlock()

	if sc.connection != 0 {
		hresult, _, _ := procedure.Call(
			sc.connection, // hSimConnect
		)

		if !isHRESULTSuccess(hresult) {
			return fmt.Errorf("SimConnect_Close failed with HRESULT: 0x%08X", hresult)
		}

		sc.connection = 0
	}

	return nil
}
