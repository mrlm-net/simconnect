//go:build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/types"
)

// setState updates the connection state and notifies handlers
func (m *Instance) setState(newState ConnectionState) {
	m.mu.Lock()
	oldState := m.state
	if oldState == newState {
		m.mu.Unlock()
		return
	}
	m.state = newState
	// Reuse pre-allocated buffers
	if cap(m.stateHandlersBuf) < len(m.stateHandlers) {
		m.stateHandlersBuf = make([]ConnectionStateChangeHandler, len(m.stateHandlers))
	} else {
		m.stateHandlersBuf = m.stateHandlersBuf[:len(m.stateHandlers)]
	}
	for i, e := range m.stateHandlers {
		m.stateHandlersBuf[i] = e.fn
	}
	handlers := m.stateHandlersBuf

	if cap(m.stateSubsBuf) < len(m.connectionStateSubscriptions) {
		m.stateSubsBuf = make([]*connectionStateSubscription, 0, len(m.connectionStateSubscriptions))
	} else {
		m.stateSubsBuf = m.stateSubsBuf[:0]
	}
	for _, sub := range m.connectionStateSubscriptions {
		m.stateSubsBuf = append(m.stateSubsBuf, sub)
	}
	stateSubs := m.stateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] State changed", "old", oldState, "new", newState)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "ConnectionStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := ConnectionStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] State subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// setSimState updates the simulator state and notifies handlers
func (m *Instance) setSimState(newState SimState) {
	m.mu.Lock()
	oldState := m.simState
	if oldState.Equal(newState) {
		m.mu.Unlock()
		return
	}
	m.simState = newState
	// Reuse pre-allocated buffers
	if cap(m.simStateHandlersBuf) < len(m.simStateHandlers) {
		m.simStateHandlersBuf = make([]SimStateChangeHandler, len(m.simStateHandlers))
	} else {
		m.simStateHandlersBuf = m.simStateHandlersBuf[:len(m.simStateHandlers)]
	}
	for i, e := range m.simStateHandlers {
		m.simStateHandlersBuf[i] = e.fn
	}
	handlers := m.simStateHandlersBuf

	if cap(m.simStateSubsBuf) < len(m.simStateSubscriptions) {
		m.simStateSubsBuf = make([]*simStateSubscription, 0, len(m.simStateSubscriptions))
	} else {
		m.simStateSubsBuf = m.simStateSubsBuf[:0]
	}
	for _, sub := range m.simStateSubscriptions {
		m.simStateSubsBuf = append(m.simStateSubsBuf, sub)
	}
	stateSubs := m.simStateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] SimState changed", "oldCamera", oldState.Camera, "newCamera", newState.Camera)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "SimStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := SimStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] SimState subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// notifySimStateChange notifies handlers and subscriptions of a SimState change.
// This is a helper used by delta update paths where state is already modified in-place.
// The caller must have already updated m.simState and must NOT hold m.mu when calling this.
func (m *Instance) notifySimStateChange(oldState, newState SimState) {
	// Gather handlers and subscriptions under lock
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.simStateHandlersBuf) < len(m.simStateHandlers) {
		m.simStateHandlersBuf = make([]SimStateChangeHandler, len(m.simStateHandlers))
	} else {
		m.simStateHandlersBuf = m.simStateHandlersBuf[:len(m.simStateHandlers)]
	}
	for i, e := range m.simStateHandlers {
		m.simStateHandlersBuf[i] = e.fn
	}
	handlers := m.simStateHandlersBuf

	if cap(m.simStateSubsBuf) < len(m.simStateSubscriptions) {
		m.simStateSubsBuf = make([]*simStateSubscription, 0, len(m.simStateSubscriptions))
	} else {
		m.simStateSubsBuf = m.simStateSubsBuf[:0]
	}
	for _, sub := range m.simStateSubscriptions {
		m.simStateSubsBuf = append(m.simStateSubsBuf, sub)
	}
	stateSubs := m.simStateSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] SimState changed", "oldCamera", oldState.Camera, "newCamera", newState.Camera)

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		safeCallHandler(m.logger, "SimStateChangeHandler", func() {
			h(oldState, newState)
		})
	}

	// Forward state change to subscriptions (non-blocking)
	stateChange := SimStateChange{OldState: oldState, NewState: newState}
	for _, sub := range stateSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- stateChange:
			default:
				// Channel full, skip state change to avoid blocking
				m.logger.Debug("[manager] SimState subscription channel full, dropping state change")
			}
		}
		sub.closeMu.Unlock()
	}
}

// setOpen invokes all registered open handlers and sends to subscriptions
func (m *Instance) setOpen(data types.ConnectionOpenData) {
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.openHandlersBuf) < len(m.openHandlers) {
		m.openHandlersBuf = make([]ConnectionOpenHandler, len(m.openHandlers))
	} else {
		m.openHandlersBuf = m.openHandlersBuf[:len(m.openHandlers)]
	}
	for i, e := range m.openHandlers {
		m.openHandlersBuf[i] = e.fn
	}
	handlers := m.openHandlersBuf

	if cap(m.openSubsBuf) < len(m.openSubscriptions) {
		m.openSubsBuf = make([]*connectionOpenSubscription, 0, len(m.openSubscriptions))
	} else {
		m.openSubsBuf = m.openSubsBuf[:0]
	}
	for _, sub := range m.openSubscriptions {
		m.openSubsBuf = append(m.openSubsBuf, sub)
	}
	openSubs := m.openSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection opened")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCallHandler(m.logger, "ConnectionOpenHandler", func() {
			h(d)
		})
	}

	// Forward open event to subscriptions (non-blocking)
	for _, sub := range openSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- data:
			default:
				// Channel full, skip event to avoid blocking
				m.logger.Debug("[manager] Open subscription channel full, dropping open event")
			}
		}
		sub.closeMu.Unlock()
	}
}

// setQuit invokes all registered quit handlers and sends to subscriptions
func (m *Instance) setQuit(data types.ConnectionQuitData) {
	m.mu.Lock()
	// Reuse pre-allocated buffers
	if cap(m.quitHandlersBuf) < len(m.quitHandlers) {
		m.quitHandlersBuf = make([]ConnectionQuitHandler, len(m.quitHandlers))
	} else {
		m.quitHandlersBuf = m.quitHandlersBuf[:len(m.quitHandlers)]
	}
	for i, e := range m.quitHandlers {
		m.quitHandlersBuf[i] = e.fn
	}
	handlers := m.quitHandlersBuf

	if cap(m.quitSubsBuf) < len(m.quitSubscriptions) {
		m.quitSubsBuf = make([]*connectionQuitSubscription, 0, len(m.quitSubscriptions))
	} else {
		m.quitSubsBuf = m.quitSubsBuf[:0]
	}
	for _, sub := range m.quitSubscriptions {
		m.quitSubsBuf = append(m.quitSubsBuf, sub)
	}
	quitSubs := m.quitSubsBuf
	m.mu.Unlock()

	m.logger.Debug("[manager] Connection quit")

	// Notify handlers outside the lock with panic recovery
	for _, handler := range handlers {
		h := handler // capture for closure
		d := data    // capture for closure
		safeCallHandler(m.logger, "ConnectionQuitHandler", func() {
			h(d)
		})
	}

	// Forward quit event to subscriptions (non-blocking)
	for _, sub := range quitSubs {
		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- data:
			default:
				// Channel full, skip event to avoid blocking
				m.logger.Debug("[manager] Quit subscription channel full, dropping quit event")
			}
		}
		sub.closeMu.Unlock()
	}
}
