//go:build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// processMessage handles a single message from the simulator.
// This method ensures defer msg.Release() fires at the end of each message processing,
// not at the end of the entire connection loop.
func (m *Instance) processMessage(msg engine.Message) {
	// Release the message buffer back to pool when done processing
	// This is critical for memory efficiency under high message load
	defer msg.Release()

	if msg.Err != nil {
		m.logger.Error("[manager] Stream error", "error", msg.Err)
		return
	}

	// Check for connection ready (OPEN) message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
		m.logger.Debug("[manager] Received OPEN message, connection is now available")
		m.setState(StateAvailable)

		// Extract version information from OPEN message
		openMsg := msg.AsOpen()
		if openMsg != nil {
			appName := engine.BytesToString(openMsg.SzApplicationName[:])
			openData := types.ConnectionOpenData{
				ApplicationName:         appName,
				ApplicationVersionMajor: uint32(openMsg.DwApplicationVersionMajor),
				ApplicationVersionMinor: uint32(openMsg.DwApplicationVersionMinor),
				ApplicationBuildMajor:   uint32(openMsg.DwApplicationBuildMajor),
				ApplicationBuildMinor:   uint32(openMsg.DwApplicationBuildMinor),
				SimConnectVersionMajor:  uint32(openMsg.DwSimConnectVersionMajor),
				SimConnectVersionMinor:  uint32(openMsg.DwSimConnectVersionMinor),
				SimConnectBuildMajor:    uint32(openMsg.DwSimConnectBuildMajor),
				SimConnectBuildMinor:    uint32(openMsg.DwSimConnectBuildMinor),
			}
			m.setOpen(openData)
		}

		// Initialize simulator state and request camera data
		m.mu.Lock()
		client := m.engine
		m.mu.Unlock()

		if client != nil {
			m.registerSimStateSubscriptions(client)
		}
		return
	}

	// Check for quit message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_QUIT {
		m.logger.Debug("[manager] Received QUIT message from simulator")
		quitData := types.ConnectionQuitData{}
		m.setQuit(quitData)
		m.setSimState(defaultSimState())
		m.setState(StateDisconnected)
		m.mu.Lock()
		m.engine = nil
		m.mu.Unlock()
		return
	}

	// Handle pause and sim events
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT {
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
					m.pauseHandlersBuf[i] = e.fn
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
					m.simRunningHandlersBuf[i] = e.fn
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
					m.crashedHandlersBuf[i] = e.fn
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
					m.crashResetHandlersBuf[i] = e.fn
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
					m.soundEventHandlersBuf[i] = e.fn
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
				m.viewHandlersBuf[i] = e.fn
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
				m.flightPlanDeactivatedHandlersBuf[i] = e.fn
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
				var ce *customSystemEvent
				for _, entry := range m.customSystemEvents {
					if entry.id == eventID {
						ce = entry
						break
					}
				}
				if ce != nil && len(ce.handlers) > 0 {
					eventName := ce.name
					eventData := uint32(eventMsg.DwData)
					handlers := make([]CustomSystemEventHandler, len(ce.handlers))
					for i, e := range ce.handlers {
						handlers[i] = e.fn
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
		// (Position change event handling removed)
	}

	// Handle filename events (FlightLoaded, AircraftLoaded, FlightPlanActivated)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_FILENAME {
		fnameMsg := msg.AsEventFilename()
		if fnameMsg != nil {
			name := engine.BytesToString(fnameMsg.SzFileName[:])
			if fnameMsg.UEventID == types.DWORD(m.flightLoadedEventID) {
				m.logger.Debug("[manager] FlightLoaded event", "filename", name)
				// Invoke registered FlightLoaded handlers with panic recovery
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.flightLoadedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightLoadedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightLoadedHandlers)]
				}
				for i, e := range m.flightLoadedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "FlightLoadedHandler", func() {
						handler(n)
					})
				}
			}

			if fnameMsg.UEventID == types.DWORD(m.aircraftLoadedEventID) {
				m.logger.Debug("[manager] AircraftLoaded event", "filename", name)
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.aircraftLoadedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.aircraftLoadedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.aircraftLoadedHandlers)]
				}
				for i, e := range m.aircraftLoadedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "AircraftLoadedHandler", func() {
						handler(n)
					})
				}
			}

			if fnameMsg.UEventID == types.DWORD(m.flightPlanActivatedEventID) {
				m.logger.Debug("[manager] FlightPlanActivated event", "filename", name)
				m.mu.RLock()
				if cap(m.flightLoadedHandlersBuf) < len(m.flightPlanActivatedHandlers) {
					m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightPlanActivatedHandlers))
				} else {
					m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightPlanActivatedHandlers)]
				}
				for i, e := range m.flightPlanActivatedHandlers {
					m.flightLoadedHandlersBuf[i] = e.fn
				}
				hs := m.flightLoadedHandlersBuf
				m.mu.RUnlock()
				for _, h := range hs {
					handler := h // capture for closure
					n := name    // capture for closure
					safeCallHandler(m.logger, "FlightPlanActivatedHandler", func() {
						handler(n)
					})
				}
			}
		}
	}

	// Handle object add/remove events (ObjectAdded, ObjectRemoved)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE {
		objMsg := msg.AsEventObjectAddRemove()
		if objMsg != nil {
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
	}

	// Handle camera state data
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		simObjMsg := msg.AsSimObjectData()
		if uint32(simObjMsg.DwRequestID) == m.cameraRequestID && uint32(simObjMsg.DwDefineID) == m.cameraDefinitionID {
			// Extract camera state and substate from data
			stateData := engine.CastDataAs[simStateDataStruct](&simObjMsg.DwData)

			// Build new state from sim state data (no lock needed)
			newState := SimState{
				Camera:                     CameraState(stateData.CameraState),
				Substate:                   CameraSubstate(stateData.CameraSubstate),
				SimulationRate:             stateData.SimulationRate,
				SimulationTime:             stateData.SimulationTime,
				LocalTime:                  stateData.LocalTime,
				ZuluTime:                   stateData.ZuluTime,
				IsInVR:                     stateData.IsInVR == 1,
				IsUsingMotionControllers:   stateData.IsUsingMotionControllers == 1,
				IsUsingJoystickThrottle:    stateData.IsUsingJoystickThrottle == 1,
				IsInRTC:                    stateData.IsInRTC == 1,
				IsAvatar:                   stateData.IsAvatar == 1,
				IsAircraft:                 stateData.IsAircraft == 1,
				LocalDay:                   int(stateData.LocalDay),
				LocalMonth:                 int(stateData.LocalMonth),
				LocalYear:                  int(stateData.LocalYear),
				ZuluDay:                    int(stateData.ZuluDay),
				ZuluMonth:                  int(stateData.ZuluMonth),
				ZuluYear:                   int(stateData.ZuluYear),
				Realism:                    stateData.Realism,
				VisualModelRadius:          stateData.VisualModelRadius,
				SimDisabled:                stateData.SimDisabled == 1,
				RealismCrashDetection:      stateData.RealismCrashDetection == 1,
				RealismCrashWithOthers:     stateData.RealismCrashWithOthers == 1,
				TrackIREnabled:             stateData.TrackIREnabled == 1,
				UserInputEnabled:           stateData.UserInputEnabled == 1,
				SimOnGround:                stateData.SimOnGround == 1,
				AmbientTemperature:         stateData.AmbientTemperature,
				AmbientPressure:            stateData.AmbientPressure,
				AmbientWindVelocity:        stateData.AmbientWindVelocity,
				AmbientWindDirection:       stateData.AmbientWindDirection,
				AmbientVisibility:          stateData.AmbientVisibility,
				AmbientInCloud:             stateData.AmbientInCloud == 1,
				AmbientPrecipState:         uint32(stateData.AmbientPrecipState),
				BarometerPressure:          stateData.BarometerPressure,
				SeaLevelPressure:           stateData.SeaLevelPressure,
				GroundAltitude:             stateData.GroundAltitude,
				MagVar:                     stateData.MagVar,
				SurfaceType:                uint32(stateData.SurfaceType),
				Latitude:                   stateData.Latitude,
				Longitude:                  stateData.Longitude,
				Altitude:                   stateData.Altitude,
				IndicatedAltitude:          stateData.IndicatedAltitude,
				TrueHeading:                stateData.TrueHeading,
				MagneticHeading:            stateData.MagneticHeading,
				Pitch:                      stateData.Pitch,
				Bank:                       stateData.Bank,
				GroundSpeed:                stateData.GroundSpeed,
				IndicatedAirspeed:          stateData.IndicatedAirspeed,
				TrueAirspeed:               stateData.TrueAirspeed,
				VerticalSpeed:              stateData.VerticalSpeed,
				SmartCameraActive:          stateData.SmartCameraActive == 1,
				HandAnimState:              stateData.HandAnimState,
				HideAvatarInAircraft:       stateData.HideAvatarInAircraft == 1,
				MissionScore:               stateData.MissionScore,
				ParachuteOpen:              stateData.ParachuteOpen == 1,
				ZuluSunriseTime:            stateData.ZuluSunriseTime,
				ZuluSunsetTime:             stateData.ZuluSunsetTime,
				TimeZoneOffset:             stateData.TimeZoneOffset,
				TooltipUnits:               stateData.TooltipUnits,
				UnitsOfMeasure:             stateData.UnitsOfMeasure,
				AmbientInSmoke:             stateData.AmbientInSmoke == 1,
				EnvSmokeDensity:            stateData.EnvSmokeDensity,
				EnvCloudDensity:            stateData.EnvCloudDensity,
				DensityAltitude:            stateData.DensityAltitude,
				SeaLevelAmbientTemperature: stateData.SeaLevelAmbientTemperature,
			}

			// Short lock: preserve event-driven fields and compare
			m.mu.Lock()
			newState.Paused = m.simState.Paused
			newState.SimRunning = m.simState.SimRunning
			newState.Crashed = m.simState.Crashed
			newState.CrashReset = m.simState.CrashReset
			newState.Sound = m.simState.Sound

			if m.simState == newState {
				m.mu.Unlock()
				return
			}
			oldState := m.simState
			m.simState = newState
			m.mu.Unlock()

			if !oldState.Equal(newState) {
				// States are not equal
				// notify handlers of the change
				m.notifySimStateChange(oldState, newState)
			}

		}
	}

	// Forward message to registered handlers
	m.mu.RLock()
	// Reuse pre-allocated slices, grow if necessary
	if cap(m.handlersBuf) < len(m.messageHandlers) {
		m.handlersBuf = make([]MessageHandler, len(m.messageHandlers))
	} else {
		m.handlersBuf = m.handlersBuf[:len(m.messageHandlers)]
	}
	for i, e := range m.messageHandlers {
		m.handlersBuf[i] = e.fn
	}
	if cap(m.subsBuf) < len(m.subscriptions) {
		m.subsBuf = make([]*subscription, 0, len(m.subscriptions))
	} else {
		m.subsBuf = m.subsBuf[:0]
	}
	for _, sub := range m.subscriptions {
		m.subsBuf = append(m.subsBuf, sub)
	}
	m.mu.RUnlock()

	for _, handler := range m.handlersBuf {
		h := handler   // capture for closure
		message := msg // capture for closure
		safeCallHandler(m.logger, "MessageHandler", func() {
			h(message)
		})
	}

	// Forward message to subscriptions (non-blocking)
	for _, sub := range m.subsBuf {
		// fast-path: skip closed subscriptions
		sub.closeMu.Lock()
		closed := sub.closed.Load()
		sub.closeMu.Unlock()
		if closed {
			continue
		}

		// Determine whether this subscription should receive the message
		allowed := true
		if sub.filter != nil {
			// Protect against panics in user-provided filters
			func() {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("[manager] Subscription filter panic", "panic", r)
						allowed = false
					}
				}()
				allowed = sub.filter(msg)
			}()
		} else if len(sub.allowedTypes) > 0 {
			_, ok := sub.allowedTypes[types.SIMCONNECT_RECV_ID(msg.DwID)]
			allowed = ok
		}

		if !allowed {
			continue
		}

		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- msg:
			default:
				// Channel full, skip message to avoid blocking
				if sub.onDrop != nil {
					// Protect dispatch loop from user callback panics
					func() {
						defer func() {
							if r := recover(); r != nil {
								m.logger.Error("[manager] OnDrop callback panicked", "panic", r)
							}
						}()
						sub.onDrop(1)
					}()
				}
				m.logger.Debug("[manager] Subscription channel full, dropping message")
			}
		}
		sub.closeMu.Unlock()
	}
}
