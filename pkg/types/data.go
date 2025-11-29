//go:build windows
// +build windows

package types

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATATYPE.htm
type SIMCONNECT_DATATYPE DWORD

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

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_RACE_RESULT.htm
type SIMCONNECT_DATA_RACE_RESULT struct {
	DwNumberOfRacers DWORD     // DWORD dwNumberOfRacers;
	MissionGUID      [16]byte  // GUID MissionGUID (16 bytes)
	SzPlayerName     [260]byte // char szPlayerName[MAX_PATH];
	SzSessionType    [260]byte // char szSessionType[MAX_PATH];
	SzAircraft       [260]byte // char szAircraft[MAX_PATH];
	SzPlayerRole     [260]byte // char szPlayerRole[MAX_PATH];
	FTotalTime       float64   // double fTotalTime;
	FPenaltyTime     float64   // double fPenaltyTime;
	DwIsDisqualified DWORD     // DWORD dwIsDisqualified;
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_LATLONALT.htm
type SIMCONNECT_DATA_LATLONALT struct {
	Latitude  float64 // double Latitude
	Longitude float64 // double Longitude
	Altitude  float64 // double Altitude
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_PBH.htm
type SIMCONNECT_DATA_PBH struct {
	Pitch   float64 // double Pitch
	Bank    float64 // double Bank
	Heading float64 // double Heading

}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_XYZ.htm
type SIMCONNECT_DATA_XYZ struct {
	X float64 // double X
	Y float64 // double Y
	Z float64 // double Z
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_INITPOSITION.htm
type SIMCONNECT_DATA_INITPOSITION struct {
	Latitude  float64 // double Latitude
	Longitude float64 // double Longitude
	Altitude  float64 // double Altitude
	Pitch     float64 // double Pitch
	Bank      float64 // double Bank
	Heading   float64 // double Heading
	OnGround  DWORD
	Airspeed  DWORD
}

const (
	INITPOSITION_AIRSPEED_CRUISE = -1 // DWORD Airspeed
	INITPOSITION_AIRSPEED_KEEP   = -2 // DWORD Airspeed
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_MARKERSTATE.htm
type SIMCONNECT_DATA_MARKERSTATE struct {
	SzMarkerName  [64]byte // char szMarkerName[64];
	DwMarkerState DWORD    // DWORD dwMarkerState;
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_AIRPORT_LIST.htm
type SIMCONNECT_RECV_AIRPORT_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST
	RgData []SIMCONNECT_DATA_FACILITY_AIRPORT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ASSIGNED_OBJECT_ID.htm
type SIMCONNECT_RECV_ASSIGNED_OBJECT_ID struct {
	SIMCONNECT_RECV
	DwRequestID DWORD // DWORD dwRequestID;
	DwObjectID  DWORD // DWORD dwObjectID;
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE.htm
type SIMCONNECT_RECV_CLIENT_DATA struct {
	SIMCONNECT_RECV_SIMOBJECT_DATA
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_CONTROLLERS_LIST.htm
type SIMCONNECT_RECV_CONTROLLERS_LIST struct {
	SIMCONNECT_RECV
	RgData []SIMCONNECT_CONTROLLER_ITEM
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToClientDataDefinition.htm
type SIMCONNECT_CLIENTDATATYPE uint32

const (
	SIMCONNECT_CLIENTDATATYPE_INT8    SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFF
	SIMCONNECT_CLIENTDATATYPE_INT16   SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFE
	SIMCONNECT_CLIENTDATATYPE_INT32   SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFD
	SIMCONNECT_CLIENTDATATYPE_INT64   SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFC
	SIMCONNECT_CLIENTDATATYPE_FLOAT32 SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFB
	SIMCONNECT_CLIENTDATATYPE_FLOAT64 SIMCONNECT_CLIENTDATATYPE = 0xFFFFFFFA
)

type SIMCONNECT_CLIENT_DATA_REQUEST_FLAG uint32

const (
	SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT SIMCONNECT_CLIENT_DATA_REQUEST_FLAG = iota
	SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_CHANGED
	SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_TAGGED
)

type SIMCONNECT_CREATE_CLIENT_DATA_FLAG uint32

const (
	SIMCONNECT_CREATE_CLIENT_DATA_FLAG_READ_ONLY SIMCONNECT_CREATE_CLIENT_DATA_FLAG = iota
)

type SIMCONNECT_DATA_REQUEST_FLAG uint32

const (
	SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT SIMCONNECT_DATA_REQUEST_FLAG = iota
	SIMCONNECT_DATA_REQUEST_FLAG_CHANGED
	SIMCONNECT_DATA_REQUEST_FLAG_TAGGED
)

type SIMCONNECT_DATA_SET_FLAG uint32

const (
	SIMCONNECT_DATA_SET_FLAG_DEFAULT SIMCONNECT_DATA_SET_FLAG = iota
	SIMCONNECT_DATA_SET_FLAG_TAGGED
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_CONTROLLER_ITEM.htm
type SIMCONNECT_CONTROLLER_ITEM struct {
	DeviceName      [256]byte                    // SIMCONNECT_STRING(DeviceName, 256)
	DeviceId        uint32                       // unsigned int DeviceId
	ProductId       uint32                       // unsigned int ProductId
	CompositeID     uint32                       // unsigned int CompositeID
	HardwareVersion SIMCONNECT_VERSION_BASE_TYPE // SIMCONNECT_VERSION_BASE_TYPE HardwareVersion
}
