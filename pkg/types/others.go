//go:build windows
// +build windows

package types

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_ICAO.htm
type SIMCONNECT_ICAO struct {
	Type    byte
	Ident   [9]byte // 8 + 1 for null terminator
	Region  [3]byte // 2 + 1 for null terminator
	Airport [5]byte
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_JETWAY_DATA.htm
type SIMCONNECT_JETWAY_DATA struct {
	SIMCONNECT_RECV
	AirportIcao         [8]byte // [8]byte
	ParkingIndex        int     // DWORD
	LLA                 SIMCONNECT_DATA_LATLONALT
	PBH                 SIMCONNECT_DATA_PBH
	Status              int
	Door                int
	ExitDoorRelativePos SIMCONNECT_DATA_XYZ
	MainHandlePos       SIMCONNECT_DATA_XYZ
	SecondaryHandle     SIMCONNECT_DATA_XYZ
	WheelGroundLock     SIMCONNECT_DATA_XYZ
	JetwayObjectId      DWORD
	AttachedObjectId    DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_VERSION_BASE_TYPE.htm
type SIMCONNECT_VERSION_BASE_TYPE struct {
	Major    uint16 // DWORD Major;
	Minor    uint16 // DWORD Minor;
	Revision uint16 // DWORD Patch;
	Build    uint16 // DWORD Build;
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_WAYPOINT_FLAGS.htm
type SIMCONNECT_WAYPOINT_FLAGS DWORD

const (
	SIMCONNECT_WAYPOINT_SPEED_REQUESTED        = 0x04
	SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED     = 0x08
	SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED = 0x10
	SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL        = 0x20
	SIMCONNECT_WAYPOINT_ON_GROUND              = 0x00100000
	SIMCONNECT_WAYPOINT_REVERSE                = 0x00200000
	SIMCONNECT_WAYPOINT_WRAP_TO_FIRST          = 0x00400000
)
