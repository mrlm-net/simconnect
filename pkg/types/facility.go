//go:build windows
// +build windows

package types

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_DATA_TYPE.htm
type SIMCONNECT_FACILITY_DATA_TYPE DWORD

const (
	SIMCONNECT_FACILITY_DATA_AIRPORT SIMCONNECT_FACILITY_DATA_TYPE = iota
	SIMCONNECT_FACILITY_DATA_RUNWAY
	SIMCONNECT_FACILITY_DATA_START
	SIMCONNECT_FACILITY_DATA_FREQUENCY
	SIMCONNECT_FACILITY_DATA_HELIPAD
	SIMCONNECT_FACILITY_DATA_APPROACH
	SIMCONNECT_FACILITY_DATA_APPROACH_TRANSITION
	SIMCONNECT_FACILITY_DATA_APPROACH_LEG
	SIMCONNECT_FACILITY_DATA_FINAL_APPROACH_LEG
	SIMCONNECT_FACILITY_DATA_MISSED_APPROACH_LEG
	SIMCONNECT_FACILITY_DATA_DEPARTURE
	SIMCONNECT_FACILITY_DATA_ARRIVAL
	SIMCONNECT_FACILITY_DATA_RUNWAY_TRANSITION
	SIMCONNECT_FACILITY_DATA_ENROUTE_TRANSITION
	SIMCONNECT_FACILITY_DATA_TAXI_POINT
	SIMCONNECT_FACILITY_DATA_TAXI_PARKING
	SIMCONNECT_FACILITY_DATA_TAXI_PATH
	SIMCONNECT_FACILITY_DATA_TAXI_NAME
	SIMCONNECT_FACILITY_DATA_JETWAY
	SIMCONNECT_FACILITY_DATA_VOR
	SIMCONNECT_FACILITY_DATA_NDB
	SIMCONNECT_FACILITY_DATA_WAYPOINT
	SIMCONNECT_FACILITY_DATA_ROUTE
	SIMCONNECT_FACILITY_DATA_PAVEMENT
	SIMCONNECT_FACILITY_DATA_APPROACH_LIGHTS
	SIMCONNECT_FACILITY_DATA_VASI
)

// SIMCONNECT_DATA_FACILITY_AIRPORT represents basic airport facility data.
// C SDK field order: char Ident[6], char Region[3], double Latitude, double Longitude, double Altitude.
// Go aligns the float64 fields to 8 bytes: unsafe.Sizeof = 40 on amd64.
//
// Note: MSFS 2024 SDK uses char Ident[9] (not char Ident[6]), giving a wire layout
// of ident[9] + region[3] + 3×float64 = 36 bytes. Multi-entry AIRPORT_LIST messages
// from RequestFacilitiesListEX1 report a 41-byte stride (36 data + 5 trailing bytes).
// Do not cast this struct directly from a multi-entry SimConnect buffer;
// use runtime stride arithmetic with offset detection instead.
// See examples/read-facilities for the reference implementation.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_AIRPORT.htm
type SIMCONNECT_DATA_FACILITY_AIRPORT struct {
	Ident     [6]byte // char ident[6]
	Region    [3]byte // char region[3]
	Latitude  float64 // double Latitude
	Longitude float64 // double Longitude
	Altitude  float64 // double Altitude
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_MINIMAL.htm
type SIMCONNECT_FACILITY_MINIMAL struct {
	ICAO SIMCONNECT_ICAO
	LLA  SIMCONNECT_DATA_LATLONALT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_NDB.htm
type SIMCONNECT_DATA_FACILITY_NDB struct {
	SIMCONNECT_DATA_FACILITY_WAYPOINT
	FFrequency DWORD // DWORD fFrequency
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_WAYPOINT.htm
type SIMCONNECT_DATA_FACILITY_WAYPOINT struct {
	SIMCONNECT_DATA_FACILITY_AIRPORT
	FMagVar float64 // double fMagVar
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_VOR.htm
//
// WARNING — compound struct misalignment: Do NOT cast this struct directly from
// a multi-entry SimConnect RECV_VOR_LIST buffer. Two independent misalignments
// accumulate here:
//
//  1. Airport base misalignment (documented on SIMCONNECT_DATA_FACILITY_AIRPORT):
//     MSFS 2020 wire: ident[6]+region[3] = 9 bytes before Latitude → Go pads 7 bytes (Go offset 16, wire 9).
//     MSFS 2024 wire: ident[9]+region[3] = 12 bytes before Latitude → Go pads 4 bytes (Go offset 16, wire 12).
//
//  2. VOR-internal misalignment: Flags (DWORD) immediately precedes FLocalizer (float64).
//     NDB Go sizeof = 56; Flags at Go offset 56, Flags+4 = 60; 60 % 8 = 4 → Go pads 4 more bytes.
//     FLocalizer Go offset = 64.
//     Wire (MSFS 2024): NDB wire size = 48, Flags at wire 48, FLocalizer at wire 52.
//     Total discrepancy at FLocalizer: Go 64 vs wire 52 = 12 bytes (MSFS 2024) or 15 bytes (MSFS 2020).
//
// Use runtime stride arithmetic identical to the AIRPORT pattern (see examples/read-facilities)
// to read VOR entries. Direct struct indexing via RgData[i] will produce garbage for all
// fields from FLocalizer onward.
type SIMCONNECT_DATA_FACILITY_VOR struct {
	SIMCONNECT_DATA_FACILITY_NDB
	Flags            DWORD
	FLocalizer       float64 // double FLocalizer — WARNING: Go offset 64, wire offset 52 (MSFS 2024); see struct comment
	GlideLat         float64 // double GlideLat
	GlideLon         float64 // double GlideLon
	GlideAlt         float64 // double GlideAlt
	FGlideSlopeAngle float64 // double FGlideSlopeAngle
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_LIST_TYPE.htm
type SIMCONNECT_FACILITY_LIST_TYPE uint32

const (
	SIMCONNECT_FACILITY_LIST_AIRPORT SIMCONNECT_FACILITY_LIST_TYPE = iota
	SIMCONNECT_FACILITY_LIST_WAYPOINT
	SIMCONNECT_FACILITY_LIST_TYPE_NDB
	SIMCONNECT_FACILITY_LIST_TYPE_VOR
	SIMCONNECT_FACILITY_LIST_TYPE_COUNT
)
