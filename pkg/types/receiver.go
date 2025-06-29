//go:build windows
// +build windows

package types

// SIMCONNECT_RECV_ID defines all possible message types that can be received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ID.htm
type SIMCONNECT_RECV_ID uint32

const (
	SIMCONNECT_RECV_ID_NULL                             SIMCONNECT_RECV_ID = iota // Null message
	SIMCONNECT_RECV_ID_EXCEPTION                                                  // Exception information
	SIMCONNECT_RECV_ID_OPEN                                                       // Connection established
	SIMCONNECT_RECV_ID_QUIT                                                       // Connection closed
	SIMCONNECT_RECV_ID_EVENT                                                      // Event information
	SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE                                     // Object added or removed
	SIMCONNECT_RECV_ID_EVENT_FILENAME                                             // Filename event
	SIMCONNECT_RECV_ID_EVENT_FRAME                                                // Frame event
	SIMCONNECT_RECV_ID_SIMOBJECT_DATA                                             // SimObject data
	SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE                                      // SimObject data by type
	SIMCONNECT_RECV_ID_WEATHER_OBSERVATION                                        // Weather observation
	SIMCONNECT_RECV_ID_CLOUD_STATE                                                // Cloud state
	SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID                                         // Assigned object ID
	SIMCONNECT_RECV_ID_RESERVED_KEY                                               // Reserved key
	SIMCONNECT_RECV_ID_CUSTOM_ACTION                                              // Custom action
	SIMCONNECT_RECV_ID_SYSTEM_STATE                                               // System state
	SIMCONNECT_RECV_ID_CLIENT_DATA                                                // Client data
	SIMCONNECT_RECV_ID_EVENT_WEATHER_MODE                                         // Weather mode event
	SIMCONNECT_RECV_ID_AIRPORT_LIST                                               // Airport list
	SIMCONNECT_RECV_ID_VOR_LIST                                                   // VOR list
	SIMCONNECT_RECV_ID_NDB_LIST                                                   // NDB list
	SIMCONNECT_RECV_ID_WAYPOINT_LIST                                              // Waypoint list
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SERVER_STARTED                           // Multiplayer server started
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_CLIENT_STARTED                           // Multiplayer client started
	SIMCONNECT_RECV_ID_EVENT_MULTIPLAYER_SESSION_ENDED                            // Multiplayer session ended
	SIMCONNECT_RECV_ID_EVENT_RACE_END                                             // Race end event
	SIMCONNECT_RECV_ID_EVENT_RACE_LAP                                             // Race lap event
	SIMCONNECT_RECV_ID_PICK                                                       // Pick event
	SIMCONNECT_RECV_ID_EVENT_EX1                                                  // Extended event 1
	SIMCONNECT_RECV_ID_FACILITY_DATA                                              // Facility data
	SIMCONNECT_RECV_ID_FACILITY_DATA_END                                          // Facility data end
	SIMCONNECT_RECV_ID_FACILITY_MINIMAL_LIST                                      // Facility minimal list
	SIMCONNECT_RECV_ID_JETWAY_DATA                                                // Jetway data
	SIMCONNECT_RECV_ID_CONTROLLERS_LIST                                           // Controllers list
	SIMCONNECT_RECV_ID_ACTION_CALLBACK                                            // Action callback
	SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS                                     // Enumerate input events
	SIMCONNECT_RECV_ID_GET_INPUT_EVENT                                            // Get input event
	SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT                                      // Subscribe to input event
	SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENT_PARAMS                               // Enumerate input event parameters
)

// SIMCONNECT_RECV is the base structure for all SimConnect messages
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV.htm
type SIMCONNECT_RECV struct {
	DwSize    uint32             // Size of the structure
	DwVersion uint32             // Version of SimConnect, matches SDK
	DwID      SIMCONNECT_RECV_ID // Message ID
}

// SIMCONNECT_RECV_SIMOBJECT_DATA represents SimObject data received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SIMOBJECT_DATA.htm
type SIMCONNECT_RECV_SIMOBJECT_DATA struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // ID of the client defined request
	DwObjectID      uint32 // ID of the client defined object
	DwDefineID      uint32 // ID of the client defined data definition
	DwFlags         uint32 // Flags that were set for this data request
	DwEntryNumber   uint32 // Index number of this object (1-based)
	DwOutOf         uint32 // Total number of objects being returned
	DwDefineCount   uint32 // Number of 8-byte elements in the data array
	DwData          uint32 // Start of data array (actual data follows)
}

