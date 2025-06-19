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
