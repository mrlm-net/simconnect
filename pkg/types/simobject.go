package types

type SIMCONNECT_SIMOBJECT_TYPE DWORD

const (
	SIMCONNECT_SIMOBJECT_TYPE_USER SIMCONNECT_SIMOBJECT_TYPE = iota
	SIMCONNECT_SIMOBJECT_TYPE_ALL
	SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT
	SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER
	SIMCONNECT_SIMOBJECT_TYPE_BOAT
	SIMCONNECT_SIMOBJECT_TYPE_GROUND
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST.htm
type SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST struct {
	SIMCONNECT_RECV_LIST_TEMPLATE
	// RgData marks offset 28, where DwArraySize entries follow (the Go struct
	// matches the wire, 512 bytes): read them with Entries.
	RgData [0]SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY.htm
type SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY struct {
	AircraftTitle [256]byte // String256
	LiveryName    [256]byte // String256
}