// SIMCONNECT_RECV_EXCEPTION represents exception information from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EXCEPTION.htm
type SIMCONNECT_RECV_EXCEPTION struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwException     uint32 // Exception code
	DwSendID        uint32 // ID of the packet that caused the exception
	DwIndex         uint32 // Index number for some exceptions
}

// SIMCONNECT_RECV_EVENT represents event information received from SimConnect
// Field names match official SimConnect documentation
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT.htm
type SIMCONNECT_RECV_EVENT struct {
	SIMCONNECT_RECV        // Inherits from base structure
	UGroupID        uint32 // ID of the client defined group (uGroupID in official docs)
	UEventID        uint32 // ID of the client defined event (uEventID in official docs)
	DwData          uint32 // Event data - usually zero, but some events require additional qualification
}

// SIMCONNECT_RECV_EVENT_EX1 represents extended event information received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_EX1.htm
type SIMCONNECT_RECV_EVENT_EX1 struct {
	SIMCONNECT_RECV        // Inherits from base structure
	UGroupID        uint32 // ID of the client defined group
	UEventID        uint32 // ID of the client defined event
	DwData0         uint32 // First event data parameter
	DwData1         uint32 // Second event data parameter
	DwData2         uint32 // Third event data parameter
	DwData3         uint32 // Fourth event data parameter
	DwData4         uint32 // Fifth event data parameter
}

// SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE represents SimObject data by type received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE.htm
type SIMCONNECT_RECV_SIMOBJECT_DATA_BYTYPE struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // ID of the client defined request
	DwObjectID      uint32 // ID of the client defined object
	DwDefineID      uint32 // ID of the client defined data definition
	DwFlags         uint32 // Flags that were set for this data request
	DwEntryNumber   uint32 // Index number of this object (1-based)
	DwOutOf         uint32 // Total number of objects being returned
	DwDefineCount   uint32 // Number of 8-byte elements in the data array
	DwData          uint32 // Start of data array (actual data follows)
}

// SIMCONNECT_RECV_ASSIGNED_OBJECT_ID represents assigned object ID received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ASSIGNED_OBJECT_ID.htm
type SIMCONNECT_RECV_ASSIGNED_OBJECT_ID struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwObjectID      uint32 // ID of the assigned object
	DwRequestID     uint32 // ID of the original request
}

// SIMCONNECT_RECV_SYSTEM_STATE represents system state received from SimConnect
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SYSTEM_STATE.htm
type SIMCONNECT_RECV_SYSTEM_STATE struct {
	SIMCONNECT_RECV           // Inherits from base structure
	DwRequestID     uint32    // ID of the client defined request
	DwInteger       uint32    // Integer value of the system state
	DwFloat         uint32    // Float value of the system state (as uint32)
	SzString        [260]byte // String value of the system state
}

// SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE represents object add/remove events
// Used for tracking when AI aircraft, vehicles, or other objects are added/removed from simulation
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE.htm
type SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE struct {
	SIMCONNECT_RECV        // Inherits from base structure
	UEventID        uint32 // Event ID for the object add/remove event
	DwData          uint32 // Object ID of the added/removed object
}

// SIMCONNECT_RECV_EVENT_FILENAME represents filename-related events
// Used for tracking flight plan loads, aircraft model changes, etc.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_FILENAME.htm
type SIMCONNECT_RECV_EVENT_FILENAME struct {
	SIMCONNECT_RECV           // Inherits from base structure
	UEventID        uint32    // Event ID for the filename event
	DwData          uint32    // Data associated with the event (MISSING FIELD, aligns struct with C SDK)
	DwFlags         uint32    // Flags associated with the filename event
	DwGroupID       uint32    // Group ID for the event
	SzFileName      [260]byte // Filename associated with the event
}

// SIMCONNECT_RECV_EVENT_FRAME represents frame timing events
// Used for frame-based notifications and timing-sensitive operations
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_FRAME.htm
type SIMCONNECT_RECV_EVENT_FRAME struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwFrameRate     uint32 // Current frame rate
	DwSimSpeed      uint32 // Current simulation speed multiplier
}

// SIMCONNECT_RECV_FACILITY_DATA represents facility (airport/navigation) data
// Used for receiving information about airports, VORs, NDBs, etc.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_DATA.htm
type SIMCONNECT_RECV_FACILITY_DATA struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // ID of the original request
	DwArraySize     uint32 // Number of facilities in the data
	DwEntryNumber   uint32 // Index of this entry (1-based)
	DwOutOf         uint32 // Total number of entries
}

