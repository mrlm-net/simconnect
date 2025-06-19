//go:build windows
// +build windows

package types

import "syscall"

type SIMCONNECT_DATATYPE uint32

const (
	SIMCONNECT_DATATYPE_INVALID SIMCONNECT_DATATYPE = iota
	SIMCONNECT_DATATYPE_INT32
	SIMCONNECT_DATATYPE_INT64
	SIMCONNECT_DATATYPE_FLOAT32
	SIMCONNECT_DATATYPE_FLOAT64
	SIMCONNECT_DATATYPE_STRING8
	SIMCONNECT_DATATYPE_STRING32
	SIMCONNECT_DATATYPE_STRING64
	SIMCONNECT_DATATYPE_STRING128
	SIMCONNECT_DATATYPE_STRING256
	SIMCONNECT_DATATYPE_STRING260
	SIMCONNECT_DATATYPE_STRINGV
	SIMCONNECT_DATATYPE_INITPOSITION
	SIMCONNECT_DATATYPE_MARKERSTATE
	SIMCONNECT_DATATYPE_WAYPOINT
	SIMCONNECT_DATATYPE_LATLONALT
	SIMCONNECT_DATATYPE_XYZ
)

// SIMCONNECT_DATA_INITPOSITION is used to initialize the position of the user aircraft, AI controlled aircraft, or other simulation object.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_INITPOSITION.htm
type SIMCONNECT_DATA_INITPOSITION struct {
	Latitude  float64 // Latitude in degrees
	Longitude float64 // Longitude in degrees
	Altitude  float64 // Altitude in feet
	Pitch     float64 // Pitch in degrees
	Bank      float64 // Bank in degrees
	Heading   float64 // Heading in degrees
	OnGround  uint32  // Set this to 1 to place the object on the ground, or 0 if the object is to be airborne
	Airspeed  uint32  // The airspeed in knots, or special values: INITPOSITION_AIRSPEED_CRUISE (-1), INITPOSITION_AIRSPEED_KEEP (-2)
}

// SIMCONNECT_DATA_LATLONALT is used to hold a world position.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_LATLONALT.htm
type SIMCONNECT_DATA_LATLONALT struct {
	Latitude  float64 // Latitude in degrees
	Longitude float64 // Longitude in degrees
	Altitude  float64 // Altitude in feet
}

// SIMCONNECT_DATA_MARKERSTATE is used to help graphically link flight model data with the graphics model.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_MARKERSTATE.htm
type SIMCONNECT_DATA_MARKERSTATE struct {
	SzMarkerName  [64]byte // Null-terminated string containing the marker name (char szMarkerName[64] in C)
	DwMarkerState uint32   // Marker state, set to 1 for on and 0 for off (DWORD dwMarkerState in C)
}

// SIMCONNECT_DATA_PBH is used to hold a world orientation.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_PBH.htm
type SIMCONNECT_DATA_PBH struct {
	Pitch   float32 // The pitch in degrees
	Bank    float32 // The bank in degrees
	Heading float32 // The heading in degrees
}

// SIMCONNECT_DATA_RACE_RESULT is used to hold multiplayer racing results.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_RACE_RESULT.htm
type SIMCONNECT_DATA_RACE_RESULT struct {
	DwNumberOfRacers uint32       // The total number of racers
	MissionGUID      syscall.GUID // The GUID of the mission that has been selected by the host
	SzPlayerName     [260]byte    // Null terminated string containing the name of the player (MAX_PATH)
	SzSessionType    [260]byte    // Null terminated string containing the type of the multiplayer session
	SzAircraft       [260]byte    // Null terminated string containing the aircraft type
	SzPlayerRole     [260]byte    // Null terminated string containing the player's role in the mission
	FTotalTime       float64      // Final race time in seconds, or lap time, or 0 for DNF
	FPenaltyTime     float64      // Final penalty time in seconds or total penalty time so far
	DwIsDisqualified uint32       // Boolean value, 0 = not disqualified, non-zero = disqualified
}

// SIMCONNECT_DATA_WAYPOINT is used to define a single waypoint.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_WAYPOINT.htm
type SIMCONNECT_DATA_WAYPOINT struct {
	Latitude        float64 // Latitude in degrees
	Longitude       float64 // Longitude in degrees
	Altitude        float64 // Altitude in feet
	Flags           uint32  // Specifies the flags set for this waypoint, see SIMCONNECT_WAYPOINT_FLAGS (unsigned long in C)
	KtsSpeed        float64 // Required speed in knots (ktsSpeed in C - if SIMCONNECT_WAYPOINT_SPEED_REQUESTED flag is set)
	PercentThrottle float64 // Required throttle as a percentage (percentThrottle in C - if SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED flag is set)
}

// SIMCONNECT_DATA_XYZ is used to hold a 3D co-ordinate.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_XYZ.htm
type SIMCONNECT_DATA_XYZ struct {
	X float64 // The position along the x axis
	Y float64 // The position along the y axis
	Z float64 // The position along the z axis
}

type SIMCONNECT_DATA_REQUEST_FLAG uint32

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObject.htm
const (
	SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT SIMCONNECT_DATA_REQUEST_FLAG = iota
	SIMCONNECT_DATA_REQUEST_FLAG_CHANGED
	SIMCONNECT_DATA_REQUEST_FLAG_TAGGED
)

type SIMCONNECT_DATA_SET_FLAG uint32

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetDataOnSimObject.htm
const (
	SIMCONNECT_DATA_SET_FLAG_DEFAULT SIMCONNECT_DATA_SET_FLAG = 0
)
