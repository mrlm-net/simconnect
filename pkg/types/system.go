//go:build windows
// +build windows

package types

// HeartbeatFrequency represents the valid heartbeat frequencies for SimConnect system events.
type HeartbeatFrequency string

const (
	Heartbeat6Hz   HeartbeatFrequency = "6Hz"
	Heartbeat1sec  HeartbeatFrequency = "1sec"
	Heartbeat4sec  HeartbeatFrequency = "4sec"
	HeartbeatFrame HeartbeatFrequency = "Frame"
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_STATE.htm
type DWORD uint32

type SIMCONNECT_STATE DWORD

const (
	SIMCONNECT_STATE_OFF SIMCONNECT_STATE = iota
	SIMCONNECT_STATE_ON
)

// SIMCONNECT_SYSTEM_STATE represents the system state constants used in SimConnect requests.
type SIMCONNECT_SYSTEM_STATE string

// System state constants as defined in the SimConnect documentation
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
const (
	SIMCONNECT_SYSTEM_STATE_AIRCRAFT_LOADED SIMCONNECT_SYSTEM_STATE = "AircraftLoaded" // Full path name of the last loaded aircraft flight dynamics file (.AIR)
	SIMCONNECT_SYSTEM_STATE_DIALOG_MODE     SIMCONNECT_SYSTEM_STATE = "DialogMode"     // Whether the simulation is in Dialog mode or not
	SIMCONNECT_SYSTEM_STATE_FLIGHT_LOADED   SIMCONNECT_SYSTEM_STATE = "FlightLoaded"   // Full path name of the last loaded flight (.FLT)
	SIMCONNECT_SYSTEM_STATE_FLIGHT_PLAN     SIMCONNECT_SYSTEM_STATE = "FlightPlan"     // Full path name of the active flight plan (empty if none)
	SIMCONNECT_SYSTEM_STATE_SIM             SIMCONNECT_SYSTEM_STATE = "Sim"            // State of the simulation (1 = user in control, 0 = navigating UI)
)
