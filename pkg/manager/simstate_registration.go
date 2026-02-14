//go:build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// registerSimStateSubscriptions subscribes to system events and defines simulator state data.
// This is called once when the SimConnect OPEN message is received.
func (m *Instance) registerSimStateSubscriptions(client engine.Client) {
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

	// Request camera data with period from configuration
	period := m.config.SimStatePeriod
	if period == types.SIMCONNECT_PERIOD_NEVER {
		m.logger.Warn("[manager] SimStatePeriod set to NEVER — SimState tracking disabled, change notifications will not fire")
		return
	}
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
