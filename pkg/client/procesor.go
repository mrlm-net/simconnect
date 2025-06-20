//go:build windows
// +build windows

package client

import (
	"log"
	"runtime"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

func (e *Engine) Stream() <-chan ParsedMessage {
	e.wg.Add(1)
	go e.dispatch()

	return e.queue // Return the channel for receiving messages
}

func (e *Engine) dispatch() {
	defer e.wg.Done() // Signal completion when function exits

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

	// Start the message processing loop
	for {
		select {
		case <-e.ctx.Done():
			// Context is done, exit the goroutine
			return
		default:
			// Check if we're closing before attempting to read messages
			e.mu.RLock()
			closing := e.isClosing
			e.mu.RUnlock()

			if closing {
				return
			}

			var ppData uintptr
			var pcbData uint32 // Call SimConnect_GetNextDispatch
			hresult, _, _ := SimConnect_GetNextDispatch.Call(
				uintptr(e.handle),                 // hSimConnect
				uintptr(unsafe.Pointer(&ppData)),  // ppData
				uintptr(unsafe.Pointer(&pcbData)), // pcbData
			)

			if helpers.IsHRESULTSuccess(hresult) {
				// CRITICAL: Copy the data immediately to prevent race conditions
				// SimConnect may reuse the buffer for subsequent messages
				rawDataCopy := make([]byte, pcbData)
				copy(rawDataCopy, (*[1 << 30]byte)(unsafe.Pointer(ppData))[:pcbData:pcbData])

				// Create a new pointer to our copied data
				copiedDataPtr := uintptr(unsafe.Pointer(&rawDataCopy[0]))

				// Parse the message using our safe copy
				parsedMsg := e.parseMessage(copiedDataPtr, pcbData)

				// Send the parsed message to the channel (non-blocking)
				select {
				case e.queue <- parsedMsg:
					// Message sent successfully
				case <-e.ctx.Done():
					// Context cancelled while trying to send
					return
				default:
					// Channel is full, log warning but don't block
					log.Printf("Warning: Message queue is full, dropping message of type %v", parsedMsg.MessageType)
				}
			}

			//time.Sleep(10 * time.Millisecond)
		}
	}
}
