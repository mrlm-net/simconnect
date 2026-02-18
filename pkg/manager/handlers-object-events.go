//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
)

// OnObjectAdded registers a callback invoked when an ObjectAdded system event arrives.
func (m *Instance) OnObjectAdded(handler ObjectChangeHandler) string {
	return handlers.RegisterObjectAddedHandler(&m.mu, &m.objectAddedHandlers, handler, m.logger)
}

// RemoveObjectAdded removes a previously registered ObjectAdded handler.
func (m *Instance) RemoveObjectAdded(id string) error {
	return handlers.RemoveObjectAddedHandler(&m.mu, &m.objectAddedHandlers, id, m.logger)
}

// OnObjectRemoved registers a callback invoked when an ObjectRemoved system event arrives.
func (m *Instance) OnObjectRemoved(handler ObjectChangeHandler) string {
	return handlers.RegisterObjectRemovedHandler(&m.mu, &m.objectRemovedHandlers, handler, m.logger)
}

// RemoveObjectRemoved removes a previously registered ObjectRemoved handler.
func (m *Instance) RemoveObjectRemoved(id string) error {
	return handlers.RemoveObjectRemovedHandler(&m.mu, &m.objectRemovedHandlers, id, m.logger)
}
