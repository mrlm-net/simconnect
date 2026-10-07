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
	SIMCONNECT_FACILITY_DATA_VDGS
	SIMCONNECT_FACILITY_DATA_HOLDING_PATTERN
	SIMCONNECT_FACILITY_DATA_TAXI_PARKING_AIRLINE
)

// SIMCONNECT_DATA_FACILITY_AIRPORT is one airport of a SIMCONNECT_RECV_AIRPORT_LIST.
//
// SimConnect.h (MSFS 2024, packed): char Ident[9], char Region[3], double
// Latitude, Longitude, Altitude — FacilityAirportSize (36) bytes on the wire.
// Go pads the doubles to 8 bytes, so this struct is not the wire layout: read
// list entries with SIMCONNECT_RECV_AIRPORT_LIST.Entries or
// DecodeFacilityAirport, never by casting.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_AIRPORT.htm
type SIMCONNECT_DATA_FACILITY_AIRPORT struct {
	Ident     [9]byte // char Ident[9]
	Region    [3]byte // char Region[3]
	Latitude  float64 // double Latitude, degrees
	Longitude float64 // double Longitude, degrees
	Altitude  float64 // double Altitude, meters
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_MINIMAL.htm
//
// Packed on the wire: SIMCONNECT_ICAO (18) + SIMCONNECT_DATA_LATLONALT (24) =
// FacilityMinimalSize (42) bytes; the Go struct is 48. Read list entries with
// SIMCONNECT_RECV_FACILITY_MINIMAL_LIST.Entries or DecodeFacilityMinimal.
type SIMCONNECT_FACILITY_MINIMAL struct {
	ICAO SIMCONNECT_ICAO
	LLA  SIMCONNECT_DATA_LATLONALT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_NDB.htm
//
// Packed on the wire: waypoint (40) + DWORD fFrequency = FacilityNDBSize (44) bytes.
type SIMCONNECT_DATA_FACILITY_NDB struct {
	SIMCONNECT_DATA_FACILITY_WAYPOINT
	FFrequency DWORD // DWORD fFrequency, Hz
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_WAYPOINT.htm
//
// Packed on the wire: airport (36) + float fMagVar = FacilityWaypointSize (40) bytes.
type SIMCONNECT_DATA_FACILITY_WAYPOINT struct {
	SIMCONNECT_DATA_FACILITY_AIRPORT
	FMagVar float32 // float fMagVar, degrees
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_VOR.htm
//
// Packed on the wire: NDB (44) + DWORD Flags (44) + float fLocalizer (48) +
// double GlideLat, GlideLon, GlideAlt (52, 60, 68) + float fGlideSlopeAngle
// (76) = FacilityVORSize (80) bytes. The Go struct is padded: read list entries
// with SIMCONNECT_RECV_VOR_LIST.Entries or DecodeFacilityVOR.
type SIMCONNECT_DATA_FACILITY_VOR struct {
	SIMCONNECT_DATA_FACILITY_NDB
	Flags            DWORD   // DWORD Flags, SIMCONNECT_VOR_FLAGS
	FLocalizer       float32 // float fLocalizer, degrees
	GlideLat         float64 // double GlideLat
	GlideLon         float64 // double GlideLon
	GlideAlt         float64 // double GlideAlt
	FGlideSlopeAngle float32 // float fGlideSlopeAngle, degrees
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
