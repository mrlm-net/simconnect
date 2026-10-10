//go:build windows
// +build windows

package engine

// MapInputEventToClientEvent maps a key or joystick input (definition:
// "Ctrl+Shift+C", "joystick:0:button:5") in input group groupID to client
// events: downEventID with downValue when pressed, upEventID with upValue
// when released (0xFFFFFFFF: none).
func (e *Engine) MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error {
	return e.api.MapInputEventToClientEvent(groupID, definition, downEventID, downValue, upEventID, upValue, maskable)
}

func (e *Engine) SetInputGroupPriority(groupID uint32, priority uint32) error {
	return e.api.SetInputGroupPriority(groupID, priority)
}

func (e *Engine) SetInputGroupState(groupID uint32, state uint32) error {
	return e.api.SetInputGroupState(groupID, state)
}

func (e *Engine) RemoveInputEvent(groupID uint32, definition string) error {
	return e.api.RemoveInputEvent(groupID, definition)
}

func (e *Engine) ClearInputGroup(groupID uint32) error {
	return e.api.ClearInputGroup(groupID)
}
