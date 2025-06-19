//go:build windows
// +build windows

package client

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

func (e *Engine) Connect() error {
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

func (e *Engine) Disconnect() error {
	var err error
	e.once.Do(func() {
		// Set closing flag
		e.mu.Lock()
		e.isClosing = true
		e.mu.Unlock()

		// Cancel context to stop message processing
		if e.cancel != nil {
			e.cancel()
		}

		// Wait for goroutines to finish
		e.wg.Wait()

		// Close the SimConnect connection if it exists
		if e.handle != 0 {
			hresult, _, _ := SimConnect_Close.Call(e.handle)
			if !helpers.IsHRESULTSuccess(hresult) {
				err = fmt.Errorf("SimConnect_Close failed with HRESULT: 0x%08X", hresult)
			} else {
				fmt.Println("SimConnect connection closed successfully")
			}
			e.handle = 0
		}

		// Close the message queue channel
		close(e.queue)
	})
	return err
}

// Shutdown triggers a graceful shutdown of the SimConnect client
func (e *Engine) Shutdown() error {
	return e.Disconnect()
}

func (e *Engine) getHandle() uintptr {
	return uintptr(unsafe.Pointer(&e.handle))
}
