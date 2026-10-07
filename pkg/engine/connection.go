//go:build windows
// +build windows

package engine

// Connect opens the connection to the simulator.
func (e *Engine) Connect() error {
	return e.api.Connect()
}

// Disconnect stops the dispatcher and closes the connection.
//
// An Engine is single-use: Disconnect cancels its context and closes its
// Stream for good, so Connect after Disconnect opens a connection nobody
// reads. To reconnect, make a new Engine with New (as pkg/manager does).
func (e *Engine) Disconnect() error {
	e.cancel()
	e.sync.Wait()
	return e.api.Disconnect()
}
