//go:build windows
// +build windows

package dispatch

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ExtractEventData parses a SIMCONNECT_RECV_ID_EVENT message and returns event ID and data.
// Returns (0, 0) if the message is not an event message.
func ExtractEventData(msg engine.Message) (eventID uint32, eventData uint32) {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
		return 0, 0
	}
	eventMsg := msg.AsEvent()
	if eventMsg == nil {
		return 0, 0
	}
	return uint32(eventMsg.UEventID), uint32(eventMsg.DwData)
}

// ExtractFilenameEventData parses a SIMCONNECT_RECV_ID_EVENT_FILENAME message.
// Returns (0, "") if the message is not a filename event message.
func ExtractFilenameEventData(msg engine.Message) (eventID uint32, filename string) {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT_FILENAME {
		return 0, ""
	}
	fnameMsg := msg.AsEventFilename()
	if fnameMsg == nil {
		return 0, ""
	}
	return uint32(fnameMsg.UEventID), engine.BytesToString(fnameMsg.SzFileName[:])
}

// ExtractObjectEventData parses a SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE message.
// Returns (0, 0, 0) if the message is not an object add/remove event message.
func ExtractObjectEventData(msg engine.Message) (eventID uint32, objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE {
		return 0, 0, 0
	}
	objMsg := msg.AsEventObjectAddRemove()
	if objMsg == nil {
		return 0, 0, 0
	}
	return uint32(objMsg.UEventID), uint32(objMsg.DwData), objMsg.EObjType
}

// ValidateSimObjectDataMessage checks if a SIMOBJECT_DATA message matches the expected request and definition IDs.
// Returns the message data if valid, nil otherwise.
func ValidateSimObjectDataMessage(
	msg engine.Message,
	expectedRequestID uint32,
	expectedDefinitionID uint32,
) *types.SIMCONNECT_RECV_SIMOBJECT_DATA {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return nil
	}
	simObjMsg := msg.AsSimObjectData()
	if simObjMsg == nil {
		return nil
	}
	if uint32(simObjMsg.DwRequestID) != expectedRequestID || uint32(simObjMsg.DwDefineID) != expectedDefinitionID {
		return nil
	}
	return simObjMsg
}