// SIMCONNECT_RECV_OPEN is used to return information to the client, after a successful call to SimConnect_Open.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_OPEN.htm
type SIMCONNECT_RECV_OPEN struct {
	SIMCONNECT_RECV                     // Inherits from base structure
	SzApplicationName         [256]byte // Null-terminated string containing the application name
	DwApplicationVersionMajor uint32    // Application version major number
	DwApplicationVersionMinor uint32    // Application version minor number
	DwApplicationBuildMajor   uint32    // Application build major number
	DwApplicationBuildMinor   uint32    // Application build minor number
	DwSimConnectVersionMajor  uint32    // SimConnect version major number
	DwSimConnectVersionMinor  uint32    // SimConnect version minor number
	DwSimConnectBuildMajor    uint32    // SimConnect build major number
	DwSimConnectBuildMinor    uint32    // SimConnect build minor number
	DwReserved1               uint32    // Reserved
	DwReserved2               uint32    // Reserved
}

// SIMCONNECT_RECV_QUIT is an identical structure to the SIMCONNECT_RECV structure.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_QUIT.htm
type SIMCONNECT_RECV_QUIT struct {
	SIMCONNECT_RECV // Inherits from base structure (no additional fields)
}

// SIMCONNECT_RECV_CLIENT_DATA will be received by the client after a successful call to SimConnect_RequestClientData.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_CLIENT_DATA.htm
type SIMCONNECT_RECV_CLIENT_DATA struct {
	SIMCONNECT_RECV_SIMOBJECT_DATA // Inherits from SIMCONNECT_RECV_SIMOBJECT_DATA (identical structure)
}

// SIMCONNECT_RECV_RESERVED_KEY is used with the SimConnect_RequestReservedKey function to return the reserved key combination.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_RESERVED_KEY.htm
type SIMCONNECT_RECV_RESERVED_KEY struct {
	SIMCONNECT_RECV           // Inherits from base structure
	SzChoiceReserved [30]byte // Null-terminated string containing the key that has been reserved
	SzReservedKey    [50]byte // Null-terminated string containing the reserved key combination
}

// SIMCONNECT_RECV_FACILITIES_LIST is used to provide information on the number of elements in a list of facilities.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITIES_LIST.htm
type SIMCONNECT_RECV_FACILITIES_LIST struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // Client defined request ID
	DwArraySize     uint32 // Number of elements in the list within this packet
	DwEntryNumber   uint32 // Index number of this list packet (0 to dwOutOf - 1)
	DwOutOf         uint32 // Total number of packets used to transmit the list
}

// SIMCONNECT_RECV_AIRPORT_LIST is used to return a list of SIMCONNECT_DATA_FACILITY_AIRPORT structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_AIRPORT_LIST.htm
type SIMCONNECT_RECV_AIRPORT_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST // Inherits from FACILITIES_LIST
	// rgData[dwArraySize] - Array of SIMCONNECT_DATA_FACILITY_AIRPORT structures follows in memory
}

// SIMCONNECT_RECV_VOR_LIST is used to return a list of SIMCONNECT_DATA_FACILITY_VOR structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_VOR_LIST.htm
type SIMCONNECT_RECV_VOR_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST // Inherits from FACILITIES_LIST
	// rgData[dwArraySize] - Array of SIMCONNECT_DATA_FACILITY_VOR structures follows in memory
}

// SIMCONNECT_RECV_NDB_LIST is used to return a list of SIMCONNECT_DATA_FACILITY_NDB structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_NDB_LIST.htm
type SIMCONNECT_RECV_NDB_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST // Inherits from FACILITIES_LIST
	// rgData[dwArraySize] - Array of SIMCONNECT_DATA_FACILITY_NDB structures follows in memory
}

// SIMCONNECT_RECV_WAYPOINT_LIST is used to return a list of SIMCONNECT_DATA_FACILITY_WAYPOINT structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_WAYPOINT_LIST.htm
type SIMCONNECT_RECV_WAYPOINT_LIST struct {
	SIMCONNECT_RECV_FACILITIES_LIST // Inherits from FACILITIES_LIST
	// rgData[dwArraySize] - Array of SIMCONNECT_DATA_FACILITY_WAYPOINT structures follows in memory
}

// SIMCONNECT_RECV_FACILITY_DATA_END is used to signify the end of a data stream from the server after a call to SimConnect_RequestFacilityData.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_DATA_END.htm
type SIMCONNECT_RECV_FACILITY_DATA_END struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // Client defined request ID
}

// SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED is sent to the host when the session is visible to other users in the lobby.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_SERVER_STARTED struct {
	SIMCONNECT_RECV_EVENT // Inherits from EVENT (no additional fields)
}

// SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED is sent to a client when they have successfully joined a multi-player race.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_CLIENT_STARTED struct {
	SIMCONNECT_RECV_EVENT // Inherits from EVENT (no additional fields)
}

// SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED is sent to a client when they have requested to leave a race, or to all players when the session is terminated by the host.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED.htm
type SIMCONNECT_RECV_EVENT_MULTIPLAYER_SESSION_ENDED struct {
	SIMCONNECT_RECV_EVENT // Inherits from EVENT (no additional fields)
}

// SIMCONNECT_RECV_EVENT_RACE_END is used in multi-player racing to hold the results for one player at the end of a race.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_RACE_END.htm
type SIMCONNECT_RECV_EVENT_RACE_END struct {
	SIMCONNECT_RECV_EVENT                             // Inherits from EVENT
	DwRacerNumber         uint32                      // Index of the racer the results are for (players indexed from 0)
	RacerData             SIMCONNECT_DATA_RACE_RESULT // Race result data
}

// SIMCONNECT_RECV_EVENT_RACE_LAP is used in multi-player racing to hold the results for one player at the end of a lap.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_EVENT_RACE_LAP.htm
type SIMCONNECT_RECV_EVENT_RACE_LAP struct {
	SIMCONNECT_RECV_EVENT                             // Inherits from EVENT
	DwLapIndex            uint32                      // Index of the lap the results are for (laps indexed from 0)
	RacerData             SIMCONNECT_DATA_RACE_RESULT // Race result data
}

// SIMCONNECT_RECV_LIST_TEMPLATE is used to provide information on the number of elements in a list returned to the client.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_LIST_TEMPLATE.htm
type SIMCONNECT_RECV_LIST_TEMPLATE struct {
	SIMCONNECT_RECV        // Inherits from base structure
	DwRequestID     uint32 // Client defined request ID
	DwArraySize     uint32 // Number of elements in the list within this packet
	DwEntryNumber   uint32 // Index number of this list packet (0 to dwOutOf - 1)
	DwOutOf         uint32 // Total number of packets used to transmit the list
}

// SIMCONNECT_INPUT_EVENT_TYPE enumeration is used with input event calls to specify the data type.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_INPUT_EVENT_TYPE.htm
type SIMCONNECT_INPUT_EVENT_TYPE uint32

const (
	SIMCONNECT_INPUT_EVENT_TYPE_NONE   SIMCONNECT_INPUT_EVENT_TYPE = iota // No data type specification required
	SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE                                    // Specifies a double
	SIMCONNECT_INPUT_EVENT_TYPE_STRING                                    // Specifies a string
)

// SIMCONNECT_JETWAY_DATA is used to return information on a single jetway.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_JETWAY_DATA.htm
type SIMCONNECT_JETWAY_DATA struct {
	SIMCONNECT_RECV                               // Inherits from base structure
	AirportIcao         [8]byte                   // ICAO code of the airport
	ParkingIndex        int32                     // Index of the parking space linked to this jetway
	Lla                 SIMCONNECT_DATA_LATLONALT // Latitude/Longitude/Altitude of the jetway
	Pbh                 SIMCONNECT_DATA_PBH       // Pitch/Bank/Heading of the jetway
	Status              int32                     // Status of the jetway (0-7, see documentation for values)
	Door                int32                     // Index of the door attached to the jetway
	ExitDoorRelativePos SIMCONNECT_DATA_XYZ       // Door position relative to aircraft
	MainHandlePos       SIMCONNECT_DATA_XYZ       // Relative position of IK_MainHandle (world pos, in meters)
	SecondaryHandle     SIMCONNECT_DATA_XYZ       // Relative position of IK_SecondaryHandle (world pos, in meters)
	WheelGroundLock     SIMCONNECT_DATA_XYZ       // Relative position of IK_WheelsGroundLock (world pos, in meters)
	JetwayObjectId      uint32                    // ObjectId of the jetway
	AttachedObjectId    uint32                    // ObjectId of the object (aircraft) attached to the jetway
}

