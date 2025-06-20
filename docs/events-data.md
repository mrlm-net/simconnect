# Events & Data Reference

This document covers working with SimConnect events and simulation data using the client API.

## Table of Contents

• [Data Definitions](#data-definitions)  
• [Requesting Data](#requesting-data)  
• [Setting Data](#setting-data)  
• [Event Handling](#event-handling)  
• [Event Groups](#event-groups)  
• [Common Patterns](#common-patterns)

## Data Definitions

Data definitions tell SimConnect what simulation variables you want to monitor and how they should be formatted.

### `AddToDataDefinition(defineID, datumName, unitsName, datumType, epsilon, datumID) error`

Adds a simulation variable to a data definition.

**Parameters:**
- `defineID` (int): Unique identifier for the data definition
- `datumName` (string): Name of the simulation variable (e.g., "PLANE ALTITUDE")
- `unitsName` (string): Units for the data (e.g., "feet", "knots", "degrees")
- `datumType` (types.SIMCONNECT_DATATYPE): Data type for the variable
- `epsilon` (float32): Minimum change required to trigger an update (0.0 for all changes)
- `datumID` (int): Unique identifier for this datum within the definition

**Example:**
```go
// Define aircraft basic data
err := client.AddToDataDefinition(
    1,                                    // Definition ID
    "PLANE ALTITUDE",                     // SimVar name
    "feet",                              // Units
    types.SIMCONNECT_DATATYPE_FLOAT64,   // Data type
    0.0,                                 // Epsilon (report all changes)
    0,                                   // Datum ID
)

err = client.AddToDataDefinition(
    1,                                    // Same definition ID
    "AIRSPEED INDICATED",                 // SimVar name
    "knots",                             // Units
    types.SIMCONNECT_DATATYPE_FLOAT64,   // Data type
    0.5,                                 // Epsilon (only report changes > 0.5 knots)
    1,                                   // Datum ID
)
```

### `ClearDataDefinition(definition) error`

Clears all data from a data definition.

**Parameters:**
- `definition` (int): The definition ID to clear

**Example:**
```go
err := client.ClearDataDefinition(1)
```

## Requesting Data

Once you've defined what data you want, you can request it from SimConnect.

### `RequestDataOnSimObject(request, definition, object, period, flags, origin, interval, limit) error`

Requests data for a specific simulation object.

**Parameters:**
- `request` (int): Unique identifier for this request
- `definition` (int): The data definition ID to use
- `object` (int): Object ID (0 for user aircraft)
- `period` (types.SIMCONNECT_PERIOD): How often to send data
- `flags` (types.SIMCONNECT_DATA_REQUEST_FLAG): Request flags
- `origin` (int): Starting point for the request
- `interval` (int): Interval between requests
- `limit` (int): Maximum number of responses

**Example:**
```go
// Request aircraft data every simulation frame, only when changed
err := client.RequestDataOnSimObject(
    1,                                              // Request ID
    1,                                              // Definition ID
    0,                                              // Object ID (user aircraft)
    types.SIMCONNECT_PERIOD_SIM_FRAME,             // Every sim frame
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,    // Only when changed
    0,                                              // Origin
    0,                                              // Interval
    0,                                              // Limit (unlimited)
)
```

### `RequestDataOnSimObjectType(request, definition, radius, objectType) error`

Requests data for all objects of a specific type within a radius.

**Parameters:**
- `request` (int): Unique identifier for this request
- `definition` (int): The data definition ID to use
- `radius` (string): Search radius (e.g., "10000" for 10km)
- `objectType` (types.SIMCONNECT_SIMOBJECT_TYPE): Type of objects to find

**Example:**
```go
// Find all aircraft within 50 nautical miles
err := client.RequestDataOnSimObjectType(
    2,                                        // Request ID
    2,                                        // Definition ID
    "50000",                                  // 50000 meters radius
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT, // Aircraft only
)
```

## Setting Data

You can also send data to the simulator to change simulation variables.

### `SetDataOnSimObject(definition, object, flags, arrayCount, unitSize, data) error`

Sets data on a simulation object.

**Parameters:**
- `definition` (int): The data definition ID
- `object` (int): Object ID (0 for user aircraft)
- `flags` (types.SIMCONNECT_DATA_SET_FLAG): Set flags
- `arrayCount` (int): Number of elements in data array
- `unitSize` (int): Size of each data unit
- `data` (uintptr): Pointer to the data to set

**Example:**
```go
import "unsafe"

// Set aircraft altitude to 5000 feet
newAltitude := float64(5000.0)
err := client.SetDataOnSimObject(
    1,                                           // Definition ID
    0,                                           // Object ID (user aircraft)
    types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,     // Flags
    0,                                           // Array count (single value)
    int(unsafe.Sizeof(newAltitude)),            // Size of float64
    uintptr(unsafe.Pointer(&newAltitude)),      // Data pointer
)
```

## Event Handling

Events allow you to trigger simulator actions and receive notifications.

### `MapClientEventToSimEvent(id, event) error`

Maps a client event ID to a simulator event name.

**Parameters:**
- `id` (int): Client-defined event ID
- `event` (string): SimConnect event name

**Example:**
```go
// Map event ID 1 to external power toggle
err := client.MapClientEventToSimEvent(1, "TOGGLE_EXTERNAL_POWER")

// Map event ID 2 to gear toggle
err := client.MapClientEventToSimEvent(2, "GEAR_TOGGLE")
```

### `TransmitClientEvent(object, event, data, group) error`

Transmits an event to the simulator.

**Parameters:**
- `object` (int): Object ID to send event to (0 for user aircraft)
- `event` (int): Client event ID
- `data` (int): Event data parameter
- `group` (int): Event group ID

**Example:**
```go
// Toggle external power
err := client.TransmitClientEvent(
    0, // User aircraft
    1, // Event ID (mapped to TOGGLE_EXTERNAL_POWER)
    0, // No additional data
    1, // Group ID
)
```

## Event Groups

Event groups help organize and prioritize events.

### `AddClientEventToNotificationGroup(group, event) error`

Adds an event to a notification group to receive notifications when it occurs.

**Parameters:**
- `group` (int): Group ID
- `event` (int): Event ID to add to the group

**Example:**
```go
// Add external power event to group 1
err := client.AddClientEventToNotificationGroup(1, 1)
```

### `SetNotificationGroupPriority(group, priority) error`

Sets the priority level for a notification group.

**Parameters:**
- `group` (int): Group ID
- `priority` (int): Priority level (higher numbers = higher priority)

**Example:**
```go
// Set high priority for critical events
err := client.SetNotificationGroupPriority(1, 1000)
```

## Common Patterns

### Basic Aircraft Monitoring

```go
package main

import (
    "fmt"
    "log"
    "unsafe"

    "github.com/mrlm-net/simconnect/pkg/client"
    "github.com/mrlm-net/simconnect/pkg/types"
)

// AircraftData represents basic aircraft telemetry
type AircraftData struct {
    Altitude float64 // feet
    Airspeed float64 // knots
    Heading  float64 // degrees
}

func main() {
    simClient := client.New("AircraftMonitor")
    if simClient == nil {
        log.Fatal("Failed to create client")
    }

    if err := simClient.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    defer simClient.Disconnect()

    // Define aircraft data structure
    defineID := 1
    err := simClient.AddToDataDefinition(defineID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    if err != nil {
        log.Fatal("Failed to add altitude:", err)
    }

    err = simClient.AddToDataDefinition(defineID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
    if err != nil {
        log.Fatal("Failed to add airspeed:", err)
    }

    err = simClient.AddToDataDefinition(defineID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
    if err != nil {
        log.Fatal("Failed to add heading:", err)
    }

    // Request data updates
    requestID := 1
    err = simClient.RequestDataOnSimObject(
        requestID,
        defineID,
        0, // User aircraft
        types.SIMCONNECT_PERIOD_SIM_FRAME,
        types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
        0, 0, 0,
    )
    if err != nil {
        log.Fatal("Failed to request data:", err)
    }

    // Process messages
    for message := range simClient.Stream() {
        if message.Error != nil {
            fmt.Printf("Error: %v\n", message.Error)
            continue
        }

        if message.IsSimObjectData() {
            if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
                if data.DwRequestID == uint32(requestID) {
                    // Parse the aircraft data from raw bytes
                    dataPtr := uintptr(unsafe.Pointer(&message.RawData[0])) + unsafe.Offsetof(data.DwData)
                    aircraftData := (*AircraftData)(unsafe.Pointer(dataPtr + unsafe.Sizeof(data.DwData)))
                    
                    fmt.Printf("Altitude: %.1f ft, Airspeed: %.1f kts, Heading: %.1f°\n",
                        aircraftData.Altitude, aircraftData.Airspeed, aircraftData.Heading)
                }
            }
        }
    }
}
```

### Event-Driven Control

```go
package main

import (
    "fmt"
    "log"

    "github.com/mrlm-net/simconnect/pkg/client"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    EVENT_EXTERNAL_POWER = 1
    GROUP_POWER         = 1
)

func main() {
    simClient := client.New("PowerController")
    if simClient == nil {
        log.Fatal("Failed to create client")
    }

    if err := simClient.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    defer simClient.Disconnect()

    // Map event
    err := simClient.MapClientEventToSimEvent(EVENT_EXTERNAL_POWER, "TOGGLE_EXTERNAL_POWER")
    if err != nil {
        log.Fatal("Failed to map event:", err)
    }

    // Add to notification group
    err = simClient.AddClientEventToNotificationGroup(GROUP_POWER, EVENT_EXTERNAL_POWER)
    if err != nil {
        log.Fatal("Failed to add to group:", err)
    }

    // Set priority
    err = simClient.SetNotificationGroupPriority(GROUP_POWER, 1000)
    if err != nil {
        log.Fatal("Failed to set priority:", err)
    }

    fmt.Println("Power controller ready. Press Enter to toggle external power...")
    
    // In a real application, you'd trigger this based on user input or other events
    go func() {
        var input string
        for {
            fmt.Scanln(&input)
            err := simClient.TransmitClientEvent(0, EVENT_EXTERNAL_POWER, 0, GROUP_POWER)
            if err != nil {
                fmt.Printf("Failed to transmit event: %v\n", err)
            } else {
                fmt.Println("External power toggled!")
            }
        }
    }()

    // Process messages
    for message := range simClient.Stream() {
        if message.Error != nil {
            fmt.Printf("Error: %v\n", message.Error)
            continue
        }

        if message.IsEvent() {
            if event, ok := message.Data.(*types.SIMCONNECT_RECV_EVENT); ok {
                fmt.Printf("Event received - Group: %d, Event: %d, Data: %d\n",
                    event.UGroupID, event.UEventID, event.DwData)
            }
        }
    }
}
```

## Best Practices

1. **Use appropriate periods**: Don't request data more frequently than needed
2. **Use epsilon values**: Only receive updates when values change significantly
3. **Group related data**: Put related variables in the same data definition
4. **Handle errors**: Always check for errors when setting up definitions and requests
5. **Use appropriate data types**: Match the expected data type for each simulation variable
6. **Clean up**: Clear data definitions when no longer needed to free resources

```go
// Good practice: Only request changes greater than 10 feet for altitude
err := client.AddToDataDefinition(
    1, "PLANE ALTITUDE", "feet", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 
    10.0, // Only report changes > 10 feet
    0,
)
```
