//go:build windows
// +build windows

package types

// System state constants as defined in the SimConnect documentation
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/General/SimConnect_RequestSystemState.htm
const (
	SIMCONNECT_SYSTEM_STATE_AIRCRAFT_LOADED = "AircraftLoaded" // Full path name of the last loaded aircraft flight dynamics file (.AIR)
	SIMCONNECT_SYSTEM_STATE_DIALOG_MODE     = "DialogMode"     // Whether the simulation is in Dialog mode or not
	SIMCONNECT_SYSTEM_STATE_FLIGHT_LOADED   = "FlightLoaded"   // Full path name of the last loaded flight (.FLT)
	SIMCONNECT_SYSTEM_STATE_FLIGHT_PLAN     = "FlightPlan"     // Full path name of the active flight plan (empty if none)
	SIMCONNECT_SYSTEM_STATE_SIM             = "Sim"            // State of the simulation (1 = user in control, 0 = navigating UI)
)
