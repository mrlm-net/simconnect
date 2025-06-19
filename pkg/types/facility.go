//go:build windows
// +build windows

package types

// SIMCONNECT_FACILITY_DATA_TYPE is used to specify the type of facility data being requested.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_DATA_TYPE.htm
type SIMCONNECT_FACILITY_DATA_TYPE uint32

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

// SIMCONNECT_FACILITY_LIST_TYPE is used to specify the type of facility list to request.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_LIST_TYPE.htm
type SIMCONNECT_FACILITY_LIST_TYPE uint32

const (
	SIMCONNECT_FACILITY_LIST_TYPE_AIRPORT SIMCONNECT_FACILITY_LIST_TYPE = iota
	SIMCONNECT_FACILITY_LIST_TYPE_WAYPOINT
	SIMCONNECT_FACILITY_LIST_TYPE_NDB
	SIMCONNECT_FACILITY_LIST_TYPE_VOR
	SIMCONNECT_FACILITY_LIST_TYPE_COUNT
)

// SIMCONNECT_DATA_FACILITY_AIRPORT is used to return information on a single airport in the facilities cache.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_AIRPORT.htm
type SIMCONNECT_DATA_FACILITY_AIRPORT struct {
	Ident     [6]byte // The airport ICAO code (char ident[6] in C)
	Region    [3]byte // The airport region code (char region[3] in C)
	Latitude  float64 // Latitude of the airport facility
	Longitude float64 // Longitude of the airport facility
	Altitude  float64 // Altitude of the facility in meters
}

// SIMCONNECT_DATA_FACILITY_WAYPOINT is used to return information on a single waypoint in the facilities cache.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_WAYPOINT.htm
type SIMCONNECT_DATA_FACILITY_WAYPOINT struct {
	SIMCONNECT_DATA_FACILITY_AIRPORT         // Inherits all members from AIRPORT
	FMagVar                          float32 // The magnetic variation of the waypoint in degrees
}

// SIMCONNECT_DATA_FACILITY_NDB is used to return information on a single NDB station in the facilities cache.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_NDB.htm
type SIMCONNECT_DATA_FACILITY_NDB struct {
	SIMCONNECT_DATA_FACILITY_WAYPOINT         // Inherits all members from WAYPOINT
	FFrequency                        float32 // Frequency of the station in Hz
}

// SIMCONNECT_DATA_FACILITY_VOR is used to return information on a single VOR station in the facilities cache.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_FACILITY_VOR.htm
type SIMCONNECT_DATA_FACILITY_VOR struct {
	SIMCONNECT_DATA_FACILITY_NDB         // Inherits all members from NDB
	Flags                        uint32  // Flags indicating field validity (NAV_SIGNAL, LOCALIZER, GLIDE_SLOPE, DME)
	FLocalizer                   float32 // The ILS localizer angle in degrees
	GlideLat                     float64 // The latitude of the glide slope transmitter in degrees
	GlideLon                     float64 // The longitude of the glide slope transmitter in degrees
	GlideAlt                     float64 // The altitude of the glide slope transmitter in degrees
	FGlideSlopeAngle             float32 // The ILS approach angle in degrees
}

// SIMCONNECT_ICAO structure contains ICAO information about a facility.
// Based on the usage context, this appears to contain ICAO code information.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_ICAO.htm
type SIMCONNECT_ICAO struct {
	Ident  [8]byte // ICAO identifier code
	Region [4]byte // ICAO region code
}

// SIMCONNECT_FACILITY_MINIMAL is used to provide minimal information about a facility.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FACILITY_MINIMAL.htm
type SIMCONNECT_FACILITY_MINIMAL struct {
	Icao SIMCONNECT_ICAO           // The ICAO struct with information about the facility
	Lla  SIMCONNECT_DATA_LATLONALT // The latitude, longitude and altitude of the facility
}
