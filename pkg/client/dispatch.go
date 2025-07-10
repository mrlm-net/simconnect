//go:build windows
// +build windows

package client

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) DispatchProc(callback types.DispatchProc, pContext uintptr) error {
	// Wrap the Go callback in a syscall callback with the correct signature
	cb := syscall.NewCallback(func(pData, cbData, context uintptr) uintptr {
		callback(
			(*types.SIMCONNECT_RECV)(unsafe.Pointer(pData)),
			uint32(cbData),
			context,
		)
		return 0
	})

	hresult, _, _ := SimConnect_CallDispatch.Call(
		e.handle, // hSimConnect handle
		cb,
		pContext,
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_CallDispatch failed: 0x%08X", uint32(hresult))
	}

	return nil
}
