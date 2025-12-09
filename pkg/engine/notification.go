//go:build windows
// +build windows

package engine

func (e *Engine) AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error {
	return e.api.AddClientEventToNotificationGroup(groupID, eventID, mask)
}

func (e *Engine) ClearNotificationGroup(groupID uint32) error {
	return e.api.ClearNotificationGroup(groupID)
}

func (e *Engine) RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error {
	return e.api.RequestNotificationGroup(groupID, dwReserved, flags)
}

func (e *Engine) SetNotificationGroupPriority(groupID uint32, priority uint32) error {
	return e.api.SetNotificationGroupPriority(groupID, priority)
}
