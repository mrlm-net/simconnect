//go:build windows
// +build windows

package client

import (
	"runtime"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

func (e *Engine) Listen() <-chan any {
	go e.dispatch()

	return e.queue // Return the channel for receiving messages
}

func (e *Engine) dispatch() {
	// Ensure the goroutine runs on the same OS thread
	// This is important for Windows API calls that require thread affinity
	// This is a workaround for the fact that SimConnect API calls are not thread-safe
	// and must be called from the same thread that created the SimConnect connection.
	// This is a common pattern in Windows API programming.
	// See: https://docs.microsoft.com/en-us/windows/win32/api/processthreads/nf-processthreads-lockosthread
	// and https://docs.microsoft.com/en-us/windows/win32/api/processthreads/nf-processthreads-unlockosthread
	// Note: This is a blocking call, so it should be used with care.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for {
		select {
		//case <-e.ctx.Done():
		//	return e.ctx.Err() // Graceful shutdown requested
		default:
			var ppData uintptr
			var pcbData uint32
			// Call SimConnect_GetNextDispatch
			hresult, _, _ := SimConnect_GetNextDispatch.Call(
				uintptr(e.handle),                 // hSimConnect
				uintptr(unsafe.Pointer(&ppData)),  // ppData
				uintptr(unsafe.Pointer(&pcbData)), // pcbData
			)

			if helpers.IsHRESULTSuccess(hresult) {
				// Parse and send message to channel (non-blocking)
				//e.handleMessage(ppData, pcbData)
				e.queue <- []any{ppData, pcbData} // Send the raw data to the channel
			}
			//time.Sleep(10 * time.Millisecond)
		}
	}

}
