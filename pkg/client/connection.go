//go:build windows
// +build windows

package client

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

func (e *Engine) Open() error {
	szName, _ := helpers.StringToBytePtr(e.name)
	hresult, _, _ := SimConnect_Open.Call(
		e.getHandle(), // phSimConnect
		szName,        // szName
		0,             // hWnd (NULL)
		0,             // UserEventWin32
		0,             // hEventHandle
		uintptr(0),    // ConfigIndex
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		// This needs to be handled properly, maybe with a custom error type.
		return fmt.Errorf("SimConnect_Open failed with HRESULT: 0x%08X", hresult)
	}

	// Verify handle was set or return an error
	if e.handle == 0 {
		return fmt.Errorf("SimConnect_Open succeeded but handle is null")
	}

	return nil
}

func (e *Engine) Close() error {
	hresult, _, _ := SimConnect_Close.Call(e.handle)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_Close failed with HRESULT: 0x%08X", hresult)

	}
	return nil
}

func (e *Engine) getHandle() uintptr {
	return uintptr(unsafe.Pointer(&e.handle))
}
