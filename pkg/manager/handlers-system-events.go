//go:build windows
// +build windows

package manager

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SubscribeOnCrashed returns a subscription that receives raw engine.Message for Crashed events
func (m *Instance) SubscribeOnCrashed(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
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
		id = generateUUID()
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
		id = generateUUID()
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
	id := generateUUID()
	m.mu.Lock()
	m.crashedHandlers = append(m.crashedHandlers, crashedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered Crashed handler", "id", id)
	}
	return id
}

// RemoveCrashed removes a previously registered Crashed handler.
func (m *Instance) RemoveCrashed(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.crashedHandlers {
		if e.id == id {
			m.crashedHandlers = append(m.crashedHandlers[:i], m.crashedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed Crashed handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("Crashed handler not found: %s", id)
}

// OnCrashReset registers a callback invoked when a CrashReset system event arrives.
func (m *Instance) OnCrashReset(handler CrashResetHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.crashResetHandlers = append(m.crashResetHandlers, crashResetHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered CrashReset handler", "id", id)
	}
	return id
}

// RemoveCrashReset removes a previously registered CrashReset handler.
func (m *Instance) RemoveCrashReset(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.crashResetHandlers {
		if e.id == id {
			m.crashResetHandlers = append(m.crashResetHandlers[:i], m.crashResetHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed CrashReset handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("CrashReset handler not found: %s", id)
}

// OnSoundEvent registers a callback invoked when a Sound event arrives.
func (m *Instance) OnSoundEvent(handler SoundEventHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.soundEventHandlers = append(m.soundEventHandlers, soundEventHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered SoundEvent handler", "id", id)
	}
	return id
}

// RemoveSoundEvent removes a previously registered Sound event handler.
func (m *Instance) RemoveSoundEvent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.soundEventHandlers {
		if e.id == id {
			m.soundEventHandlers = append(m.soundEventHandlers[:i], m.soundEventHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed SoundEvent handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("SoundEvent handler not found: %s", id)
}

// OnView registers a callback invoked when a View system event arrives.
func (m *Instance) OnView(handler ViewHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.viewHandlers = append(m.viewHandlers, viewHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered View handler", "id", id)
	}
	return id
}

// RemoveView removes a previously registered View handler.
func (m *Instance) RemoveView(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.viewHandlers {
		if e.id == id {
			m.viewHandlers = append(m.viewHandlers[:i], m.viewHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed View handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("View handler not found: %s", id)
}

// SubscribeOnView returns a subscription that receives raw engine.Message for View events
func (m *Instance) SubscribeOnView(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
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
	id := generateUUID()
	m.mu.Lock()
	m.flightPlanDeactivatedHandlers = append(m.flightPlanDeactivatedHandlers, flightPlanDeactivatedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightPlanDeactivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanDeactivated removes a previously registered FlightPlanDeactivated handler.
func (m *Instance) RemoveFlightPlanDeactivated(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightPlanDeactivatedHandlers {
		if e.id == id {
			m.flightPlanDeactivatedHandlers = append(m.flightPlanDeactivatedHandlers[:i], m.flightPlanDeactivatedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightPlanDeactivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanDeactivated handler not found: %s", id)
}

// SubscribeOnFlightPlanDeactivated returns a subscription for FlightPlanDeactivated events
func (m *Instance) SubscribeOnFlightPlanDeactivated(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
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
