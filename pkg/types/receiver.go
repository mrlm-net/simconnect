//go:build windows
// +build windows

package types

import "unsafe"

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV.htm
type SIMCONNECT_RECV struct {
	DwSize    DWORD // Size of the nested SIMCONNECT_RECV structure in bytes
	DwVersion DWORD // Version of the SIMCONNECT_RECV structure
	DwID      DWORD // SIMCONNECT_RECV_ID
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ID.htm
type SIMCONNECT_RECV_ID DWORD

const (
	SIMCONNECT_RECV_ID_NULL SIMCONNECT_RECV_ID = iota
	SIMCONNECT_RECV_ID_EXCEPTION
	SIMCONNECT_RECV_ID_OPEN
	SIMCONNECT_RECV_ID_QUIT
	SIMCONNECT_RECV_ID_EVENT
	SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE
	SIMCONNECT_RECV_ID_EVENT_FILENAME
	SIMCONNECT_RECV_ID_EVENT_FRAME
	SIMCONNECT_RECV_ID_SIMOBJECT_DATA
	SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
	SIMCONNECT_RECV_ID_WEATHER_OBSERVATION
	SIMCONNECT_RECV_ID_CLOUD_STATE
	SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID
	SIMCONNECT_RECV_ID_RESERVED_KEY
	SIMCONNECT_RECV_ID_CUSTOM_ACTION
	SIMCONNECT_RECV_ID_SYSTEM_STATE
	SIMCONNECT_RECV_ID_CLIENT_DATA
	SIMCONNECT_RECV_ID_EVENT_WEATHER_MODE
	SIMCONNECT_RECV_ID_AIRPORT_LIST
	SIMCONNECT_RECV_ID_VOR_LIST
	SIMCONNECT_RECV_ID_NDB_LIST
	SIMCONNECT_RECV_ID_WAYPOINT_LIST
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SERVER_STARTED
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_CLIENT_STARTED
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SESSION_ENDED
	SIMCONNECT_RECV_ID_EVENT_RACE_END
	SIMCONNECT_RECV_ID_EVENT_RACE_LAP
	SIMCONNECT_RECV_ID_PICK
	//SIMCONNECT_RECV_ID_EVENT_EX1 ??? there is some issue with probably this ID in the docs
	SIMCONNECT_RECV_ID_FACILITY_DATA
	SIMCONNECT_RECV_ID_FACILITY_DATA_END
	SIMCONNECT_RECV_ID_FACILITY_MINIMAL_LIST
	SIMCONNECT_RECV_ID_JETWAY_DATA
	SIMCONNECT_RECV_ID_CONTROLLERS_LIST
	SIMCONNECT_RECV_ID_ACTION_CALLBACK
	SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS
	SIMCONNECT_RECV_ID_GET_INPUT_EVENT
	SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT
	SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENT_PARAMS
	SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST
	SIMCONNECT_RECV_ID_FLOW_EVENT
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_OPEN.htm
type SIMCONNECT_RECV_OPEN struct {
	SIMCONNECT_RECV
	SzApplicationName         [260]byte // Name of the application that opened the connection
	DwApplicationVersionMajor DWORD
	DwApplicationVersionMinor DWORD
	DwApplicationBuildMajor   DWORD
	DwApplicationBuildMinor   DWORD
	DwSimConnectVersionMajor  DWORD
	DwSimConnectVersionMinor  DWORD
	DwSimConnectBuildMajor    DWORD
	DwSimConnectBuildMinor    DWORD
	DwReserved1               DWORD
	DwReserved2               DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_QUIT.htm
type SIMCONNECT_RECV_QUIT struct {
	SIMCONNECT_RECV
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SIMOBJECT_DATA.htm
type SIMCONNECT_RECV_SIMOBJECT_DATA struct {
	SIMCONNECT_RECV
	DwRequestID   DWORD // Request ID for the data
	DwObjectID    DWORD // Object ID for the data
	DwDefineID    DWORD // Define ID for the data
	DwFlags       DWORD // Flags for the data
	DwEntryNumber DWORD // Entry number for the data
	DwOutOf       DWORD // Out of for the data
	DwDefineCount DWORD // Define count for the data
	DwData        DWORD // Data for the data
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE.htm
type SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE struct {
	SIMCONNECT_RECV_SIMOBJECT_DATA
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SYSTEM_STATE.htm
type SIMCONNECT_RECV_SYSTEM_STATE struct {
	SIMCONNECT_RECV
	DwRequestID DWORD
	WInteger    DWORD
	FFloat      float64
	SzString    [260]byte // String data, typically used for state names or identifiers
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT.htm
type SIMCONNECT_RECV_EVENT struct {
	SIMCONNECT_RECV
	UGroupID DWORD
	UEventID DWORD
	DwData   DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_EX1.htm
type SIMCONNECT_RECV_EVENT_EX1 struct {
	SIMCONNECT_RECV
	UGroupID DWORD
	UEventID DWORD
	DwData0  DWORD
	DwData1  DWORD
	DwData2  DWORD
	DwData3  DWORD
	DwData4  DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_FILENAME.htm
type SIMCONNECT_RECV_EVENT_FILENAME struct {
	SIMCONNECT_RECV
	SzFileName [260]byte // Name of the file associated with the event;
	DwFlags    DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_FRAME.htm
type SIMCONNECT_RECV_EVENT_FRAME struct {
	SIMCONNECT_RECV_EVENT
	FFrameRate float64 // Frame rate at the time of the SIMCONNECT_RECV_EVENT_FRAME
	FSimSpeed  float64 // Simulation speed at the time of the event
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED struct {
	SIMCONNECT_RECV_EVENT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED struct {
	SIMCONNECT_RECV_EVENT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED struct {
	SIMCONNECT_RECV_EVENT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE.htm
type SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE struct {
	SIMCONNECT_RECV_EVENT
	EObjType SIMCONNECT_SIMOBJECT_TYPE // Type of the object being added or removed
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_RACE_END.htm
type SIMCONNECT_RECV_EVENT_RACE_END struct {
	SIMCONNECT_RECV_EVENT
	DwRacerNumber DWORD // Number of the racer that ended
	RacerData     SIMCONNECT_DATA_RACE_RESULT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_RACE_LAP.htm
type SIMCONNECT_RECV_EVENT_RACE_LAP struct {
	SIMCONNECT_RECV_EVENT
	DwLapIndex DWORD // Index of the lap
	RacerData  SIMCONNECT_DATA_RACE_RESULT
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EXCEPTION.htm
type SIMCONNECT_RECV_EXCEPTION struct {
	SIMCONNECT_RECV
	DwException DWORD // Exception code
	DwSendID    DWORD
	DwIndex     DWORD // Index of the exception
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITIES_LIST.htm
type SIMCONNECT_RECV_FACILITIES_LIST struct {
	SIMCONNECT_RECV
	DwRequestID   DWORD // Request ID for the facilities list
	DwArraySize   DWORD // Size of the array of facilities
	DwEntryNumber DWORD // Entry number in the facilities list
	DwOutOf       DWORD // Out of for the facilities list
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_DATA.htm
type SIMCONNECT_RECV_FACILITY_DATA struct {
	SIMCONNECT_RECV
	UserRequestId         DWORD                         // Request ID for the facility data
	UniqueRequestId       DWORD                         // Unique request ID for the facility data
	ParentUniqueRequestId DWORD                         // Parent request ID for the facility data
	Type                  SIMCONNECT_FACILITY_DATA_TYPE // Type of the facility
	IsListItem            DWORD                         // Indicates if this is a list item
	ItemIndex             DWORD
	ListSize              DWORD // Size of the list of facilities
	Data                  DWORD
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_DATA_END.htm
type SIMCONNECT_RECV_FACILITY_DATA_END struct {
	SIMCONNECT_RECV
	RequestId DWORD // Request ID for the end of facility data
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_MINIMAL_LIST.htm
type SIMCONNECT_RECV_FACILITY_MINIMAL_LIST struct {
	SIMCONNECT_RECV
	RequestID   DWORD                         // Request ID for the minimal facility list
	ArraySize   DWORD                         // Size of the array of facilities
	EntryNumber DWORD                         // Entry number in the minimal facility list
	OutOf       DWORD                         // Out of for the minimal facility list
	RgData      []SIMCONNECT_FACILITY_MINIMAL // Array of minimal facility data
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_GET_INPUT_EVENT.htm
type SIMCONNECT_RECV_GET_INPUT_EVENT struct {
	SIMCONNECT_RECV
	RequestID DWORD // Request ID for the input event
	Type      SIMCONNECT_INPUT_EVENT_TYPE
	Value     unsafe.Pointer // Pointer to the value of the input event, can be a double or string
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_JETWAY_DATA.htm
type SIMCONNECT_RECV_JETWAY_DATA struct {
	SIMCONNECT_RECV_LIST_TEMPLATE
	RgData []SIMCONNECT_JETWAY_DATA // Array of jetway data
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_LIST_TEMPLATE.htm
type SIMCONNECT_RECV_LIST_TEMPLATE struct {
	SIMCONNECT_RECV
	DwRequestID   DWORD // Request ID for the list template
	DwArraySize   DWORD // Size of the array of templates
	DwEntryNumber DWORD // Entry number in the list of templates
	DwOutOf       DWORD // Out of for the list of templates
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_NDB_LIST.htm
type SIMCONNECT_RECV_NDB_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST
	RgData []SIMCONNECT_DATA_FACILITY_NDB
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_RESERVED_KEY.htm
type SIMCONNECT_RECV_RESERVED_KEY struct {
	SIMCONNECT_RECV
	SzChoiceReserved [30]byte // Reserved key choice string
	SzReservedKey    [50]byte // Reserved key string
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.htm
type SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT struct {
	SIMCONNECT_RECV
	Hash  uint64                      // UINT64 Hash;
	Type  SIMCONNECT_INPUT_EVENT_TYPE // SIMCONNECT_INPUT_EVENT_TYPE Type;
	Value unsafe.Pointer              // PVOID Value;
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_VOR_LIST.htm
type SIMCONNECT_RECV_VOR_LIST struct {
	SIMCONNECT_RECV
	DwRequestID DWORD     // Request ID for the VOR SIMCONNECT_RECV_VOR_LIST
	DwInteger   DWORD     // Integer value
	FFloat      float64   // Float value
	SzString    [260]byte // String data, typically used for VOR names or identifiers
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_WAYPOINT_LIST.htm
type SIMCONNECT_RECV_WAYPOINT_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST
	RgData []SIMCONNECT_DATA_FACILITY_WAYPOINT // Array of waypoint data
}
