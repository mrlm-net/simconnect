//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// OnConnectionStateChange registers a callback to be invoked when connection state changes.
// Returns a unique id that can be used to remove the handler via RemoveConnectionStateChange.
func (m *Instance) OnConnectionStateChange(handler ConnectionStateChangeHandler) string {
	// Wrap as func(interface{}, interface{}) so the internal notify package can type-assert
	// without importing the manager package (which would cause an import cycle).
	wrapped := func(old, new interface{}) {
		handler(old.(ConnectionState), new.(ConnectionState))
	}
	return handlers.RegisterStateHandler(&m.mu, &m.stateHandlers, wrapped, m.logger)
}

// RemoveConnectionStateChange removes a previously registered connection state change handler by id.
func (m *Instance) RemoveConnectionStateChange(id string) error {
	return handlers.RemoveStateHandler(&m.mu, &m.stateHandlers, id, m.logger)
}

// OnMessage registers a callback to be invoked when a message is received.
// Returns a unique id that can be used to remove the handler via RemoveMessage.
func (m *Instance) OnMessage(handler MessageHandler) string {
	return handlers.RegisterMessageHandler(&m.mu, &m.messageHandlers, handler, m.logger)
}

// RemoveMessage removes a previously registered message handler by id.
func (m *Instance) RemoveMessage(id string) error {
	return handlers.RemoveMessageHandler(&m.mu, &m.messageHandlers, id, m.logger)
}

// OnOpen registers a callback to be invoked when the simulator connection opens.
// Returns a unique id that can be used to remove the handler via RemoveOpen.
func (m *Instance) OnOpen(handler ConnectionOpenHandler) string {
	// Wrap as anonymous func so the internal notify package can type-assert
	// without importing the named ConnectionOpenHandler type.
	wrapped := func(data types.ConnectionOpenData) {
		handler(data)
	}
	return handlers.RegisterOpenHandler(&m.mu, &m.openHandlers, wrapped, m.logger)
}

// RemoveOpen removes a previously registered open handler by id.
func (m *Instance) RemoveOpen(id string) error {
	return handlers.RemoveOpenHandler(&m.mu, &m.openHandlers, id, m.logger)
}

// OnQuit registers a callback to be invoked when the simulator quits.
// Returns a unique id that can be used to remove the handler via RemoveQuit.
func (m *Instance) OnQuit(handler ConnectionQuitHandler) string {
	// Wrap as anonymous func so the internal notify package can type-assert
	// without importing the named ConnectionQuitHandler type.
	wrapped := func(data types.ConnectionQuitData) {
		handler(data)
	}
	return handlers.RegisterQuitHandler(&m.mu, &m.quitHandlers, wrapped, m.logger)
}

// RemoveQuit removes a previously registered quit handler by id.
func (m *Instance) RemoveQuit(id string) error {
	return handlers.RemoveQuitHandler(&m.mu, &m.quitHandlers, id, m.logger)
}
