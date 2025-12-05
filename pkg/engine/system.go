//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/types"

func (e *Engine) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	return e.api.RequestSystemState(requestID, state)
}

func (e *Engine) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	return e.api.SubscribeToSystemEvent(eventID, eventName)
}

func (e *Engine) UnsubscribeFromSystemEvent(eventID uint32) error {
	return e.api.UnsubscribeFromSystemEvent(eventID)
}

func (e *Engine) SetSystemEventState(eventID uint32, state types.SIMCONNECT_STATE) error {
	return e.api.SetSystemEventState(eventID, state)
}
