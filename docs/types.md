# Types Reference

The `types` package contains all SimConnect data types, structures, and enumerations used for communication with the simulator.

## Table of Contents

• [Data Types](#data-types)  
• [Message Structures](#message-structures)  
• [Enumerations](#enumerations)  
• [Helper Structures](#helper-structures)

## Data Types

### `SIMCONNECT_DATATYPE`

Enumeration defining the data types that can be used with SimConnect.

```go
type SIMCONNECT_DATATYPE uint32

const (
    SIMCONNECT_DATATYPE_INVALID     SIMCONNECT_DATATYPE = iota
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
```

**Usage Example:**
```go
err := client.AddToDataDefinition(
    1,                                    // Definition ID
    "PLANE ALTITUDE",                     // SimVar name
    "feet",                              // Units
    types.SIMCONNECT_DATATYPE_FLOAT64,   // Data type
    0.0,                                 // Epsilon
    0,                                   // Datum ID
)
```

## Message Structures

### `SIMCONNECT_RECV`

Base structure for all SimConnect messages.

```go
type SIMCONNECT_RECV struct {
    DwSize    uint32             // Size of the structure
    DwVersion uint32             // Version of SimConnect, matches SDK
    DwID      SIMCONNECT_RECV_ID // Message ID
}
```

### `SIMCONNECT_RECV_SIMOBJECT_DATA`

Structure for simulation object data messages.

```go
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
```

### `SIMCONNECT_RECV_EVENT`

Structure for event messages from SimConnect.

```go
type SIMCONNECT_RECV_EVENT struct {
    SIMCONNECT_RECV        // Inherits from base structure
    UGroupID        uint32 // ID of the client defined group
    UEventID        uint32 // ID of the client defined event
    DwData          uint32 // Event data - usually zero, but some events require additional qualification
}
```

### `SIMCONNECT_RECV_EXCEPTION`

Structure for exception information from SimConnect.

```go
type SIMCONNECT_RECV_EXCEPTION struct {
    SIMCONNECT_RECV        // Inherits from base structure
    DwException     uint32 // Exception code
    DwSendID        uint32 // ID of the packet that caused the exception
    DwIndex         uint32 // Index number for some exceptions
}
```

## Enumerations

### `SIMCONNECT_RECV_ID`

Enumeration defining all possible message types that can be received from SimConnect.

```go
type SIMCONNECT_RECV_ID uint32

const (
    SIMCONNECT_RECV_ID_NULL                             SIMCONNECT_RECV_ID = iota
    SIMCONNECT_RECV_ID_EXCEPTION                        // Exception information
    SIMCONNECT_RECV_ID_OPEN                             // Connection established
    SIMCONNECT_RECV_ID_QUIT                             // Connection closed
    SIMCONNECT_RECV_ID_EVENT                            // Event information
    SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE           // Object added or removed
    SIMCONNECT_RECV_ID_EVENT_FILENAME                   // Filename event
    SIMCONNECT_RECV_ID_EVENT_FRAME                      // Frame event
    SIMCONNECT_RECV_ID_SIMOBJECT_DATA                   // SimObject data
    SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE            // SimObject data by type
    SIMCONNECT_RECV_ID_WEATHER_OBSERVATION              // Weather observation
    SIMCONNECT_RECV_ID_CLOUD_STATE                      // Cloud state
    SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID               // Assigned object ID
    SIMCONNECT_RECV_ID_RESERVED_KEY                     // Reserved key
    SIMCONNECT_RECV_ID_CUSTOM_ACTION                    // Custom action
    SIMCONNECT_RECV_ID_SYSTEM_STATE                     // System state
    SIMCONNECT_RECV_ID_CLIENT_DATA                      // Client data
    // ... additional message types
)
```

### `SIMCONNECT_PERIOD`

Defines how often data is sent from SimConnect.

```go
type SIMCONNECT_PERIOD uint32

const (
    SIMCONNECT_PERIOD_NEVER                SIMCONNECT_PERIOD = iota
    SIMCONNECT_PERIOD_ONCE                 // Send data once
    SIMCONNECT_PERIOD_VISUAL_FRAME         // Send data every visual frame
    SIMCONNECT_PERIOD_SIM_FRAME            // Send data every simulation frame
    SIMCONNECT_PERIOD_SECOND               // Send data every second
)
```

### `SIMCONNECT_SIMOBJECT_TYPE`

Defines the type of simulation object.

```go
type SIMCONNECT_SIMOBJECT_TYPE uint32

const (
    SIMCONNECT_SIMOBJECT_TYPE_USER         SIMCONNECT_SIMOBJECT_TYPE = iota
    SIMCONNECT_SIMOBJECT_TYPE_ALL          // All objects
    SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT     // Aircraft objects
    SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER   // Helicopter objects
    SIMCONNECT_SIMOBJECT_TYPE_BOAT         // Boat objects
    SIMCONNECT_SIMOBJECT_TYPE_GROUND       // Ground objects
)
```

### `SIMCONNECT_DATA_REQUEST_FLAG`

Flags for data requests.

```go
type SIMCONNECT_DATA_REQUEST_FLAG uint32

const (
    SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT   SIMCONNECT_DATA_REQUEST_FLAG = 0x00000000
    SIMCONNECT_DATA_REQUEST_FLAG_CHANGED   SIMCONNECT_DATA_REQUEST_FLAG = 0x00000001  // Send data only when changed
    SIMCONNECT_DATA_REQUEST_FLAG_TAGGED    SIMCONNECT_DATA_REQUEST_FLAG = 0x00000002  // Include tagged data
)
```

### `SIMCONNECT_DATA_SET_FLAG`

Flags for setting data.

```go
type SIMCONNECT_DATA_SET_FLAG uint32

const (
    SIMCONNECT_DATA_SET_FLAG_DEFAULT      SIMCONNECT_DATA_SET_FLAG = 0x00000000
    SIMCONNECT_DATA_SET_FLAG_TAGGED       SIMCONNECT_DATA_SET_FLAG = 0x00000001
)
```

## Helper Structures

### `SIMCONNECT_DATA_INITPOSITION`

Structure for initializing aircraft position.

```go
type SIMCONNECT_DATA_INITPOSITION struct {
    Latitude  float64 // Latitude in degrees
    Longitude float64 // Longitude in degrees
    Altitude  float64 // Altitude in feet
    Pitch     float64 // Pitch in degrees
    Bank      float64 // Bank in degrees
    Heading   float64 // Heading in degrees
    OnGround  uint32  // Set to 1 for on ground, 0 for airborne
    Airspeed  uint32  // Airspeed in knots or special values
}
```

**Special airspeed values:**
- `INITPOSITION_AIRSPEED_CRUISE (-1)`: Use cruise airspeed
- `INITPOSITION_AIRSPEED_KEEP (-2)`: Keep current airspeed

### `SIMCONNECT_DATA_LATLONALT`

Structure for world position data.

```go
type SIMCONNECT_DATA_LATLONALT struct {
    Latitude  float64 // Latitude in degrees
    Longitude float64 // Longitude in degrees
    Altitude  float64 // Altitude in feet
}
```

### `SIMCONNECT_DATA_XYZ`

Structure for 3D coordinate data.

```go
type SIMCONNECT_DATA_XYZ struct {
    X float64 // X coordinate
    Y float64 // Y coordinate
    Z float64 // Z coordinate
}
```

## Usage Examples

### Working with Data Definitions

```go
// Add multiple data points to a definition
err := client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
err = client.AddToDataDefinition(1, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
err = client.AddToDataDefinition(1, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)

// Request data periodically
err = client.RequestDataOnSimObject(
    1,                                              // Request ID
    1,                                              // Definition ID
    0,                                              // Object ID (user aircraft)
    types.SIMCONNECT_PERIOD_SIM_FRAME,             // Period
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,    // Flags
    0,                                              // Origin
    0,                                              // Interval
    0,                                              // Limit
)
```

### Handling Different Message Types

```go
for message := range client.Stream() {
    switch message.MessageType {
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
        if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
            fmt.Printf("Received data for request %d\n", data.DwRequestID)
        }
    case types.SIMCONNECT_RECV_ID_EVENT:
        if event, ok := message.Data.(*types.SIMCONNECT_RECV_EVENT); ok {
            fmt.Printf("Received event %d with data %d\n", event.UEventID, event.DwData)
        }
    case types.SIMCONNECT_RECV_ID_EXCEPTION:
        if exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION); ok {
            fmt.Printf("Exception %d occurred\n", exception.DwException)
        }
    }
}
```

## Type Safety

All types in this package are designed to match the official SimConnect SDK types exactly. When working with raw data from SimConnect, always use the appropriate type casting and check for proper data sizes to ensure memory safety.

```go
// Safe type casting example
if message.IsSimObjectData() {
    if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
        // Safely work with the data
        fmt.Printf("Object ID: %d, Define ID: %d\n", data.DwObjectID, data.DwDefineID)
    }
}
```