// SIMCONNECT_INPUT_EVENT_DESCRIPTOR is used to return an item of data for a specific input event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_INPUT_EVENT_DESCRIPTOR.htm
type SIMCONNECT_INPUT_EVENT_DESCRIPTOR struct {
	Name      [64]byte            // The name of the Input Event (SIMCONNECT_STRING(Name, 64))
	Hash      uint32              // The hash ID for the event
	Type      SIMCONNECT_DATATYPE // The expected datatype (usually FLOAT32 or STRING128)
	NodeNames [1024]byte          // List of node names linked to this InputEvent, separated by ; (SIMCONNECT_STRING(NodeNames, 1024))
}

// SIMCONNECT_RECV_CONTROLLERS_LIST is used to return a list of SIMCONNECT_CONTROLLER_ITEM structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_CONTROLLERS_LIST.htm
type SIMCONNECT_RECV_CONTROLLERS_LIST struct {
	SIMCONNECT_RECV // Inherits from base structure
	// rgData[dwArraySize] - Array of SIMCONNECT_CONTROLLER_ITEM structures follows in memory
}

// SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS is used to return a single page of data about an input event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS.htm
type SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS struct {
	SIMCONNECT_RECV_LIST_TEMPLATE // Inherits from LIST_TEMPLATE
	// rgData[dwArraySize] - Array of SIMCONNECT_INPUT_EVENT_DESCRIPTOR structures follows in memory
}

// SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS is a response with the available parameters for an input event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS.htm
type SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS struct {
	SIMCONNECT_RECV           // Inherits from base structure
	Hash            uint64    // Hash ID that identifies the input event
	Value           [260]byte // String that contains the values, separated by ; (STRING type mapped to byte array)
}

// SIMCONNECT_RECV_JETWAY_DATA is used to return a list of SIMCONNECT_JETWAY_DATA structures.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_JETWAY_DATA.htm
type SIMCONNECT_RECV_JETWAY_DATA struct {
	SIMCONNECT_RECV_LIST_TEMPLATE // Inherits from LIST_TEMPLATE
	// rgData[dwArraySize] - Array of SIMCONNECT_JETWAY_DATA structures follows in memory
}

// SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT is used to return the value of a subscribed input event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.htm
type SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT struct {
	SIMCONNECT_RECV                             // Inherits from base structure
	Hash            uint64                      // Hash ID that will identify the subscribed input event
	Type            SIMCONNECT_INPUT_EVENT_TYPE // Type enumeration to cast Value correctly
	Value           uintptr                     // The value of the requested input event (PVOID mapped to uintptr)
}

// SIMCONNECT_RECV_INPUT_EVENT_VALUE is used to return the value of a specific input event.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_INPUT_EVENT_VALUE.htm
type SIMCONNECT_RECV_INPUT_EVENT_VALUE struct {
	SIMCONNECT_RECV                             // Inherits from base structure
	DwRequestID     uint32                      // Client defined request ID
	Type            SIMCONNECT_INPUT_EVENT_TYPE // Type enumeration to cast Value correctly
	Value           uintptr                     // The value of the requested input event (PVOID mapped to uintptr)
}

// SIMCONNECT_VERSION_BASE_TYPE contains version information for hardware.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_VERSION_BASE_TYPE.htm
// Note: Currently not used in the simulation, members will be 0.
type SIMCONNECT_VERSION_BASE_TYPE struct {
	DwMajor    uint32 // Major version number (currently 0)
	DwMinor    uint32 // Minor version number (currently 0)
	DwBuild    uint32 // Build number (currently 0)
	DwRevision uint32 // Revision number (currently 0)
}

// SIMCONNECT_CONTROLLER_ITEM contains data related to a single controller currently connected to the simulation.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_CONTROLLER_ITEM.htm
type SIMCONNECT_CONTROLLER_ITEM struct {
	DeviceName      [256]byte                    // Descriptive name for the device (SIMCONNECT_STRING(DeviceName, 256))
	DeviceId        uint32                       // The device ID
	ProductId       uint32                       // The product ID
	CompositeID     uint32                       // ID of the USB composite device
	HardwareVersion SIMCONNECT_VERSION_BASE_TYPE // Version data for the hardware (currently unused, will be 0)
}

// SIMCONNECT_RECV_FACILITY_MINIMAL_LIST is used to provide minimal information on facilities.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FACILITY_MINIMAL_LIST.htm
type SIMCONNECT_RECV_FACILITY_MINIMAL_LIST struct {
	SIMCONNECT_RECV_LIST_TEMPLATE // Inherits from LIST_TEMPLATE
	// rgData[dwArraySize] - Array of SIMCONNECT_FACILITY_MINIMAL structures follows in memory
}
