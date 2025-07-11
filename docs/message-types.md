# Message Types

This document provides a comprehensive reference for all SimConnect message types supported by the Go library, including their structure, usage, and helper methods.

## Table of Contents

- [Message Processing Overview](#message-processing-overview)
- [Core Message Types](#core-message-types)
- [Data Messages](#data-messages)
- [Event Messages](#event-messages)
- [System Messages](#system-messages)
- [Facility Messages](#facility-messages)
- [Input Event Messages](#input-event-messages)
- [Client Data Messages](#client-data-messages)
- [Error and Exception Messages](#error-and-exception-messages)
- [Helper Methods](#helper-methods)

## Message Processing Overview

All SimConnect messages are wrapped in the `ParsedMessage` structure:

```go
type ParsedMessage struct {
    MessageType types.SIMCONNECT_RECV_ID // Message type identifier
    Header      *types.SIMCONNECT_RECV   // Base header information
    Data        interface{}              // Parsed message data
    RawData     []byte                   // Raw byte data
    Error       error                    // Any parsing error
}
```

### Message Flow

1. **Raw Message** - Received from SimConnect as binary data
2. **Parsing** - Converted to appropriate Go struct based on message type
3. **Dispatch** - Sent through message channel to application
4. **Processing** - Handled by application message handlers

## Core Message Types

### SIMCONNECT_RECV_ID_OPEN

Sent when a connection is successfully established.

**Type:** `types.SIMCONNECT_RECV_OPEN`

**Structure:**
```go
type SIMCONNECT_RECV_OPEN struct {
    SIMCONNECT_RECV             // Base header
    SzApplicationName    [256]byte // Application name
    DwApplicationVersionMajor uint32 // Major version
    DwApplicationVersionMinor uint32 // Minor version
    DwApplicationBuildMajor   uint32 // Build major
    DwApplicationBuildMinor   uint32 // Build minor
    DwSimConnectVersionMajor  uint32 // SimConnect major version
    DwSimConnectVersionMinor  uint32 // SimConnect minor version
    DwSimConnectBuildMajor    uint32 // SimConnect build major
    DwSimConnectBuildMinor    uint32 // SimConnect build minor
    DwReserved1              uint32 // Reserved
    DwReserved2              uint32 // Reserved
}
```

**Usage:**
```go
if msg.IsOpen() {
    if openData, ok := msg.GetOpen(); ok {
        fmt.Printf("Connected to: %s\n", string(openData.SzApplicationName[:]))
        fmt.Printf("SimConnect Version: %d.%d\n", 
            openData.DwSimConnectVersionMajor, 
            openData.DwSimConnectVersionMinor)
    }
}
```

### SIMCONNECT_RECV_ID_QUIT

Sent when the connection is terminated by the simulator.

**Type:** `types.SIMCONNECT_RECV_QUIT`

**Structure:**
```go
type SIMCONNECT_RECV_QUIT struct {
    SIMCONNECT_RECV // Base header only
}
```

**Usage:**
```go
if msg.IsQuit() {
    fmt.Println("SimConnect connection terminated by simulator")
    // Perform cleanup
    return
}
```

## Data Messages

### SIMCONNECT_RECV_ID_SIMOBJECT_DATA

Contains simulation object data requested via `RequestDataOnSimObject`.

**Type:** `types.SIMCONNECT_RECV_SIMOBJECT_DATA`

**Structure:**
```go
type SIMCONNECT_RECV_SIMOBJECT_DATA struct {
    SIMCONNECT_RECV
    DwRequestID   uint32 // Request identifier
    DwObjectID    uint32 // Object identifier
    DwDefineID    uint32 // Data definition identifier
    DwFlags       uint32 // Flags
    Dwentrynumber uint32 // Entry number (for arrays)
    DwoutofRange  uint32 // Out of range indicator
    DwDefineCount uint32 // Number of definitions
    DwData        uint32 // Start of actual data
}
```

**Usage:**
```go
if msg.IsSimObjectData() {
    if data, ok := msg.GetSimObjectData(); ok {
        // Cast DwData to your struct
        aircraft := (*AircraftData)(unsafe.Pointer(&data.DwData))
        
        fmt.Printf("Request ID: %d\n", data.DwRequestID)
        fmt.Printf("Object ID: %d\n", data.DwObjectID)
        fmt.Printf("Altitude: %.0f ft\n", aircraft.Altitude)
    }
}
```

### SIMCONNECT_RECV_ID_CLIENT_DATA

Contains client data area information.

**Type:** `types.SIMCONNECT_RECV_CLIENT_DATA`

**Structure:**
```go
type SIMCONNECT_RECV_CLIENT_DATA struct {
    SIMCONNECT_RECV
    DwRequestID   uint32 // Request identifier
    DwClientDataID uint32 // Client data area identifier
    DwDefineID    uint32 // Data definition identifier
    DwFlags       uint32 // Flags
    Dwentrynumber uint32 // Entry number
    DwData        uint32 // Start of actual data
}
```

**Usage:**
```go
if msg.IsClientData() {
    if data, ok := msg.GetClientData(); ok {
        // Process client data
        clientData := (*YourClientDataStruct)(unsafe.Pointer(&data.DwData))
        fmt.Printf("Client Data ID: %d\n", data.DwClientDataID)
    }
}
```

## Event Messages

### SIMCONNECT_RECV_ID_EVENT

Standard event notification.

**Type:** `types.SIMCONNECT_RECV_EVENT`

**Structure:**
```go
type SIMCONNECT_RECV_EVENT struct {
    SIMCONNECT_RECV
    UGroupID  uint32 // Group identifier
    UEventID  uint32 // Event identifier
    DwData    uint32 // Event data
}
```

**Usage:**
```go
if msg.IsEvent() {
    if event, ok := msg.GetEvent(); ok {
        switch event.UEventID {
        case GEAR_TOGGLE_EVENT:
            fmt.Printf("Landing gear toggled, data: %d\n", event.DwData)
        case FLAPS_EVENT:
            fmt.Printf("Flaps event, position: %d\n", event.DwData)
        }
    }
}
```

### SIMCONNECT_RECV_ID_EVENT_EX1

Extended event with multiple parameters.

**Type:** `types.SIMCONNECT_RECV_EVENT_EX1`

**Structure:**
```go
type SIMCONNECT_RECV_EVENT_EX1 struct {
    SIMCONNECT_RECV
    UGroupID uint32    // Group identifier
    UEventID uint32    // Event identifier
    DwData   [5]uint32 // Event data array (up to 5 parameters)
}
```

**Usage:**
```go
if msg.IsEventEX1() {
    if event, ok := msg.GetEventEX1(); ok {
        fmt.Printf("Extended Event ID: %d\n", event.UEventID)
        fmt.Printf("Parameters: %v\n", event.DwData)
    }
}
```

## System Messages

### SIMCONNECT_RECV_ID_SYSTEM_STATE

System state information response.

**Type:** `types.SIMCONNECT_RECV_SYSTEM_STATE`

**Structure:**
```go
type SIMCONNECT_RECV_SYSTEM_STATE struct {
    SIMCONNECT_RECV
    DwRequestID uint32     // Request identifier
    DwInteger   uint32     // Integer value
    FFloat      float32    // Float value
    SzString    [260]byte  // String value (MAX_PATH)
}
```

**Usage:**
```go
if msg.IsSystemState() {
    if state, ok := msg.GetSystemState(); ok {
        stateName := string(state.SzString[:])
        fmt.Printf("System State: %s\n", stateName)
        fmt.Printf("Integer Value: %d\n", state.DwInteger)
        fmt.Printf("Float Value: %.2f\n", state.FFloat)
    }
}
```

## Facility Messages

### SIMCONNECT_RECV_ID_AIRPORT_LIST

List of airports in response to facility requests.

**Type:** `types.SIMCONNECT_RECV_AIRPORT_LIST`

**Structure:**
```go
type SIMCONNECT_RECV_AIRPORT_LIST struct {
    SIMCONNECT_RECV
    DwRequestID    uint32 // Request identifier
    DwArraySize    uint32 // Number of airports
    DwEntryNumber  uint32 // Entry number
    DwOutOf        uint32 // Total entries available
    // Followed by array of SIMCONNECT_DATA_FACILITY_AIRPORT
}
```

**Usage:**
```go
if msg.IsAirportList() {
    if airports, ok := msg.GetAirportList(); ok {
        fmt.Printf("Found %d airports\n", airports.DwArraySize)
        // Process airport data
    }
}
```

### SIMCONNECT_RECV_ID_VOR_LIST

List of VOR stations.

**Type:** `types.SIMCONNECT_RECV_VOR_LIST`

**Structure:**
```go
type SIMCONNECT_RECV_VOR_LIST struct {
    SIMCONNECT_RECV
    DwRequestID    uint32 // Request identifier
    DwArraySize    uint32 // Number of VORs
    DwEntryNumber  uint32 // Entry number
    DwOutOf        uint32 // Total entries available
    // Followed by array of SIMCONNECT_DATA_FACILITY_VOR
}
```

### SIMCONNECT_RECV_ID_NDB_LIST

List of NDB stations.

**Type:** `types.SIMCONNECT_RECV_NDB_LIST`

### SIMCONNECT_RECV_ID_WAYPOINT_LIST

List of waypoints.

**Type:** `types.SIMCONNECT_RECV_WAYPOINT_LIST`

### SIMCONNECT_RECV_ID_FACILITY_DATA

Detailed facility data.

**Type:** `types.SIMCONNECT_RECV_FACILITY_DATA`

**Structure:**
```go
type SIMCONNECT_RECV_FACILITY_DATA struct {
    SIMCONNECT_RECV
    DwRequestID uint32 // Request identifier
    DwData      uint32 // Start of facility-specific data
}
```

## Input Event Messages

### SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS

Response to input event enumeration request.

**Type:** `types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS`

**Structure:**
```go
type SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS struct {
    SIMCONNECT_RECV
    DwRequestID    uint32 // Request identifier
    DwArraySize    uint32 // Number of input events
    DwEntryNumber  uint32 // Entry number
    DwOutOf        uint32 // Total entries available
    // Followed by array of input event data
}
```

### SIMCONNECT_RECV_ID_GET_INPUT_EVENT

Response to get input event request.

**Type:** `types.SIMCONNECT_RECV_GET_INPUT_EVENT`

### SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT

Input event subscription notification.

**Type:** `types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT`

### SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENT_PARAMS

Input event parameters enumeration.

**Type:** `types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENT_PARAMS`

## Error and Exception Messages

### SIMCONNECT_RECV_ID_EXCEPTION

Error and exception information.

**Type:** `types.SIMCONNECT_RECV_EXCEPTION`

**Structure:**
```go
type SIMCONNECT_RECV_EXCEPTION struct {
    SIMCONNECT_RECV
    DwException    uint32 // Exception code
    DwSendID       uint32 // Send identifier that caused exception
    DwIndex        uint32 // Parameter index (for data definition errors)
}
```

**Common Exception Codes:**
- `SIMCONNECT_EXCEPTION_NONE` (0) - No error
- `SIMCONNECT_EXCEPTION_ERROR` (1) - General error
- `SIMCONNECT_EXCEPTION_SIZE_MISMATCH` (2) - Size mismatch
- `SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID` (3) - Unrecognized ID
- `SIMCONNECT_EXCEPTION_UNOPENED` (4) - Connection not opened
- `SIMCONNECT_EXCEPTION_VERSION_MISMATCH` (5) - Version mismatch
- `SIMCONNECT_EXCEPTION_TOO_MANY_GROUPS` (6) - Too many groups
- `SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED` (7) - Name not recognized
- `SIMCONNECT_EXCEPTION_TOO_MANY_EVENT_NAMES` (8) - Too many event names
- `SIMCONNECT_EXCEPTION_EVENT_ID_DUPLICATE` (9) - Duplicate event ID
- `SIMCONNECT_EXCEPTION_TOO_MANY_MAPS` (10) - Too many maps
- `SIMCONNECT_EXCEPTION_TOO_MANY_OBJECTS` (11) - Too many objects
- `SIMCONNECT_EXCEPTION_TOO_MANY_REQUESTS` (12) - Too many requests
- `SIMCONNECT_EXCEPTION_WEATHER_INVALID_PORT` (13) - Invalid weather port
- `SIMCONNECT_EXCEPTION_WEATHER_INVALID_METAR` (14) - Invalid METAR
- `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_GET_OBSERVATION` (15) - Unable to get weather observation
- `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_CREATE_STATION` (16) - Unable to create weather station
- `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_REMOVE_STATION` (17) - Unable to remove weather station
- `SIMCONNECT_EXCEPTION_INVALID_DATA_TYPE` (18) - Invalid data type
- `SIMCONNECT_EXCEPTION_INVALID_DATA_SIZE` (19) - Invalid data size
- `SIMCONNECT_EXCEPTION_DATA_ERROR` (20) - Data error
- `SIMCONNECT_EXCEPTION_INVALID_ARRAY` (21) - Invalid array
- `SIMCONNECT_EXCEPTION_CREATE_OBJECT_FAILED` (22) - Create object failed
- `SIMCONNECT_EXCEPTION_LOAD_FLIGHTPLAN_FAILED` (23) - Load flight plan failed
- `SIMCONNECT_EXCEPTION_OPERATION_INVALID_FOR_OBJECT_TYPE` (24) - Operation invalid for object type
- `SIMCONNECT_EXCEPTION_ILLEGAL_OPERATION` (25) - Illegal operation
- `SIMCONNECT_EXCEPTION_ALREADY_SUBSCRIBED` (26) - Already subscribed
- `SIMCONNECT_EXCEPTION_INVALID_ENUM` (27) - Invalid enumeration
- `SIMCONNECT_EXCEPTION_DEFINITION_ERROR` (28) - Definition error
- `SIMCONNECT_EXCEPTION_DUPLICATE_ID` (29) - Duplicate ID
- `SIMCONNECT_EXCEPTION_DATUM_ID` (30) - Datum ID error
- `SIMCONNECT_EXCEPTION_OUT_OF_BOUNDS` (31) - Out of bounds
- `SIMCONNECT_EXCEPTION_ALREADY_CREATED` (32) - Already created
- `SIMCONNECT_EXCEPTION_OBJECT_OUTSIDE_REALITY_BUBBLE` (33) - Object outside reality bubble
- `SIMCONNECT_EXCEPTION_OBJECT_CONTAINER` (34) - Object container error
- `SIMCONNECT_EXCEPTION_OBJECT_AI` (35) - AI object error
- `SIMCONNECT_EXCEPTION_OBJECT_ATC` (36) - ATC object error
- `SIMCONNECT_EXCEPTION_OBJECT_SCHEDULE` (37) - Object schedule error

**Usage:**
```go
if msg.IsException() {
    if exc, ok := msg.GetException(); ok {
        switch exc.DwException {
        case types.SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED:
            fmt.Printf("Unrecognized name in request %d\n", exc.DwSendID)
        case types.SIMCONNECT_EXCEPTION_DATA_ERROR:
            fmt.Printf("Data error in request %d, parameter %d\n", 
                exc.DwSendID, exc.DwIndex)
        default:
            fmt.Printf("SimConnect exception %d in request %d\n", 
                exc.DwException, exc.DwSendID)
        }
    }
}
```

## Helper Methods

The `ParsedMessage` struct provides convenient helper methods for type checking and data access:

### Type Checking Methods

```go
// Core message types
msg.IsOpen()         // Connection opened
msg.IsQuit()         // Connection closed
msg.IsException()    // Error/exception occurred

// Data messages
msg.IsSimObjectData() // Simulation object data
msg.IsClientData()    // Client data area data

// Event messages
msg.IsEvent()         // Standard event
msg.IsEventEX1()      // Extended event with parameters

// Facility messages
msg.IsFacilityData()  // Facility data
msg.IsAirportList()   // Airport list

// Input events
msg.IsInputEvent()    // Any input event type

// Multiplayer events
msg.IsMultiplayerEvent() // Multiplayer session events
```

### Data Access Methods

```go
// Safe casting with boolean return
data, ok := msg.GetSimObjectData()
event, ok := msg.GetEvent()
exc, ok := msg.GetException()
open, ok := msg.GetOpen()
quit, ok := msg.GetQuit()

// Extended types
eventEX1, ok := msg.GetEventEX1()
clientData, ok := msg.GetClientData()
facilityData, ok := msg.GetFacilityData()
airportList, ok := msg.GetAirportList()
vorList, ok := msg.GetVORList()
ndbList, ok := msg.GetNDBList()
waypointList, ok := msg.GetWaypointList()
controllersList, ok := msg.GetControllersList()
inputEvents, ok := msg.GetEnumerateInputEvents()
reservedKey, ok := msg.GetReservedKey()
```

### Message Processing Pattern

```go
func processMessage(msg ParsedMessage) {
    // Check for parsing errors first
    if msg.Error != nil {
        log.Printf("Message parsing error: %v", msg.Error)
        return
    }
    
    // Handle different message types
    switch {
    case msg.IsException():
        handleException(msg)
    case msg.IsSimObjectData():
        handleSimObjectData(msg)
    case msg.IsEvent():
        handleEvent(msg)
    case msg.IsOpen():
        handleConnectionOpen(msg)
    case msg.IsQuit():
        handleConnectionClosed(msg)
    default:
        log.Printf("Unhandled message type: %d", msg.MessageType)
    }
}

func handleException(msg ParsedMessage) {
    if exc, ok := msg.GetException(); ok {
        // Handle exception based on type
        logException(exc)
    }
}

func handleSimObjectData(msg ParsedMessage) {
    if data, ok := msg.GetSimObjectData(); ok {
        // Process simulation data
        processSimData(data)
    }
}
```

This comprehensive message type reference enables efficient handling of all SimConnect communication patterns in your Go applications.
