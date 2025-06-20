//go:build windows
// +build windows

package types

// SIMCONNECT_SYSTEM_STATE represents the system state constants used in SimConnect requests.
type SIMCONNECT_SYSTEM_STATE string

// System state constants as defined in the SimConnect documentation
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
const (
	SIMCONNECT_SYSTEM_STATE_AIRCRAFT_LOADED SIMCONNECT_SYSTEM_STATE = "AircraftLoaded" // Full path name of the last loaded aircraft flight dynamics file (.AIR)
	SIMCONNECT_SYSTEM_STATE_DIALOG_MODE     SIMCONNECT_SYSTEM_STATE = "DialogMode"     // Whether the simulation is in Dialog mode or not
	SIMCONNECT_SYSTEM_STATE_FLIGHT_LOADED   SIMCONNECT_SYSTEM_STATE = "FlightLoaded"   // Full path name of the last loaded flight (.FLT)
	SIMCONNECT_SYSTEM_STATE_FLIGHT_PLAN     SIMCONNECT_SYSTEM_STATE = "FlightPlan"     // Full path name of the active flight plan (empty if none)
	SIMCONNECT_SYSTEM_STATE_SIM             SIMCONNECT_SYSTEM_STATE = "Sim"            // State of the simulation (1 = user in control, 0 = navigating UI)
)

type SIMCONNECT_SYSTEM_EVENT string

