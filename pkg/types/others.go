//go:build windows
// +build windows

package types

import (
	"encoding/binary"
	"math"
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_ICAO.htm
type SIMCONNECT_ICAO struct {
	Type    byte
	Ident   [9]byte // 8 + 1 for null terminator
	Region  [3]byte // 2 + 1 for null terminator
	Airport [5]byte
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_JETWAY_DATA.htm
// It is a jetway as SimConnect reports it; on the wire each list entry is
// packed, JetwayDataSize bytes (Pbh as three floats): read them with
// DecodeJetwayData, not by casting (a Go struct of these types is laid out
// differently).
type SIMCONNECT_JETWAY_DATA struct {
	AirportIcao         [8]byte
	ParkingIndex        int32 // int
	LLA                 SIMCONNECT_DATA_LATLONALT
	PBH                 SIMCONNECT_DATA_PBH // floats on the wire
	Status              uint32              // JETWAY_STATUS_*: 0 rest, 1 approach outside, 2 approach door, 3 hood connect, 4 hood disconnect, 5 retract outside, 6 retract home, 7 fully attached
	Door                uint32
	ExitDoorRelativePos SIMCONNECT_DATA_XYZ
	MainHandlePos       SIMCONNECT_DATA_XYZ
	SecondaryHandle     SIMCONNECT_DATA_XYZ
	WheelGroundLock     SIMCONNECT_DATA_XYZ
	JetwayObjectId      DWORD
	AttachedObjectId    DWORD
}

// JetwayDataSize is the size of one SIMCONNECT_JETWAY_DATA on the wire.
const JetwayDataSize = 160

// DecodeJetwayData reads one packed SIMCONNECT_JETWAY_DATA entry from b
// (at least JetwayDataSize bytes); false when b is too short.
func DecodeJetwayData(b []byte) (SIMCONNECT_JETWAY_DATA, bool) {
	var j SIMCONNECT_JETWAY_DATA
	if len(b) < JetwayDataSize {
		return j, false
	}
	le := binary.LittleEndian
	f64 := func(o int) float64 { return math.Float64frombits(le.Uint64(b[o:])) }
	f32 := func(o int) float64 { return float64(math.Float32frombits(le.Uint32(b[o:]))) }
	xyz := func(o int) SIMCONNECT_DATA_XYZ { return SIMCONNECT_DATA_XYZ{X: f64(o), Y: f64(o + 8), Z: f64(o + 16)} }
	copy(j.AirportIcao[:], b[0:8])
	j.ParkingIndex = int32(le.Uint32(b[8:]))
	j.LLA = SIMCONNECT_DATA_LATLONALT{Latitude: f64(12), Longitude: f64(20), Altitude: f64(28)}
	j.PBH = SIMCONNECT_DATA_PBH{Pitch: f32(36), Bank: f32(40), Heading: f32(44)}
	j.Status, j.Door = le.Uint32(b[48:]), le.Uint32(b[52:])
	j.ExitDoorRelativePos, j.MainHandlePos, j.SecondaryHandle, j.WheelGroundLock = xyz(56), xyz(80), xyz(104), xyz(128)
	j.JetwayObjectId, j.AttachedObjectId = DWORD(le.Uint32(b[152:])), DWORD(le.Uint32(b[156:]))
	return j, true
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
