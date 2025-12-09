//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/types"

func (e *Engine) MapClientEventToSimEvent(eventID uint32, eventName string) error {
	return e.api.MapClientEventToSimEvent(eventID, eventName)
}

func (e *Engine) RemoveClientEvent(groupID uint32, eventID uint32) error {
	return e.api.RemoveClientEvent(groupID, eventID)
}

func (e *Engine) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	return e.api.TransmitClientEvent(objectID, eventID, data, groupID, flags)
}

func (e *Engine) TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error {
	return e.api.TransmitClientEventEx1(objectID, eventID, groupID, flags, data)
}

func (e *Engine) MapClientDataNameToID(clientDataName string, clientDataID uint32) error {
	return e.api.MapClientDataNameToID(clientDataName, clientDataID)
}
