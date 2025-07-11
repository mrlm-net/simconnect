//go:build windows
// +build windows

package client

import "github.com/mrlm-net/simconnect/pkg/types"

// RawMessage represents the raw message data from SimConnect
type RawMessage struct {
	PPData  uintptr // Pointer to the data
	PCBData uint32  // Size of the data
}

// ParsedMessage represents a parsed SimConnect message with its type and data
type ParsedMessage struct {
	MessageType types.SIMCONNECT_RECV_ID // The type of message received
	Header      *types.SIMCONNECT_RECV   // Base header information
	Data        interface{}              // Parsed message data (specific type based on MessageType)
	RawData     []byte                   // Raw byte data for custom parsing if needed
	Error       error                    // Any parsing error that occurred
}

// Helper methods for easy message type checking

// IsSimObjectData checks if the message is sim object data
func (msg *ParsedMessage) IsSimObjectData() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA
}

// IsEvent checks if the message is an event
func (msg *ParsedMessage) IsEvent() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_EVENT
}

// IsException checks if the message is an exception
func (msg *ParsedMessage) IsException() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_EXCEPTION
}

// IsOpen checks if the message is a connection open confirmation
func (msg *ParsedMessage) IsOpen() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_OPEN
}

// IsQuit checks if the message is a connection quit notification
func (msg *ParsedMessage) IsQuit() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_QUIT
}

// GetSimObjectData safely casts the data to SIMCONNECT_RECV_SIMOBJECT_DATA
func (msg *ParsedMessage) GetSimObjectData() (*types.SIMCONNECT_RECV_SIMOBJECT_DATA, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
		return data, true
	}
	return nil, false
}

// GetEvent safely casts the data to SIMCONNECT_RECV_EVENT
func (msg *ParsedMessage) GetEvent() (*types.SIMCONNECT_RECV_EVENT, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_EVENT); ok {
		return data, true
	}
	return nil, false
}

// GetException safely casts the data to SIMCONNECT_RECV_EXCEPTION
func (msg *ParsedMessage) GetException() (*types.SIMCONNECT_RECV_EXCEPTION, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_EXCEPTION); ok {
		return data, true
	}
	return nil, false
}

// GetOpen safely casts the data to SIMCONNECT_RECV_OPEN
func (msg *ParsedMessage) GetOpen() (*types.SIMCONNECT_RECV_OPEN, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_OPEN); ok {
		return data, true
	}
	return nil, false
}

// GetQuit safely casts the data to SIMCONNECT_RECV_QUIT
func (msg *ParsedMessage) GetQuit() (*types.SIMCONNECT_RECV_QUIT, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_QUIT); ok {
		return data, true
	}
	return nil, false
}

// Additional helper methods for new message types

// IsEventEX1 checks if the message is an extended event with multiple parameters
func (msg *ParsedMessage) IsEventEX1() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_EVENT_EX1
}

// IsClientData checks if the message is client data
func (msg *ParsedMessage) IsClientData() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_CLIENT_DATA
}

// IsFacilityData checks if the message is facility data
func (msg *ParsedMessage) IsFacilityData() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_FACILITY_DATA
}

// IsAirportList checks if the message is an airport list
func (msg *ParsedMessage) IsAirportList() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_AIRPORT_LIST
}

// IsInputEvent checks if the message is related to input events
func (msg *ParsedMessage) IsInputEvent() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS ||
		msg.MessageType == types.SIMCONNECT_RECV_ID_GET_INPUT_EVENT ||
		msg.MessageType == types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT ||
		msg.MessageType == types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENT_PARAMS
}

// IsMultiplayerEvent checks if the message is a multiplayer event
func (msg *ParsedMessage) IsMultiplayerEvent() bool {
	return msg.MessageType == types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SERVER_STARTED ||
		msg.MessageType == types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_CLIENT_STARTED ||
		msg.MessageType == types.SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SESSION_ENDED
}

// GetEventEX1 safely casts the data to SIMCONNECT_RECV_EVENT_EX1
func (msg *ParsedMessage) GetEventEX1() (*types.SIMCONNECT_RECV_EVENT_EX1, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_EVENT_EX1); ok {
		return data, true
	}
	return nil, false
}

// GetClientData safely casts the data to SIMCONNECT_RECV_CLIENT_DATA
func (msg *ParsedMessage) GetClientData() (*types.SIMCONNECT_RECV_CLIENT_DATA, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_CLIENT_DATA); ok {
		return data, true
	}
	return nil, false
}

// GetFacilityData safely casts the data to SIMCONNECT_RECV_FACILITY_DATA
func (msg *ParsedMessage) GetFacilityData() (*types.SIMCONNECT_RECV_FACILITY_DATA, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_FACILITY_DATA); ok {
		return data, true
	}
	return nil, false
}

// GetAirportList safely casts the data to SIMCONNECT_RECV_AIRPORT_LIST
func (msg *ParsedMessage) GetAirportList() (*types.SIMCONNECT_RECV_AIRPORT_LIST, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_AIRPORT_LIST); ok {
		return data, true
	}
	return nil, false
}

// GetVORList safely casts the data to SIMCONNECT_RECV_VOR_LIST
func (msg *ParsedMessage) GetVORList() (*types.SIMCONNECT_RECV_VOR_LIST, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_VOR_LIST); ok {
		return data, true
	}
	return nil, false
}

// GetNDBList safely casts the data to SIMCONNECT_RECV_NDB_LIST
func (msg *ParsedMessage) GetNDBList() (*types.SIMCONNECT_RECV_NDB_LIST, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_NDB_LIST); ok {
		return data, true
	}
	return nil, false
}

// GetWaypointList safely casts the data to SIMCONNECT_RECV_WAYPOINT_LIST
func (msg *ParsedMessage) GetWaypointList() (*types.SIMCONNECT_RECV_WAYPOINT_LIST, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_WAYPOINT_LIST); ok {
		return data, true
	}
	return nil, false
}

// GetControllersList safely casts the data to SIMCONNECT_RECV_CONTROLLERS_LIST
func (msg *ParsedMessage) GetControllersList() (*types.SIMCONNECT_RECV_CONTROLLERS_LIST, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_CONTROLLERS_LIST); ok {
		return data, true
	}
	return nil, false
}

// GetEnumerateInputEvents safely casts the data to SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS
func (msg *ParsedMessage) GetEnumerateInputEvents() (*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS); ok {
		return data, true
	}
	return nil, false
}

// GetReservedKey safely casts the data to SIMCONNECT_RECV_RESERVED_KEY
func (msg *ParsedMessage) GetReservedKey() (*types.SIMCONNECT_RECV_RESERVED_KEY, bool) {
	if data, ok := msg.Data.(*types.SIMCONNECT_RECV_RESERVED_KEY); ok {
		return data, true
	}
	return nil, false
}
