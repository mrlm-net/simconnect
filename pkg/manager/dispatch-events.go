//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// processEventMessage handles SIMCONNECT_RECV_ID_EVENT messages.
func (m *Instance) processEventMessage(msg engine.Message) {
	eventMsg := msg.AsEvent()
	switch eventMsg.UEventID {
	case types.DWORD(m.pauseEventID):
		// Handle pause event
		newPausedState := eventMsg.DwData == 1

		m.mu.Lock()
		if m.simState.Paused != newPausedState {
			oldState := m.simState
			m.simState.Paused = newPausedState
			newState := m.simState
			// Copy handlers under lock using pre-allocated buffer
			if cap(m.pauseHandlersBuf) < len(m.pauseHandlers) {
				m.pauseHandlersBuf = make([]PauseHandler, len(m.pauseHandlers))
			} else {
				m.pauseHandlersBuf = m.pauseHandlersBuf[:len(m.pauseHandlers)]
			}
			for i, e := range m.pauseHandlers {
				m.pauseHandlersBuf[i] = e.Fn.(PauseHandler)
			}
			hs := m.pauseHandlersBuf
			m.mu.Unlock()
			m.notifySimStateChange(oldState, newState)
			for _, h := range hs {
				handler := h
				paused := newPausedState
				safeCallHandler(m.logger, "PauseHandler", func() {
					handler(paused)
				})
			}
		} else {
			m.mu.Unlock()
		}

	case types.DWORD(m.simEventID):
		// Handle sim running event
		newSimRunningState := eventMsg.DwData == 1

		m.mu.Lock()
		if m.simState.SimRunning != newSimRunningState {
			oldState := m.simState
			m.simState.SimRunning = newSimRunningState
			newState := m.simState
			// Copy handlers under lock using pre-allocated buffer
			if cap(m.simRunningHandlersBuf) < len(m.simRunningHandlers) {
				m.simRunningHandlersBuf = make([]SimRunningHandler, len(m.simRunningHandlers))
			} else {
				m.simRunningHandlersBuf = m.simRunningHandlersBuf[:len(m.simRunningHandlers)]
			}
			for i, e := range m.simRunningHandlers {
				m.simRunningHandlersBuf[i] = e.Fn.(SimRunningHandler)
			}
			hs := m.simRunningHandlersBuf
			m.mu.Unlock()
			m.notifySimStateChange(oldState, newState)
			for _, h := range hs {
				handler := h
				running := newSimRunningState
				safeCallHandler(m.logger, "SimRunningHandler", func() {
					handler(running)
				})
			}
		} else {
			m.mu.Unlock()
		}

	case types.DWORD(m.crashedEventID):
		// Handle crashed event
		newCrashed := eventMsg.DwData == 1

		m.mu.Lock()
		if m.simState.Crashed != newCrashed {
			oldState := m.simState
			m.simState.Crashed = newCrashed
			newState := m.simState

			// Copy handlers under lock using pre-allocated buffer
			if cap(m.crashedHandlersBuf) < len(m.crashedHandlers) {
				m.crashedHandlersBuf = make([]CrashedHandler, len(m.crashedHandlers))
			} else {
				m.crashedHandlersBuf = m.crashedHandlersBuf[:len(m.crashedHandlers)]
			}
			for i, e := range m.crashedHandlers {
				m.crashedHandlersBuf[i] = e.Fn.(CrashedHandler)
			}
			hs := m.crashedHandlersBuf
			m.mu.Unlock()

			m.notifySimStateChange(oldState, newState)

			// Invoke handlers outside lock with panic recovery
			for _, h := range hs {
				handler := h // capture for closure
				safeCallHandler(m.logger, "CrashedHandler", func() {
					handler()
				})
			}
		} else {
			m.mu.Unlock()
		}

	case types.DWORD(m.crashResetEventID):
		// Handle crash reset event
		newReset := eventMsg.DwData == 1

		m.mu.Lock()
		if m.simState.CrashReset != newReset {
			oldState := m.simState
			m.simState.CrashReset = newReset
			newState := m.simState

			// Copy handlers under lock using pre-allocated buffer
			if cap(m.crashResetHandlersBuf) < len(m.crashResetHandlers) {
				m.crashResetHandlersBuf = make([]CrashResetHandler, len(m.crashResetHandlers))
			} else {
				m.crashResetHandlersBuf = m.crashResetHandlersBuf[:len(m.crashResetHandlers)]
			}
			for i, e := range m.crashResetHandlers {
				m.crashResetHandlersBuf[i] = e.Fn.(CrashResetHandler)
			}
			hs := m.crashResetHandlersBuf
			m.mu.Unlock()

			m.notifySimStateChange(oldState, newState)

			// Invoke handlers outside lock with panic recovery
			for _, h := range hs {
				handler := h // capture for closure
				safeCallHandler(m.logger, "CrashResetHandler", func() {
					handler()
				})
			}
		} else {
			m.mu.Unlock()
		}

	case types.DWORD(m.soundEventID):
		// Handle sound event
		newSound := uint32(eventMsg.DwData)

		m.mu.Lock()
		if m.simState.Sound != newSound {
			oldState := m.simState
			m.simState.Sound = newSound
			newState := m.simState

			// Copy handlers under lock using pre-allocated buffer
			if cap(m.soundEventHandlersBuf) < len(m.soundEventHandlers) {
				m.soundEventHandlersBuf = make([]SoundEventHandler, len(m.soundEventHandlers))
			} else {
				m.soundEventHandlersBuf = m.soundEventHandlersBuf[:len(m.soundEventHandlers)]
			}
			for i, e := range m.soundEventHandlers {
				m.soundEventHandlersBuf[i] = e.Fn.(SoundEventHandler)
			}
			hs := m.soundEventHandlersBuf
			m.mu.Unlock()

			m.notifySimStateChange(oldState, newState)

			// Invoke handlers outside lock with panic recovery
			for _, h := range hs {
				handler := h      // capture for closure
				sound := newSound // capture for closure
				safeCallHandler(m.logger, "SoundEventHandler", func() {
					handler(sound)
				})
			}
		} else {
			m.mu.Unlock()
		}

	case types.DWORD(m.viewEventID):
		// Handle view change event
		newView := uint32(eventMsg.DwData)
		m.logger.Debug("[manager] View event", "viewID", newView)

		m.mu.RLock()
		if cap(m.viewHandlersBuf) < len(m.viewHandlers) {
			m.viewHandlersBuf = make([]ViewHandler, len(m.viewHandlers))
		} else {
			m.viewHandlersBuf = m.viewHandlersBuf[:len(m.viewHandlers)]
		}
		for i, e := range m.viewHandlers {
			m.viewHandlersBuf[i] = e.Fn.(ViewHandler)
		}
		hs := m.viewHandlersBuf
		m.mu.RUnlock()

		for _, h := range hs {
			handler := h
			view := newView
			safeCallHandler(m.logger, "ViewHandler", func() {
				handler(view)
			})
		}

	case types.DWORD(m.flightPlanDeactivatedEventID):
		// Handle flight plan deactivated event
		m.logger.Debug("[manager] FlightPlanDeactivated event")

		m.mu.RLock()
		if cap(m.flightPlanDeactivatedHandlersBuf) < len(m.flightPlanDeactivatedHandlers) {
			m.flightPlanDeactivatedHandlersBuf = make([]FlightPlanDeactivatedHandler, len(m.flightPlanDeactivatedHandlers))
		} else {
			m.flightPlanDeactivatedHandlersBuf = m.flightPlanDeactivatedHandlersBuf[:len(m.flightPlanDeactivatedHandlers)]
		}
		for i, e := range m.flightPlanDeactivatedHandlers {
			m.flightPlanDeactivatedHandlersBuf[i] = e.Fn.(FlightPlanDeactivatedHandler)
		}
		hs := m.flightPlanDeactivatedHandlersBuf
		m.mu.RUnlock()

		for _, h := range hs {
			handler := h
			safeCallHandler(m.logger, "FlightPlanDeactivatedHandler", func() {
				handler()
			})
		}

	default:
		// Check if this is a custom system event
		eventID := uint32(eventMsg.UEventID)
		if eventID >= CustomEventIDMin && eventID <= CustomEventIDMax {
			m.mu.RLock()
			var ce *instance.CustomSystemEvent
			for _, entry := range m.customSystemEvents {
				if entry.ID == eventID {
					ce = entry
					break
				}
			}
			if ce != nil && len(ce.Handlers) > 0 {
				eventName := ce.Name
				eventData := uint32(eventMsg.DwData)
				handlers := make([]CustomSystemEventHandler, len(ce.Handlers))
				for i, e := range ce.Handlers {
					handlers[i] = e.Fn.(CustomSystemEventHandler)
				}
				m.mu.RUnlock()
				for _, h := range handlers {
					handler := h
					name := eventName
					data := eventData
					safeCallHandler(m.logger, "CustomSystemEventHandler", func() {
						handler(name, data)
					})
				}
			} else {
				m.mu.RUnlock()
			}
		}
	}
}
