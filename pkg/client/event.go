//go:build windows
// +build windows

package client

func (e *Engine) TransmitClientEvent() error {
	panic("TransmitClientEvent not implemented")
}

func (e *Engine) MapClientEventToSimEvent() error {
	panic("MapClientEventToSimEvent not implemented")
}

func (e *Engine) AddClientEventToNotificationGroup() error {
	panic("AddClientEventToNotificationGroup not implemented")
}
