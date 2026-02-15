//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// processObjectEvent handles SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE messages.
func (m *Instance) processObjectEvent(msg engine.Message) {
	objMsg := msg.AsEventObjectAddRemove()
	if objMsg == nil {
		return
	}

	if objMsg.UEventID == types.DWORD(m.objectAddedEventID) {
		m.logger.Debug("[manager] ObjectAdded event", "id", objMsg.DwData, "type", objMsg.EObjType)
		// Invoke object added handlers with panic recovery
		m.mu.RLock()
		if cap(m.objectChangeHandlersBuf) < len(m.objectAddedHandlers) {
			m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectAddedHandlers))
		} else {
			m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectAddedHandlers)]
		}
		for i, e := range m.objectAddedHandlers {
			m.objectChangeHandlersBuf[i] = e.fn
		}
		hs := m.objectChangeHandlersBuf
		m.mu.RUnlock()
		objID := uint32(objMsg.DwData)
		objType := objMsg.EObjType
		for _, h := range hs {
			handler := h // capture for closure
			safeCallHandler(m.logger, "ObjectAddedHandler", func() {
				handler(objID, objType)
			})
		}
	}

	if objMsg.UEventID == types.DWORD(m.objectRemovedEventID) {
		m.logger.Debug("[manager] ObjectRemoved event", "id", objMsg.DwData, "type", objMsg.EObjType)
		m.mu.RLock()
		if cap(m.objectChangeHandlersBuf) < len(m.objectRemovedHandlers) {
			m.objectChangeHandlersBuf = make([]ObjectChangeHandler, len(m.objectRemovedHandlers))
		} else {
			m.objectChangeHandlersBuf = m.objectChangeHandlersBuf[:len(m.objectRemovedHandlers)]
		}
		for i, e := range m.objectRemovedHandlers {
			m.objectChangeHandlersBuf[i] = e.fn
		}
		hs := m.objectChangeHandlersBuf
		m.mu.RUnlock()
		objID := uint32(objMsg.DwData)
		objType := objMsg.EObjType
		for _, h := range hs {
			handler := h // capture for closure
			safeCallHandler(m.logger, "ObjectRemovedHandler", func() {
				handler(objID, objType)
			})
		}
	}
}
