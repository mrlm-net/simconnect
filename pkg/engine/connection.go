//go:build windows
// +build windows

package engine

func (e *Engine) Connect() error {
	return e.api.Connect()
}

func (e *Engine) Disconnect() error {
	return e.api.Disconnect()
}
