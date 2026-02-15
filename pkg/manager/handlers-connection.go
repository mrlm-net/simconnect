//go:build windows
// +build windows

package manager

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// OnConnectionStateChange registers a callback to be invoked when connection state changes.
// Returns a unique id that can be used to remove the handler via RemoveConnectionStateChange.
func (m *Instance) OnConnectionStateChange(handler ConnectionStateChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.stateHandlers = append(m.stateHandlers, instance.StateHandlerEntry{ID: id, Fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered state handler", "id", id)
	}
	return id
}

// RemoveConnectionStateChange removes a previously registered connection state change handler by id.
func (m *Instance) RemoveConnectionStateChange(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.stateHandlers {
		if e.ID == id {
			m.stateHandlers = append(m.stateHandlers[:i], m.stateHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed state handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("state handler not found: %s", id)
}

// OnMessage registers a callback to be invoked when a message is received.
// Returns a unique id that can be used to remove the handler via RemoveMessage.
func (m *Instance) OnMessage(handler MessageHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.messageHandlers = append(m.messageHandlers, instance.MessageHandlerEntry{ID: id, Fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered message handler", "id", id)
	}
	return id
}

// RemoveMessage removes a previously registered message handler by id.
func (m *Instance) RemoveMessage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.messageHandlers {
		if e.ID == id {
			m.messageHandlers = append(m.messageHandlers[:i], m.messageHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed message handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("message handler not found: %s", id)
}

// OnOpen registers a callback to be invoked when the simulator connection opens.
// Returns a unique id that can be used to remove the handler via RemoveOpen.
func (m *Instance) OnOpen(handler ConnectionOpenHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.openHandlers = append(m.openHandlers, instance.OpenHandlerEntry{ID: id, Fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered open handler", "id", id)
	}
	return id
}

// RemoveOpen removes a previously registered open handler by id.
func (m *Instance) RemoveOpen(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.openHandlers {
		if e.ID == id {
			m.openHandlers = append(m.openHandlers[:i], m.openHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed open handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("open handler not found: %s", id)
}

// OnQuit registers a callback to be invoked when the simulator quits.
// Returns a unique id that can be used to remove the handler via RemoveQuit.
func (m *Instance) OnQuit(handler ConnectionQuitHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.quitHandlers = append(m.quitHandlers, instance.QuitHandlerEntry{ID: id, Fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered quit handler", "id", id)
	}
	return id
}

// RemoveQuit removes a previously registered quit handler by id.
func (m *Instance) RemoveQuit(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.quitHandlers {
		if e.ID == id {
			m.quitHandlers = append(m.quitHandlers[:i], m.quitHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed quit handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("quit handler not found: %s", id)
}
