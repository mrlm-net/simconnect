//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SubscribeOnCrashed returns a subscription that receives raw engine.Message for Crashed events
func (m *Instance) SubscribeOnCrashed(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.crashedEventID)
	}
	return m.SubscribeWithFilter(id+"-crashed", bufferSize, filter)
}

// SubscribeOnCrashReset returns a subscription for CrashReset events
func (m *Instance) SubscribeOnCrashReset(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.crashResetEventID)
	}
	return m.SubscribeWithFilter(id+"-crashreset", bufferSize, filter)
}

// SubscribeOnSoundEvent returns a subscription for Sound events
func (m *Instance) SubscribeOnSoundEvent(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.soundEventID)
	}
	return m.SubscribeWithFilter(id+"-sound", bufferSize, filter)
}

// OnCrashed registers a callback invoked when a Crashed system event arrives.
func (m *Instance) OnCrashed(handler CrashedHandler) string {
	return handlers.RegisterCrashedHandler(&m.mu, &m.crashedHandlers, handler, m.logger)
}

// RemoveCrashed removes a previously registered Crashed handler.
func (m *Instance) RemoveCrashed(id string) error {
	return handlers.RemoveCrashedHandler(&m.mu, &m.crashedHandlers, id, m.logger)
}

// OnCrashReset registers a callback invoked when a CrashReset system event arrives.
func (m *Instance) OnCrashReset(handler CrashResetHandler) string {
	return handlers.RegisterCrashResetHandler(&m.mu, &m.crashResetHandlers, handler, m.logger)
}

// RemoveCrashReset removes a previously registered CrashReset handler.
func (m *Instance) RemoveCrashReset(id string) error {
	return handlers.RemoveCrashResetHandler(&m.mu, &m.crashResetHandlers, id, m.logger)
}

// OnSoundEvent registers a callback invoked when a Sound event arrives.
func (m *Instance) OnSoundEvent(handler SoundEventHandler) string {
	return handlers.RegisterSoundEventHandler(&m.mu, &m.soundEventHandlers, handler, m.logger)
}

// RemoveSoundEvent removes a previously registered Sound event handler.
func (m *Instance) RemoveSoundEvent(id string) error {
	return handlers.RemoveSoundEventHandler(&m.mu, &m.soundEventHandlers, id, m.logger)
}

// OnView registers a callback invoked when a View system event arrives.
func (m *Instance) OnView(handler ViewHandler) string {
	return handlers.RegisterViewHandler(&m.mu, &m.viewHandlers, handler, m.logger)
}

// RemoveView removes a previously registered View handler.
func (m *Instance) RemoveView(id string) error {
	return handlers.RemoveViewHandler(&m.mu, &m.viewHandlers, id, m.logger)
}

// SubscribeOnView returns a subscription that receives raw engine.Message for View events
func (m *Instance) SubscribeOnView(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.viewEventID)
	}
	return m.SubscribeWithFilter(id+"-view", bufferSize, filter)
}

// OnFlightPlanDeactivated registers a callback invoked when the active flight plan is deactivated.
func (m *Instance) OnFlightPlanDeactivated(handler FlightPlanDeactivatedHandler) string {
	return handlers.RegisterFlightPlanDeactivatedHandler(&m.mu, &m.flightPlanDeactivatedHandlers, handler, m.logger)
}

// RemoveFlightPlanDeactivated removes a previously registered FlightPlanDeactivated handler.
func (m *Instance) RemoveFlightPlanDeactivated(id string) error {
	return handlers.RemoveFlightPlanDeactivatedHandler(&m.mu, &m.flightPlanDeactivatedHandlers, id, m.logger)
}

// SubscribeOnFlightPlanDeactivated returns a subscription for FlightPlanDeactivated events
func (m *Instance) SubscribeOnFlightPlanDeactivated(id string, bufferSize int) Subscription {
	if id == "" {
		id = handlers.GenerateUUID()
	}
	filter := func(msg engine.Message) bool {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
			return false
		}
		ev := msg.AsEvent()
		return ev != nil && ev.UEventID == types.DWORD(m.flightPlanDeactivatedEventID)
	}
	return m.SubscribeWithFilter(id+"-flightplandeactivated", bufferSize, filter)
}
