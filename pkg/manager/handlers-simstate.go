//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// OnSimStateChange registers a callback to be invoked when simulator state changes.
// Returns a unique id that can be used to remove the handler via RemoveSimStateChange.
func (m *Instance) OnSimStateChange(handler SimStateChangeHandler) string {
	// Wrap as func(interface{}, interface{}) so the internal notify package can type-assert
	// without importing the manager package (which would cause an import cycle).
	wrapped := func(old, new interface{}) {
		handler(old.(SimState), new.(SimState))
	}
	return handlers.RegisterSimStateHandler(&m.mu, &m.simStateHandlers, wrapped, m.logger)
}

// RemoveSimStateChange removes a previously registered simulator state change handler by id.
func (m *Instance) RemoveSimStateChange(id string) error {
	return handlers.RemoveSimStateHandler(&m.mu, &m.simStateHandlers, id, m.logger)
}

// OnPause registers a callback invoked when the simulator pause state changes.
func (m *Instance) OnPause(handler PauseHandler) string {
	return handlers.RegisterPauseHandler(&m.mu, &m.pauseHandlers, handler, m.logger)
}

// RemovePause removes a previously registered Pause handler.
func (m *Instance) RemovePause(id string) error {
	return handlers.RemovePauseHandler(&m.mu, &m.pauseHandlers, id, m.logger)
}

// SubscribeOnPause returns a subscription that receives raw engine.Message for Pause events
func (m *Instance) SubscribeOnPause(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
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
	return handlers.RegisterSimRunningHandler(&m.mu, &m.simRunningHandlers, handler, m.logger)
}

// RemoveSimRunning removes a previously registered SimRunning handler.
func (m *Instance) RemoveSimRunning(id string) error {
	return handlers.RemoveSimRunningHandler(&m.mu, &m.simRunningHandlers, id, m.logger)
}

// SubscribeOnSimRunning returns a subscription that receives raw engine.Message for Sim running events
func (m *Instance) SubscribeOnSimRunning(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
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