// System event constants as defined in the SimConnect documentation
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SubscribeToSystemEvent.htm
const (
	// Timing events
	SIMCONNECT_SYSTEM_EVENT_1_SECOND SIMCONNECT_SYSTEM_EVENT = "1sec" // Request a notification every second
	SIMCONNECT_SYSTEM_EVENT_4_SECOND SIMCONNECT_SYSTEM_EVENT = "4sec" // Request a notification every four seconds
	SIMCONNECT_SYSTEM_EVENT_6_HZ     SIMCONNECT_SYSTEM_EVENT = "6Hz"  // Request notifications six times per second (same rate as joystick movement events)

	// Aircraft and flight events
	SIMCONNECT_SYSTEM_EVENT_AIRCRAFT_LOADED SIMCONNECT_SYSTEM_EVENT = "AircraftLoaded" // Notification when aircraft flight dynamics file (.AIR) is changed
	SIMCONNECT_SYSTEM_EVENT_FLIGHT_LOADED   SIMCONNECT_SYSTEM_EVENT = "FlightLoaded"   // Notification when a flight is loaded
	SIMCONNECT_SYSTEM_EVENT_FLIGHT_SAVED    SIMCONNECT_SYSTEM_EVENT = "FlightSaved"    // Notification when a flight is saved correctly

	// Flight plan events
	SIMCONNECT_SYSTEM_EVENT_FLIGHT_PLAN_ACTIVATED   SIMCONNECT_SYSTEM_EVENT = "FlightPlanActivated"   // Notification when a new flight plan is activated
	SIMCONNECT_SYSTEM_EVENT_FLIGHT_PLAN_DEACTIVATED SIMCONNECT_SYSTEM_EVENT = "FlightPlanDeactivated" // Notification when the active flight plan is de-activated

	// Crash events
	SIMCONNECT_SYSTEM_EVENT_CRASHED     SIMCONNECT_SYSTEM_EVENT = "Crashed"    // Notification if the user aircraft crashes
	SIMCONNECT_SYSTEM_EVENT_CRASH_RESET SIMCONNECT_SYSTEM_EVENT = "CrashReset" // Notification when the crash cut-scene has completed

	// Frame events
	SIMCONNECT_SYSTEM_EVENT_FRAME       SIMCONNECT_SYSTEM_EVENT = "Frame"      // Notifications every visual frame
	SIMCONNECT_SYSTEM_EVENT_PAUSE_FRAME SIMCONNECT_SYSTEM_EVENT = "PauseFrame" // Notifications for every visual frame that the simulation is paused

	// Object events
	SIMCONNECT_SYSTEM_EVENT_OBJECT_ADDED   SIMCONNECT_SYSTEM_EVENT = "ObjectAdded"   // Notification when an AI object is added to the simulation
	SIMCONNECT_SYSTEM_EVENT_OBJECT_REMOVED SIMCONNECT_SYSTEM_EVENT = "ObjectRemoved" // Notification when an AI object is removed from the simulation

	// Pause events
	SIMCONNECT_SYSTEM_EVENT_PAUSE    SIMCONNECT_SYSTEM_EVENT = "Pause"     // Notifications when flight is paused/unpaused, returns current pause state
	SIMCONNECT_SYSTEM_EVENT_PAUSE_EX SIMCONNECT_SYSTEM_EVENT = "Pause_EX1" // Detailed pause notifications with extended state information
	SIMCONNECT_SYSTEM_EVENT_PAUSED   SIMCONNECT_SYSTEM_EVENT = "Paused"    // Notification when the flight is paused
	SIMCONNECT_SYSTEM_EVENT_UNPAUSED SIMCONNECT_SYSTEM_EVENT = "Unpaused"  // Notification when the flight is un-paused

	// Position events
	SIMCONNECT_SYSTEM_EVENT_POSITION_CHANGED SIMCONNECT_SYSTEM_EVENT = "PositionChanged" // Notification when user changes aircraft position through dialog

	// Simulation state events
	SIMCONNECT_SYSTEM_EVENT_SIM       SIMCONNECT_SYSTEM_EVENT = "Sim"      // Notifications when flight is running/not running, returns current state
	SIMCONNECT_SYSTEM_EVENT_SIM_START SIMCONNECT_SYSTEM_EVENT = "SimStart" // The simulator is running (user actively controlling aircraft)
	SIMCONNECT_SYSTEM_EVENT_SIM_STOP  SIMCONNECT_SYSTEM_EVENT = "SimStop"  // The simulator is not running (loading flight, navigating UI, in dialog)

	// Sound events
	SIMCONNECT_SYSTEM_EVENT_SOUND SIMCONNECT_SYSTEM_EVENT = "Sound" // Notification when master sound switch is changed

	// View events
	SIMCONNECT_SYSTEM_EVENT_VIEW SIMCONNECT_SYSTEM_EVENT = "View" // Notification when user aircraft view is changed

	// Deprecated events (kept for backwards compatibility)
	SIMCONNECT_SYSTEM_EVENT_CUSTOM_MISSION_ACTION_EXECUTED SIMCONNECT_SYSTEM_EVENT = "CustomMissionActionExecutedDeprecated" // Deprecated: Notification when mission action executed
	SIMCONNECT_SYSTEM_EVENT_WEATHER_MODE_CHANGED           SIMCONNECT_SYSTEM_EVENT = "WeatherModeChangedDeprecated"          // Deprecated: Notification when weather mode changed
)

// Pause state flags for PAUSE_EX1 system event
const (
	PAUSE_STATE_FLAG_OFF              = 0 // No Pause
	PAUSE_STATE_FLAG_PAUSE            = 1 // "full" Pause (sim + traffic + etc...)
	PAUSE_STATE_FLAG_PAUSE_WITH_SOUND = 2 // FSX Legacy Pause (not used anymore)
	PAUSE_STATE_FLAG_ACTIVE_PAUSE     = 4 // Pause was activated using the "Active Pause" Button
	PAUSE_STATE_FLAG_SIM_PAUSE        = 8 // Pause the player sim but traffic, multi, etc... will still run
)

// Sound system event data flags
const (
	SIMCONNECT_SOUND_SYSTEM_EVENT_DATA_MASTER = 0x1 // Master sound switch is on
)

// View system event data flags
const (
	SIMCONNECT_VIEW_SYSTEM_EVENT_DATA_COCKPIT_2D      = iota // 2D cockpit view
	SIMCONNECT_VIEW_SYSTEM_EVENT_DATA_COCKPIT_VIRTUAL        // Virtual cockpit view
	SIMCONNECT_VIEW_SYSTEM_EVENT_DATA_ORTHOGONAL             // Orthogonal (map) view
)
