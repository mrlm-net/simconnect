//go:build windows
// +build windows

package client

// TODO consider https://blog.gopheracademy.com/advent-2019/safe-use-of-unsafe-pointer/

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// parseMessage converts raw SimConnect message data into a ParsedMessage
// Note: ppData should point to a stable copy of the data, not SimConnect's internal buffer
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
	case types.SIMCONNECT_RECV_ID_EVENT_FILENAME:
		parsedMsg.Data = e.parseEventFilename(ppData, pcbData)
	default:
		// For unhandled message types, just provide the raw header
		parsedMsg.Data = header
		//log.Printf("Unhandled message type: %v", header.DwID)
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
func (e *Engine) parseOpen(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_OPEN {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_OPEN{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_OPEN)(unsafe.Pointer(ppData))
}

// parseQuit parses SIMCONNECT_RECV_QUIT messages (connection closed)
func (e *Engine) parseQuit(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_QUIT {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_QUIT{})) {
		return nil
	}

	return (*types.SIMCONNECT_RECV_QUIT)(unsafe.Pointer(ppData))
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

// parseEventFilename parses SIMCONNECT_RECV_EVENT_FILENAME messages
func (e *Engine) parseEventFilename(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_FILENAME {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_FILENAME{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FILENAME)(unsafe.Pointer(ppData))
}
