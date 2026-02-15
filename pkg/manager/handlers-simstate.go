//go:build windows
// +build windows

package manager

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// OnSimStateChange registers a callback to be invoked when simulator state changes.
// Returns a unique id that can be used to remove the handler via RemoveSimStateChange.
func (m *Instance) OnSimStateChange(handler SimStateChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.simStateHandlers = append(m.simStateHandlers, simStateHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered SimState handler", "id", id)
	}
	return id
}

// RemoveSimStateChange removes a previously registered simulator state change handler by id.
func (m *Instance) RemoveSimStateChange(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.simStateHandlers {
		if e.id == id {
			m.simStateHandlers = append(m.simStateHandlers[:i], m.simStateHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed SimState handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SimState handler not found: %s", id)
}

// OnPause registers a callback invoked when the simulator pause state changes.
func (m *Instance) OnPause(handler PauseHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.pauseHandlers = append(m.pauseHandlers, pauseHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered Pause handler", "id", id)
	}
	return id
}

// RemovePause removes a previously registered Pause handler.
func (m *Instance) RemovePause(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.pauseHandlers {
		if e.id == id {
			m.pauseHandlers = append(m.pauseHandlers[:i], m.pauseHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed Pause handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("Pause handler not found: %s", id)
}

// SubscribeOnPause returns a subscription that receives raw engine.Message for Pause events
func (m *Instance) SubscribeOnPause(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.pauseEventID)
	}
	return m.SubscribeWithFilter(id+"-pause", bufferSize, filter)
}

// OnSimRunning registers a callback invoked when the simulator running state changes.
func (m *Instance) OnSimRunning(handler SimRunningHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.simRunningHandlers = append(m.simRunningHandlers, simRunningHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered SimRunning handler", "id", id)
	}
	return id
}

// RemoveSimRunning removes a previously registered SimRunning handler.
func (m *Instance) RemoveSimRunning(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.simRunningHandlers {
		if e.id == id {
			m.simRunningHandlers = append(m.simRunningHandlers[:i], m.simRunningHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed SimRunning handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SimRunning handler not found: %s", id)
}

// SubscribeOnSimRunning returns a subscription that receives raw engine.Message for Sim running events
func (m *Instance) SubscribeOnSimRunning(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.simEventID)
	}
	return m.SubscribeWithFilter(id+"-simrunning", bufferSize, filter)
}
