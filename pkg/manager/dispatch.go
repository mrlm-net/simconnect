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
			// Set initial SimState
			m.setSimState(defaultSimState())

			// Subscribe to pause events
			// Register manager ID for tracking, but subscribe with actual SimConnect event ID 1000
			m.requestRegistry.Register(m.pauseEventID, RequestTypeEvent, "Pause Event Subscription")
			if err := client.SubscribeToSystemEvent(m.pauseEventID, "Pause"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Pause event", "error", err)
			}

			// Subscribe to sim events
			// Register manager ID for tracking, but subscribe with actual SimConnect event ID 1001
			m.requestRegistry.Register(m.simEventID, RequestTypeEvent, "Sim Event Subscription")
			if err := client.SubscribeToSystemEvent(m.simEventID, "Sim"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Sim event", "error", err)
			}

			// Subscribe to additional system events
			m.requestRegistry.Register(m.flightLoadedEventID, RequestTypeEvent, "FlightLoaded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.flightLoadedEventID, "FlightLoaded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to FlightLoaded event", "error", err)
			}

			m.requestRegistry.Register(m.aircraftLoadedEventID, RequestTypeEvent, "AircraftLoaded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.aircraftLoadedEventID, "AircraftLoaded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to AircraftLoaded event", "error", err)
			}

			m.requestRegistry.Register(m.flightPlanActivatedEventID, RequestTypeEvent, "FlightPlanActivated Event Subscription")
			if err := client.SubscribeToSystemEvent(m.flightPlanActivatedEventID, "FlightPlanActivated"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to FlightPlanActivated event", "error", err)
			}

			m.requestRegistry.Register(m.objectAddedEventID, RequestTypeEvent, "ObjectAdded Event Subscription")
			if err := client.SubscribeToSystemEvent(m.objectAddedEventID, "ObjectAdded"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to ObjectAdded event", "error", err)
			}

			m.requestRegistry.Register(m.objectRemovedEventID, RequestTypeEvent, "ObjectRemoved Event Subscription")
			if err := client.SubscribeToSystemEvent(m.objectRemovedEventID, "ObjectRemoved"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to ObjectRemoved event", "error", err)
			}

			m.requestRegistry.Register(m.crashedEventID, RequestTypeEvent, "Crashed Event Subscription")
			if err := client.SubscribeToSystemEvent(m.crashedEventID, "Crashed"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Crashed event", "error", err)
			}

			m.requestRegistry.Register(m.crashResetEventID, RequestTypeEvent, "CrashReset Event Subscription")
			if err := client.SubscribeToSystemEvent(m.crashResetEventID, "CrashReset"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to CrashReset event", "error", err)
			}

			m.requestRegistry.Register(m.soundEventID, RequestTypeEvent, "Sound Event Subscription")
			if err := client.SubscribeToSystemEvent(m.soundEventID, "Sound"); err != nil {
				m.logger.Error("[manager] Failed to subscribe to Sound event", "error", err)
			}

			// Define camera data structure
			m.requestRegistry.Register(m.cameraDefinitionID, RequestTypeDataDefinition, "Simulator State Definition")
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0); err != nil {
				m.logger.Error("[manager] Failed to add CAMERA STATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1); err != nil {
				m.logger.Error("[manager] Failed to add CAMERA SUBSTATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIMULATION RATE", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2); err != nil {
				m.logger.Error("[manager] Failed to add SIMULATION RATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIMULATION TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3); err != nil {
				m.logger.Error("[manager] Failed to add SIMULATION TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU TIME", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5); err != nil {
				m.logger.Error("[manager] Failed to add ZULU TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS IN VR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 6); err != nil {
				m.logger.Error("[manager] Failed to add IS IN VR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS USING MOTION CONTROLLERS", "", types.SIMCONNECT_DATATYPE_INT32, 0, 7); err != nil {
				m.logger.Error("[manager] Failed to add IS USING MOTION CONTROLLERS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS USING JOYSTICK THROTTLE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 8); err != nil {
				m.logger.Error("[manager] Failed to add IS USING JOYSTICK THROTTLE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS IN RTC", "", types.SIMCONNECT_DATATYPE_INT32, 0, 9); err != nil {
				m.logger.Error("[manager] Failed to add IS IN RTC definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS AVATAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 10); err != nil {
				m.logger.Error("[manager] Failed to add IS AVATAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "IS AIRCRAFT", "", types.SIMCONNECT_DATATYPE_INT32, 0, 11); err != nil {
				m.logger.Error("[manager] Failed to add IS AIRCRAFT definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL DAY OF MONTH", "", types.SIMCONNECT_DATATYPE_INT32, 0, 12); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL DAY OF MONTH definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL MONTH OF YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 13); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL MONTH OF YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "LOCAL YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 14); err != nil {
				m.logger.Error("[manager] Failed to add LOCAL YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU DAY OF MONTH", "", types.SIMCONNECT_DATATYPE_INT32, 0, 15); err != nil {
				m.logger.Error("[manager] Failed to add ZULU DAY OF MONTH definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU MONTH OF YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 16); err != nil {
				m.logger.Error("[manager] Failed to add ZULU MONTH OF YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU YEAR", "", types.SIMCONNECT_DATATYPE_INT32, 0, 17); err != nil {
				m.logger.Error("[manager] Failed to add ZULU YEAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 18); err != nil {
				m.logger.Error("[manager] Failed to add REALISM definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "VISUAL MODEL RADIUS", "meters", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 19); err != nil {
				m.logger.Error("[manager] Failed to add VISUAL MODEL RADIUS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIM DISABLED", "", types.SIMCONNECT_DATATYPE_INT32, 0, 20); err != nil {
				m.logger.Error("[manager] Failed to add SIM DISABLED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM CRASH DETECTION", "", types.SIMCONNECT_DATATYPE_INT32, 0, 21); err != nil {
				m.logger.Error("[manager] Failed to add REALISM CRASH DETECTION definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "REALISM CRASH WITH OTHERS", "", types.SIMCONNECT_DATATYPE_INT32, 0, 22); err != nil {
				m.logger.Error("[manager] Failed to add REALISM CRASH WITH OTHERS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "TRACK IR ENABLE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 23); err != nil {
				m.logger.Error("[manager] Failed to add TRACK IR ENABLE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "USER INPUT ENABLED", "", types.SIMCONNECT_DATATYPE_INT32, 0, 24); err != nil {
				m.logger.Error("[manager] Failed to add USER INPUT ENABLED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SIM ON GROUND", "", types.SIMCONNECT_DATATYPE_INT32, 0, 25); err != nil {
				m.logger.Error("[manager] Failed to add SIM ON GROUND definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT TEMPERATURE", "Celsius", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 26); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT TEMPERATURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT PRESSURE", "inHg", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 27); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT WIND VELOCITY", "Knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 28); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT WIND VELOCITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT WIND DIRECTION", "Degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 29); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT WIND DIRECTION definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT VISIBILITY", "Meters", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 30); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT VISIBILITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT IN CLOUD", "", types.SIMCONNECT_DATATYPE_INT32, 0, 31); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT IN CLOUD definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT PRECIP STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 32); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT PRECIP STATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "BAROMETER PRESSURE", "Millibars", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 33); err != nil {
				m.logger.Error("[manager] Failed to add BAROMETER PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SEA LEVEL PRESSURE", "Millibars", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 34); err != nil {
				m.logger.Error("[manager] Failed to add SEA LEVEL PRESSURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "GROUND ALTITUDE", "Feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 35); err != nil {
				m.logger.Error("[manager] Failed to add GROUND ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "MAGVAR", "Degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 36); err != nil {
				m.logger.Error("[manager] Failed to add MAGVAR definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SURFACE TYPE", "Enum", types.SIMCONNECT_DATATYPE_INT32, 0, 37); err != nil {
				m.logger.Error("[manager] Failed to add SURFACE TYPE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 38); err != nil {
				m.logger.Error("[manager] Failed to add PLANE LATITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 39); err != nil {
				m.logger.Error("[manager] Failed to add PLANE LONGITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 40); err != nil {
				m.logger.Error("[manager] Failed to add PLANE ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "INDICATED ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 41); err != nil {
				m.logger.Error("[manager] Failed to add INDICATED ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 42); err != nil {
				m.logger.Error("[manager] Failed to add PLANE HEADING DEGREES TRUE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 43); err != nil {
				m.logger.Error("[manager] Failed to add PLANE HEADING DEGREES MAGNETIC definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 44); err != nil {
				m.logger.Error("[manager] Failed to add PLANE PITCH DEGREES definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 45); err != nil {
				m.logger.Error("[manager] Failed to add PLANE BANK DEGREES definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 46); err != nil {
				m.logger.Error("[manager] Failed to add GROUND VELOCITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 47); err != nil {
				m.logger.Error("[manager] Failed to add AIRSPEED INDICATED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 48); err != nil {
				m.logger.Error("[manager] Failed to add AIRSPEED TRUE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 49); err != nil {
				m.logger.Error("[manager] Failed to add VERTICAL SPEED definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SMART CAMERA ACTIVE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 50); err != nil {
				m.logger.Error("[manager] Failed to add SMART CAMERA ACTIVE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "HAND ANIM STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 51); err != nil {
				m.logger.Error("[manager] Failed to add HAND ANIM STATE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "HIDE AVATAR IN AIRCRAFT", "", types.SIMCONNECT_DATATYPE_INT32, 0, 52); err != nil {
				m.logger.Error("[manager] Failed to add HIDE AVATAR IN AIRCRAFT definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "MISSION SCORE", "", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 53); err != nil {
				m.logger.Error("[manager] Failed to add MISSION SCORE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "PARACHUTE OPEN", "", types.SIMCONNECT_DATATYPE_INT32, 0, 54); err != nil {
				m.logger.Error("[manager] Failed to add PARACHUTE OPEN definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU SUNRISE TIME", "Seconds", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 55); err != nil {
				m.logger.Error("[manager] Failed to add ZULU SUNRISE TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ZULU SUNSET TIME", "Seconds", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 56); err != nil {
				m.logger.Error("[manager] Failed to add ZULU SUNSET TIME definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "TIME ZONE OFFSET", "Seconds", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 57); err != nil {
				m.logger.Error("[manager] Failed to add TIME ZONE OFFSET definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "TOOLTIP UNITS", "", types.SIMCONNECT_DATATYPE_INT32, 0, 58); err != nil {
				m.logger.Error("[manager] Failed to add TOOLTIP UNITS definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "UNITS OF MEASURE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 59); err != nil {
				m.logger.Error("[manager] Failed to add UNITS OF MEASURE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "AMBIENT IN SMOKE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 60); err != nil {
				m.logger.Error("[manager] Failed to add AMBIENT IN SMOKE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ENV SMOKE DENSITY", "Percent Over 100", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 61); err != nil {
				m.logger.Error("[manager] Failed to add ENV SMOKE DENSITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "ENV CLOUD DENSITY", "Percent Over 100", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 62); err != nil {
				m.logger.Error("[manager] Failed to add ENV CLOUD DENSITY definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "DENSITY ALTITUDE", "Feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 63); err != nil {
				m.logger.Error("[manager] Failed to add DENSITY ALTITUDE definition", "error", err)
			}
			if err := client.AddToDataDefinition(m.cameraDefinitionID, "SEA LEVEL AMBIENT TEMPERATURE", "Celsius", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 64); err != nil {
				m.logger.Error("[manager] Failed to add SEA LEVEL AMBIENT TEMPERATURE definition", "error", err)
			}

			// Request camera data with period matching heartbeat configuration
			period := types.SIMCONNECT_PERIOD_SIM_FRAME
			m.requestRegistry.Register(m.cameraRequestID, RequestTypeDataRequest, "Simulator State Data Request")
			if err := client.RequestDataOnSimObject(m.cameraRequestID, m.cameraDefinitionID, types.SIMCONNECT_OBJECT_ID_USER, period, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
				m.logger.Error("[manager] Failed to request camera data", "error", err)
			} else {
				m.mu.Lock()
				m.cameraDataRequestPending = true
				m.mu.Unlock()
				m.logger.Debug("[manager] Camera data request submitted")
			}
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
				m.mu.Unlock()
				m.notifySimStateChange(oldState, newState)
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
				m.mu.Unlock()
				m.notifySimStateChange(oldState, newState)
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
