//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/dispatch"
)

// processObjectEvent handles SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE messages.
func (m *Instance) processObjectEvent(msg engine.Message) {
	eventID, objectID, objType := dispatch.ExtractObjectEventData(msg)
	if eventID == 0 {
		return
	}

	if eventID == m.objectAddedEventID {
		m.logger.Debug("[manager] ObjectAdded event", "id", objectID, "type", objType)
		// Invoke object added handlers with panic recovery
		m.mu.RLock()
		if cap(m.objectChangeHandlersBuf) < len(m.objectAddedHandlers) {
			m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectAddedHandlers))
		} else {
			m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectAddedHandlers)]
		}
		for i, e := range m.objectAddedHandlers {
			m.objectChangeHandlersBuf[i] = e.Fn.(ObjectChangeHandler)
		}
		hs := m.objectChangeHandlersBuf
		m.mu.RUnlock()
		for _, h := range hs {
			handler := h // capture for closure
			id := objectID
			typ := objType
			safeCallHandler(m.logger, "ObjectAddedHandler", func() {
				handler(id, typ)
			})
		}
	}

	if eventID == m.objectRemovedEventID {
		m.logger.Debug("[manager] ObjectRemoved event", "id", objectID, "type", objType)
		m.mu.RLock()
		if cap(m.objectChangeHandlersBuf) < len(m.objectRemovedHandlers) {
			m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectRemovedHandlers))
		} else {
			m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectRemovedHandlers)]
		}
		for i, e := range m.objectRemovedHandlers {
			m.objectChangeHandlersBuf[i] = e.Fn.(ObjectChangeHandler)
		}
		hs := m.objectChangeHandlersBuf
		m.mu.RUnlock()
		for _, h := range hs {
			handler := h // capture for closure
			id := objectID
			typ := objType
			safeCallHandler(m.logger, "ObjectRemovedHandler", func() {
				handler(id, typ)
			})
		}
	}
}
