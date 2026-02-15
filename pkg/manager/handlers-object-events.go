//go:build windows
// +build windows

package manager

import "fmt"

// OnObjectAdded registers a callback invoked when an ObjectAdded system event arrives.
func (m *Instance) OnObjectAdded(handler ObjectChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.objectAddedHandlers = append(m.objectAddedHandlers, objectChangeHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered ObjectAdded handler", "id", id)
	}
	return id
}

// RemoveObjectAdded removes a previously registered ObjectAdded handler.
func (m *Instance) RemoveObjectAdded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.objectAddedHandlers {
		if e.id == id {
			m.objectAddedHandlers = append(m.objectAddedHandlers[:i], m.objectAddedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed ObjectAdded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectAdded handler not found: %s", id)
}

// OnObjectRemoved registers a callback invoked when an ObjectRemoved system event arrives.
func (m *Instance) OnObjectRemoved(handler ObjectChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.objectRemovedHandlers = append(m.objectRemovedHandlers, objectChangeHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered ObjectRemoved handler", "id", id)
	}
	return id
}

// RemoveObjectRemoved removes a previously registered ObjectRemoved handler.
func (m *Instance) RemoveObjectRemoved(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.objectRemovedHandlers {
		if e.id == id {
			m.objectRemovedHandlers = append(m.objectRemovedHandlers[:i], m.objectRemovedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed ObjectRemoved handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectRemoved handler not found: %s", id)
}
