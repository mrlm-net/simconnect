//go:build windows

package manager

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// OnConnectionStateChange registers a callback to be invoked when connection state changes.
// Returns a unique id that can be used to remove the handler via RemoveConnectionStateChange.
func (m *Instance) OnConnectionStateChange(handler ConnectionStateChangeHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.stateHandlers = append(m.stateHandlers, stateHandlerEntry{id: id, fn: handler})
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
		if e.id == id {
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
	m.messageHandlers = append(m.messageHandlers, messageHandlerEntry{id: id, fn: handler})
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
		if e.id == id {
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
	m.openHandlers = append(m.openHandlers, openHandlerEntry{id: id, fn: handler})
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
		if e.id == id {
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
	m.quitHandlers = append(m.quitHandlers, quitHandlerEntry{id: id, fn: handler})
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
		if e.id == id {
			m.quitHandlers = append(m.quitHandlers[:i], m.quitHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed quit handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("quit handler not found: %s", id)
}

// OnFlightLoaded registers a callback invoked when a FlightLoaded system event arrives.
func (m *Instance) OnFlightLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightLoadedHandlers = append(m.flightLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightLoaded handler", "id", id)
	}
	return id
}

// RemoveFlightLoaded removes a previously registered FlightLoaded handler.
func (m *Instance) RemoveFlightLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightLoadedHandlers {
		if e.id == id {
			m.flightLoadedHandlers = append(m.flightLoadedHandlers[:i], m.flightLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightLoaded handler not found: %s", id)
}

// OnAircraftLoaded registers a callback invoked when an AircraftLoaded system event arrives.
func (m *Instance) OnAircraftLoaded(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered AircraftLoaded handler", "id", id)
	}
	return id
}

// RemoveAircraftLoaded removes a previously registered AircraftLoaded handler.
func (m *Instance) RemoveAircraftLoaded(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.aircraftLoadedHandlers {
		if e.id == id {
			m.aircraftLoadedHandlers = append(m.aircraftLoadedHandlers[:i], m.aircraftLoadedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed AircraftLoaded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("AircraftLoaded handler not found: %s", id)
}

// OnFlightPlanActivated registers a callback invoked when a FlightPlanActivated system event arrives.
func (m *Instance) OnFlightPlanActivated(handler FlightLoadedHandler) string {
	id := generateUUID()
	m.mu.Lock()
	m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers, flightLoadedHandlerEntry{id: id, fn: handler})
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug("[manager] Registered FlightPlanActivated handler", "id", id)
	}
	return id
}

// RemoveFlightPlanActivated removes a previously registered FlightPlanActivated handler.
func (m *Instance) RemoveFlightPlanActivated(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.flightPlanActivatedHandlers {
		if e.id == id {
			m.flightPlanActivatedHandlers = append(m.flightPlanActivatedHandlers[:i], m.flightPlanActivatedHandlers[i+1:]...)
			if m.logger != nil {
				m.logger.Debug("[manager] Removed FlightPlanActivated handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("FlightPlanActivated handler not found: %s", id)
}

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
