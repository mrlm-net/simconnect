# API Reference

This document provides a complete reference for all public APIs in the SimConnect Go library.

## Table of Contents

- [Client Creation](#client-creation)
- [Connection Management](#connection-management)
- [Data Definition and Requests](#data-definition-and-requests)
- [Event Management](#event-management)
- [Message Processing](#message-processing)
- [Input Events](#input-events)
- [Client Data](#client-data)
- [Facility Data](#facility-data)
- [System Information](#system-information)
- [Error Handling](#error-handling)

## Client Creation

### `client.New(name string) *Engine`

Creates a new SimConnect client instance with the default DLL path.

**Parameters:**
- `name` - Application name displayed in SimConnect

**Returns:**
- `*Engine` - Client instance, or `nil` if initialization fails

**Example:**
```go
sc := client.New("My Flight Sim App")
if sc == nil {
    log.Fatal("Failed to create client")
}
```

### `client.NewWithDLL(name string, dllPath string) *Engine`

Creates a new SimConnect client instance with a custom DLL path.

**Parameters:**
- `name` - Application name displayed in SimConnect
- `dllPath` - Path to SimConnect.dll

**Returns:**
- `*Engine` - Client instance, or `nil` if initialization fails

**Example:**
```go
sc := client.NewWithDLL("My App", "C:/Custom/Path/SimConnect.dll")
```

## Connection Management

### `Connect() error`

Establishes a connection to the flight simulator.

**Returns:**
- `error` - Connection error, or `nil` on success

**Example:**
```go
if err := sc.Connect(); err != nil {
    log.Fatal("Connection failed:", err)
}
```

### `Disconnect() error`

Closes the SimConnect connection and performs cleanup. This method is safe to call multiple times.

**Returns:**
- `error` - Disconnection error, or `nil` on success

**Example:**
```go
defer sc.Disconnect()
```

### `Shutdown() error`

Alias for `Disconnect()` - triggers graceful shutdown.

**Returns:**
- `error` - Shutdown error, or `nil` on success

## Data Definition and Requests

### `AddToDataDefinition(defineID int, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID int) error`

Adds a simulation variable to a data definition.

**Parameters:**
- `defineID` - Unique identifier for the data definition
- `datumName` - Name of the simulation variable (e.g., "PLANE ALTITUDE")
- `unitsName` - Units for the variable (e.g., "feet", "knots")
- `datumType` - Data type from `types.SIMCONNECT_DATATYPE_*`
- `epsilon` - Minimum change threshold for updates
- `datumID` - Unique identifier within the definition

**Returns:**
- `error` - Definition error, or `nil` on success

**Example:**
```go
err := sc.AddToDataDefinition(
    1,                                     // Definition ID
    "PLANE ALTITUDE",                      // Variable name
    "feet",                               // Units
    types.SIMCONNECT_DATATYPE_FLOAT64,   // Data type
    0.0,                                  // Epsilon
    0,                                    // Datum ID
)
```

### `RequestDataOnSimObject(request int, definition int, object int, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin int, interval int, limit int) error`

Requests data updates for a simulation object.

**Parameters:**
- `request` - Unique request identifier
- `definition` - Data definition ID (from `AddToDataDefinition`)
- `object` - Object ID (typically `types.SIMCONNECT_OBJECT_ID_USER` for user aircraft)
- `period` - Update frequency from `types.SIMCONNECT_PERIOD_*`
- `flags` - Request flags from `types.SIMCONNECT_DATA_REQUEST_FLAG_*`
- `origin` - Starting point for periodic updates
- `interval` - Interval between updates (frames or seconds)
- `limit` - Maximum number of updates (0 = unlimited)

**Returns:**
- `error` - Request error, or `nil` on success

**Example:**
```go
err := sc.RequestDataOnSimObject(
    1,                                        // Request ID
    1,                                        // Definition ID
    types.SIMCONNECT_OBJECT_ID_USER,         // User aircraft
    types.SIMCONNECT_PERIOD_SIM_FRAME,       // Every frame
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, // Only when changed
    0, 1, 0,                                 // Origin, interval, limit
)
```

### `RequestDataOnSimObjectType(request int, definition int, radius string, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error`

Requests data for all objects of a specific type within a radius.

**Parameters:**
- `request` - Unique request identifier
- `definition` - Data definition ID
- `radius` - Search radius (e.g., "10000" for 10km)
- `objectType` - Object type from `types.SIMCONNECT_SIMOBJECT_TYPE_*`

**Returns:**
- `error` - Request error, or `nil` on success

**Example:**
```go
err := sc.RequestDataOnSimObjectType(
    2,                                    // Request ID
    1,                                    // Definition ID
    "5000",                              // 5km radius
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT, // Aircraft objects
)
```

### `SetDataOnSimObject(definition int, object int, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount int, unitSize int, data uintptr) error`

Sets data on a simulation object.

**Parameters:**
- `definition` - Data definition ID
- `object` - Target object ID
- `flags` - Set operation flags
- `arrayCount` - Number of elements in data array
- `unitSize` - Size of each data element
- `data` - Pointer to data buffer

**Returns:**
- `error` - Set operation error, or `nil` on success

## Event Management

### `MapClientEventToSimEvent(id int, event string) error`

Maps a client-defined event ID to a simulator event name.

**Parameters:**
- `id` - Client event ID
- `event` - Simulator event name (e.g., "GEAR_TOGGLE")

**Returns:**
- `error` - Mapping error, or `nil` on success

**Example:**
```go
err := sc.MapClientEventToSimEvent(1, "GEAR_TOGGLE")
```

### `TransmitClientEvent(object int, event int, data int, group int) error`

Transmits an event to the simulator.

**Parameters:**
- `object` - Target object ID
- `event` - Client event ID (mapped with `MapClientEventToSimEvent`)
- `data` - Event parameter data
- `group` - Notification group ID

**Returns:**
- `error` - Transmission error, or `nil` on success

**Example:**
```go
err := sc.TransmitClientEvent(
    types.SIMCONNECT_OBJECT_ID_USER, // User aircraft
    1,                               // Event ID
    0,                               // No additional data
    0,                               // Default group
)
```

### `TransmitClientEvent_EX1(object int, event int, group int, flags int, param0, param1, param2, param3, param4 int) error`

Transmits an event with up to five parameters.

**Parameters:**
- `object` - Target object ID
- `event` - Client event ID
- `group` - Notification group ID
- `flags` - Event flags
- `param0-4` - Event parameters

**Returns:**
- `error` - Transmission error, or `nil` on success

### `AddClientEventToNotificationGroup(group int, event int) error`

Adds a client event to a notification group.

**Parameters:**
- `group` - Notification group ID
- `event` - Client event ID

**Returns:**
- `error` - Addition error, or `nil` on success

### `SetNotificationGroupPriority(group int, priority int) error`

Sets the priority of a notification group.

**Parameters:**
- `group` - Notification group ID
- `priority` - Priority level (higher numbers = higher priority)

**Returns:**
- `error` - Priority setting error, or `nil` on success

### `RemoveClientEvent(group int, event int) error`

Removes a client event from a notification group.

**Parameters:**
- `group` - Notification group ID
- `event` - Client event ID

**Returns:**
- `error` - Removal error, or `nil` on success

### `ClearNotificationGroup(group int) error`

Removes all events from a notification group.

**Parameters:**
- `group` - Notification group ID

**Returns:**
- `error` - Clear operation error, or `nil` on success

### `RequestNotificationGroup(group int, reserved int) error`

Requests events from a notification group when the simulation is in Dialog Mode.

**Parameters:**
- `group` - Notification group ID
- `reserved` - Reserved parameter (should be 0)

**Returns:**
- `error` - Request error, or `nil` on success

## Message Processing

### `Stream() <-chan ParsedMessage`

Starts message processing and returns a read-only channel for receiving parsed messages. This method starts message processing in a background goroutine and returns immediately.

**Returns:**
- `<-chan ParsedMessage` - Channel for receiving messages

**Example:**
```go
messageStream := sc.Stream()
for msg := range messageStream {
    if msg.IsSimObjectData() {
        data, ok := msg.GetSimObjectData()
        if ok {
            // Process sim object data
        }
    }
}
```

### `DispatchProc(callback types.DispatchProc, context uintptr) error`

Sets up custom message dispatch processing.

**Parameters:**
- `callback` - Dispatch callback function
- `context` - User context pointer

**Returns:**
- `error` - Dispatch setup error, or `nil` on success

## Input Events

### `MapInputEventToClientEvent_EX1(group int, inputDefinition string, clientEventDownID int, downValue int, clientEventUpID int, upValue int, maskable int) error`

Maps an input event to client events.

**Parameters:**
- `group` - Input group ID
- `inputDefinition` - Input event definition string
- `clientEventDownID` - Client event ID for down action
- `downValue` - Value for down action
- `clientEventUpID` - Client event ID for up action
- `upValue` - Value for up action
- `maskable` - Whether the event can be masked

**Returns:**
- `error` - Mapping error, or `nil` on success

### `RequestInputEventInformation(requestID int, hash uint64) error`

Requests information about a specific input event.

**Parameters:**
- `requestID` - Request identifier
- `hash` - Input event hash

**Returns:**
- `error` - Request error, or `nil` on success

### `EnumerateInputEvents(requestID int) error`

Enumerates all available input events.

**Parameters:**
- `requestID` - Request identifier

**Returns:**
- `error` - Enumeration error, or `nil` on success

### `GetInputEvent(requestID int, hash uint64) error`

Gets information about a specific input event by hash.

**Parameters:**
- `requestID` - Request identifier
- `hash` - Input event hash

**Returns:**
- `error` - Get operation error, or `nil` on success

### `SetInputEvent(hash uint64, value string) error`

Sets the value of an input event.

**Parameters:**
- `hash` - Input event hash
- `value` - New value as string

**Returns:**
- `error` - Set operation error, or `nil` on success

### `SubscribeInputEvent(hash uint64) error`

Subscribes to notifications for an input event.

**Parameters:**
- `hash` - Input event hash

**Returns:**
- `error` - Subscription error, or `nil` on success

### `UnsubscribeInputEvent(hash uint64) error`

Unsubscribes from notifications for an input event.

**Parameters:**
- `hash` - Input event hash

**Returns:**
- `error` - Unsubscription error, or `nil` on success

### `EnumerateInputEventParams(hash uint64) error`

Enumerates parameters for a specific input event.

**Parameters:**
- `hash` - Input event hash

**Returns:**
- `error` - Enumeration error, or `nil` on success

## Client Data

### `MapClientDataNameToID(name string, id int) error`

Maps a client data area name to an ID.

**Parameters:**
- `name` - Client data area name
- `id` - Client data ID

**Returns:**
- `error` - Mapping error, or `nil` on success

### `CreateClientData(id int, size int, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error`

Creates a client data area.

**Parameters:**
- `id` - Client data ID
- `size` - Size in bytes
- `flags` - Creation flags

**Returns:**
- `error` - Creation error, or `nil` on success

### `AddToClientDataDefinition(defineID int, offset int, sizeOrType int, epsilon float32, datumID int) error`

Adds a data element to a client data definition.

**Parameters:**
- `defineID` - Definition ID
- `offset` - Byte offset within client data
- `sizeOrType` - Size in bytes or data type
- `epsilon` - Change threshold
- `datumID` - Element ID

**Returns:**
- `error` - Addition error, or `nil` on success

### `RequestClientData(clientDataID int, requestID int, defineID int, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin int, interval int, limit int) error`

Requests client data updates.

**Parameters:**
- `clientDataID` - Client data area ID
- `requestID` - Request ID
- `defineID` - Data definition ID
- `period` - Update period
- `flags` - Request flags
- `origin` - Starting point
- `interval` - Update interval
- `limit` - Maximum updates

**Returns:**
- `error` - Request error, or `nil` on success

### `SetClientData(clientDataID int, defineID int, flags types.SIMCONNECT_CLIENT_DATA_SET_FLAG, reserved int, size int, data uintptr) error`

Sets client data values.

**Parameters:**
- `clientDataID` - Client data area ID
- `defineID` - Data definition ID
- `flags` - Set flags
- `reserved` - Reserved parameter
- `size` - Data size
- `data` - Data pointer

**Returns:**
- `error` - Set operation error, or `nil` on success

### `ClearClientDataDefinition(defineID int) error`

Clears a client data definition.

**Parameters:**
- `defineID` - Definition ID to clear

**Returns:**
- `error` - Clear operation error, or `nil` on success

## Facility Data

### `RequestFacilitiesList(type_ types.SIMCONNECT_FACILITY_LIST_TYPE, requestID int) error`

Requests a list of facilities of a specific type.

**Parameters:**
- `type_` - Facility type (airport, VOR, NDB, waypoint)
- `requestID` - Request identifier

**Returns:**
- `error` - Request error, or `nil` on success

### `RequestFacilitiesList_EX1(type_ types.SIMCONNECT_FACILITY_LIST_TYPE, requestID int, ident string, region string) error`

Requests facilities with filtering by identifier and region.

**Parameters:**
- `type_` - Facility type
- `requestID` - Request identifier
- `ident` - Facility identifier filter
- `region` - Region filter

**Returns:**
- `error` - Request error, or `nil` on success

### `RequestFacilityData(type_ types.SIMCONNECT_FACILITY_DATA_TYPE, requestID int, ident string, region string) error`

Requests detailed data for a specific facility.

**Parameters:**
- `type_` - Facility data type
- `requestID` - Request identifier
- `ident` - Facility identifier
- `region` - Facility region

**Returns:**
- `error` - Request error, or `nil` on success

### `RequestFacilityData_EX1(type_ types.SIMCONNECT_FACILITY_DATA_TYPE, requestID int, ident string, region string, icao string) error`

Requests facility data with ICAO code specification.

**Parameters:**
- `type_` - Facility data type
- `requestID` - Request identifier
- `ident` - Facility identifier
- `region` - Facility region
- `icao` - ICAO code

**Returns:**
- `error` - Request error, or `nil` on success

### `SubscribeToFacilities(type_ types.SIMCONNECT_FACILITY_LIST_TYPE, requestID int) error`

Subscribes to facility list updates.

**Parameters:**
- `type_` - Facility type
- `requestID` - Request identifier

**Returns:**
- `error` - Subscription error, or `nil` on success

### `SubscribeToFacilities_EX1(type_ types.SIMCONNECT_FACILITY_LIST_TYPE, requestID int, ident string, region string) error`

Subscribes to facility updates with filtering.

**Parameters:**
- `type_` - Facility type
- `requestID` - Request identifier
- `ident` - Facility identifier filter
- `region` - Region filter

**Returns:**
- `error` - Subscription error, or `nil` on success

### `UnsubscribeToFacilities(type_ types.SIMCONNECT_FACILITY_LIST_TYPE) error`

Unsubscribes from facility list updates.

**Parameters:**
- `type_` - Facility type

**Returns:**
- `error` - Unsubscription error, or `nil` on success

## System Information

### `RequestSystemState(requestID int, state string) error`

Requests system state information.

**Parameters:**
- `requestID` - Request identifier
- `state` - State name (e.g., "Sim", "DialogMode", "Flight")

**Returns:**
- `error` - Request error, or `nil` on success

### `SetSystemState(state string, integer int, float_ float32, string_ string) error`

Sets system state values.

**Parameters:**
- `state` - State name
- `integer` - Integer value
- `float_` - Float value
- `string_` - String value

**Returns:**
- `error` - Set operation error, or `nil` on success

### `RequestDataOnSimObjectType(request int, definition int, radius string, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error`

Requests data for objects within a radius.

**Parameters:**
- `request` - Request ID
- `definition` - Data definition ID
- `radius` - Search radius
- `objectType` - Object type filter

**Returns:**
- `error` - Request error, or `nil` on success

## Error Handling

All API methods return an error value that should be checked. The library provides detailed error messages including HRESULT codes from the underlying SimConnect API.

**Example Error Handling:**
```go
if err := sc.Connect(); err != nil {
    switch {
    case strings.Contains(err.Error(), "0x80040108"):
        log.Println("Simulator not running")
    case strings.Contains(err.Error(), "0x80040109"):
        log.Println("Connection limit reached")
    default:
        log.Printf("Connection error: %v", err)
    }
}
```

## Constants and Types

See [Data Types](data-types.md) for complete reference of all available constants and type definitions.
