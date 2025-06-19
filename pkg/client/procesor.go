//go:build windows
// +build windows

package client

import (
	"fmt"
	"log"
	"runtime"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) Stream() <-chan ParsedMessage {
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
		case <-e.ctx.Done():
			// Context is done, exit the goroutine
			e.Disconnect() // Exit the goroutine if the context is done
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
				fmt.Println("SimConnect_GetNextDispatch succeeded")

				// Parse the raw message data
				parsedMsg := e.parseMessage(ppData, pcbData)

				// Send the parsed message to the channel (non-blocking)
				select {
				case e.queue <- parsedMsg:
					// Message sent successfully
				default:
					// Channel is full, log warning but don't block
					log.Printf("Warning: Message queue is full, dropping message of type %v", parsedMsg.MessageType)
				}
			}

			//time.Sleep(10 * time.Millisecond)
		}
	}
}

// parseMessage converts raw SimConnect message data into a ParsedMessage
func (e *Engine) parseMessage(ppData uintptr, pcbData uint32) ParsedMessage {
	// Convert raw data to byte slice for easier handling
	rawData := (*[1 << 30]byte)(unsafe.Pointer(ppData))[:pcbData:pcbData]

	// Parse the base header first
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV{})) {
		return ParsedMessage{
			Error:   fmt.Errorf("message too small: %d bytes", pcbData),
			RawData: rawData,
		}
	}

	// Extract the base header
	header := (*types.SIMCONNECT_RECV)(unsafe.Pointer(ppData))

	parsedMsg := ParsedMessage{
		MessageType: header.DwID,
		Header:      header,
		RawData:     rawData,
	}

	// Parse specific message types
	switch header.DwID {
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
		parsedMsg.Data = e.parseSimObjectData(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
		parsedMsg.Data = e.parseSimObjectDataByType(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT:
		parsedMsg.Data = e.parseEvent(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_EX1:
		parsedMsg.Data = e.parseEventEx1(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EXCEPTION:
		parsedMsg.Data = e.parseException(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_OPEN:
		parsedMsg.Data = e.parseOpen(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_QUIT:
		parsedMsg.Data = e.parseQuit(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
		parsedMsg.Data = e.parseAssignedObjectID(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_SYSTEM_STATE:
		parsedMsg.Data = e.parseSystemState(ppData, pcbData)
	default:
		// For unhandled message types, just provide the raw header
		parsedMsg.Data = header
		log.Printf("Unhandled message type: %v", header.DwID)
	}

	return parsedMsg
}

// parseSimObjectData parses SIMCONNECT_RECV_SIMOBJECT_DATA messages
func (e *Engine) parseSimObjectData(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_SIMOBJECT_DATA {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_SIMOBJECT_DATA{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(ppData))
}

// parseSimObjectDataByType parses SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE messages
func (e *Engine) parseSimObjectDataByType(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE)(unsafe.Pointer(ppData))
}

// parseEvent parses SIMCONNECT_RECV_EVENT messages
func (e *Engine) parseEvent(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(ppData))
}

// parseEventEx1 parses SIMCONNECT_RECV_EVENT_EX1 messages
func (e *Engine) parseEventEx1(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_EX1 {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_EX1{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_EX1)(unsafe.Pointer(ppData))
}

// parseException parses SIMCONNECT_RECV_EXCEPTION messages
func (e *Engine) parseException(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EXCEPTION {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EXCEPTION{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(ppData))
}

// parseOpen parses SIMCONNECT_RECV_OPEN messages (connection established)
func (e *Engine) parseOpen(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV {
	return (*types.SIMCONNECT_RECV)(unsafe.Pointer(ppData))
}

// parseQuit parses SIMCONNECT_RECV_QUIT messages (connection closed)
func (e *Engine) parseQuit(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV {
	return (*types.SIMCONNECT_RECV)(unsafe.Pointer(ppData))
}

// parseAssignedObjectID parses SIMCONNECT_RECV_ASSIGNED_OBJECT_ID messages
func (e *Engine) parseAssignedObjectID(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID)(unsafe.Pointer(ppData))
}

// parseSystemState parses SIMCONNECT_RECV_SYSTEM_STATE messages
func (e *Engine) parseSystemState(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_SYSTEM_STATE {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_SYSTEM_STATE{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SYSTEM_STATE)(unsafe.Pointer(ppData))
}
