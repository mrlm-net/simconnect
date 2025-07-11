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
	case types.SIMCONNECT_RECV_ID_EVENT_FRAME:
		parsedMsg.Data = e.parseEventFrame(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE:
		parsedMsg.Data = e.parseEventObjectAddRemove(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_CLIENT_DATA:
		parsedMsg.Data = e.parseClientData(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_RESERVED_KEY:
		parsedMsg.Data = e.parseReservedKey(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
		parsedMsg.Data = e.parseFacilityData(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
		parsedMsg.Data = e.parseFacilityDataEnd(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_FACILITY_MINIMAL_LIST:
		parsedMsg.Data = e.parseFacilityMinimalList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_AIRPORT_LIST:
		parsedMsg.Data = e.parseAirportList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_VOR_LIST:
		parsedMsg.Data = e.parseVORList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_NDB_LIST:
		parsedMsg.Data = e.parseNDBList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_WAYPOINT_LIST:
		parsedMsg.Data = e.parseWaypointList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_JETWAY_DATA:
		parsedMsg.Data = e.parseJetwayData(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_CONTROLLERS_LIST:
		parsedMsg.Data = e.parseControllersList(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS:
		parsedMsg.Data = e.parseEnumerateInputEvents(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENT_PARAMS:
		parsedMsg.Data = e.parseEnumerateInputEventParams(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT:
		parsedMsg.Data = e.parseSubscribeInputEvent(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SERVER_STARTED:
		parsedMsg.Data = e.parseEventMultiplayerServerStarted(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_CLIENT_STARTED:
		parsedMsg.Data = e.parseEventMultiplayerClientStarted(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SESSION_ENDED:
		parsedMsg.Data = e.parseEventMultiplayerSessionEnded(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_RACE_END:
		parsedMsg.Data = e.parseEventRaceEnd(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_EVENT_RACE_LAP:
		parsedMsg.Data = e.parseEventRaceLap(ppData, pcbData)
	case types.SIMCONNECT_RECV_ID_WEATHER_OBSERVATION:
		// Legacy message type - handle as raw header
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_CLOUD_STATE:
		// Legacy message type - handle as raw header
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_EVENT_WEATHER_MODE:
		// Legacy message type - handle as raw header
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_CUSTOM_ACTION:
		// Custom action callback - handle as raw header
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_ACTION_CALLBACK:
		// Action callback - handle as raw header
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_GET_INPUT_EVENT:
		// Handle as raw header for now - could be extended if structure is defined
		parsedMsg.Data = header
	case types.SIMCONNECT_RECV_ID_PICK:
		// Pick event - handle as raw header
		parsedMsg.Data = header
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

// parseEventFrame parses SIMCONNECT_RECV_EVENT_FRAME messages
func (e *Engine) parseEventFrame(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_FRAME {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_FRAME{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FRAME)(unsafe.Pointer(ppData))
}

// parseEventObjectAddRemove parses SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE messages
func (e *Engine) parseEventObjectAddRemove(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE)(unsafe.Pointer(ppData))
}

// parseClientData parses SIMCONNECT_RECV_CLIENT_DATA messages
func (e *Engine) parseClientData(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_CLIENT_DATA {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_CLIENT_DATA{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_CLIENT_DATA)(unsafe.Pointer(ppData))
}

// parseReservedKey parses SIMCONNECT_RECV_RESERVED_KEY messages
func (e *Engine) parseReservedKey(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_RESERVED_KEY {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_RESERVED_KEY{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_RESERVED_KEY)(unsafe.Pointer(ppData))
}

// parseFacilityData parses SIMCONNECT_RECV_FACILITY_DATA messages
func (e *Engine) parseFacilityData(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_FACILITY_DATA {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITY_DATA{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_DATA)(unsafe.Pointer(ppData))
}

// parseFacilityDataEnd parses SIMCONNECT_RECV_FACILITY_DATA_END messages
func (e *Engine) parseFacilityDataEnd(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_FACILITY_DATA_END {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITY_DATA_END{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_DATA_END)(unsafe.Pointer(ppData))
}

// parseFacilityMinimalList parses SIMCONNECT_RECV_FACILITY_MINIMAL_LIST messages
func (e *Engine) parseFacilityMinimalList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_FACILITY_MINIMAL_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITY_MINIMAL_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_MINIMAL_LIST)(unsafe.Pointer(ppData))
}

// parseAirportList parses SIMCONNECT_RECV_AIRPORT_LIST messages
func (e *Engine) parseAirportList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_AIRPORT_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_AIRPORT_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_AIRPORT_LIST)(unsafe.Pointer(ppData))
}

// parseVORList parses SIMCONNECT_RECV_VOR_LIST messages
func (e *Engine) parseVORList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_VOR_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_VOR_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_VOR_LIST)(unsafe.Pointer(ppData))
}

// parseNDBList parses SIMCONNECT_RECV_NDB_LIST messages
func (e *Engine) parseNDBList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_NDB_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_NDB_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_NDB_LIST)(unsafe.Pointer(ppData))
}

// parseWaypointList parses SIMCONNECT_RECV_WAYPOINT_LIST messages
func (e *Engine) parseWaypointList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_WAYPOINT_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_WAYPOINT_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_WAYPOINT_LIST)(unsafe.Pointer(ppData))
}

// parseJetwayData parses SIMCONNECT_RECV_JETWAY_DATA messages
func (e *Engine) parseJetwayData(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_JETWAY_DATA {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_JETWAY_DATA{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_JETWAY_DATA)(unsafe.Pointer(ppData))
}

// parseControllersList parses SIMCONNECT_RECV_CONTROLLERS_LIST messages
func (e *Engine) parseControllersList(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_CONTROLLERS_LIST {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_CONTROLLERS_LIST{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_CONTROLLERS_LIST)(unsafe.Pointer(ppData))
}

// parseEnumerateInputEvents parses SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS messages
func (e *Engine) parseEnumerateInputEvents(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS)(unsafe.Pointer(ppData))
}

// parseEnumerateInputEventParams parses SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS messages
func (e *Engine) parseEnumerateInputEventParams(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS)(unsafe.Pointer(ppData))
}

// parseSubscribeInputEvent parses SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT messages
func (e *Engine) parseSubscribeInputEvent(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT)(unsafe.Pointer(ppData))
}

// parseEventMultiplayerServerStarted parses SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED messages
func (e *Engine) parseEventMultiplayerServerStarted(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED)(unsafe.Pointer(ppData))
}

// parseEventMultiplayerClientStarted parses SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED messages
func (e *Engine) parseEventMultiplayerClientStarted(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED)(unsafe.Pointer(ppData))
}

// parseEventMultiplayerSessionEnded parses SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED messages
func (e *Engine) parseEventMultiplayerSessionEnded(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED)(unsafe.Pointer(ppData))
}

// parseEventRaceEnd parses SIMCONNECT_RECV_EVENT_RACE_END messages
func (e *Engine) parseEventRaceEnd(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_RACE_END {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_RACE_END{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_RACE_END)(unsafe.Pointer(ppData))
}

// parseEventRaceLap parses SIMCONNECT_RECV_EVENT_RACE_LAP messages
func (e *Engine) parseEventRaceLap(ppData uintptr, pcbData uint32) *types.SIMCONNECT_RECV_EVENT_RACE_LAP {
	if pcbData < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_EVENT_RACE_LAP{})) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_RACE_LAP)(unsafe.Pointer(ppData))
}
