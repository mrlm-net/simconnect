# Engine/Client Usage

The `engine` package provides a high-level client for interacting with SimConnect. This document covers the complete API for direct simulator communication.

> **See also:** [Configuration Options](config-client.md) for all available options when creating a client.

## Creating a Client

### Using the Root Package

```go
import "github.com/mrlm-net/simconnect"

client := simconnect.NewClient("MyApp")
```

### Using the Engine Package Directly

```go
import "github.com/mrlm-net/simconnect/pkg/engine"

client := engine.New("MyApp")
```

## Connection Lifecycle

### Connect

Establishes a connection to the SimConnect server.

```go
if err := client.Connect(); err != nil {
    log.Fatal("Failed to connect:", err)
}
```

### Disconnect

Gracefully closes the connection and releases resources.

```go
defer client.Disconnect()
```

### Stream

Returns a read-only channel for receiving SimConnect messages.

```go
stream := client.Stream()
for msg := range stream {
    // Handle incoming messages
}
```

## Data Definitions

Data definitions describe the structure of data you want to receive from or send to the simulator.

### AddToDataDefinition

Adds a variable to a data definition. Call multiple times to build composite definitions.

```go
import "github.com/mrlm-net/simconnect/pkg/types"

// Define aircraft position data
const PositionDefID = 1000

client.AddToDataDefinition(
    PositionDefID,
    "PLANE LATITUDE",
    "degrees",
    types.SIMCONNECT_DATATYPE_FLOAT64,
    0,    // epsilon (change threshold)
    0,    // datum ID
)
client.AddToDataDefinition(
    PositionDefID,
    "PLANE LONGITUDE",
    "degrees",
    types.SIMCONNECT_DATATYPE_FLOAT64,
    0, 0,
)
client.AddToDataDefinition(
    PositionDefID,
    "PLANE ALTITUDE",
    "feet",
    types.SIMCONNECT_DATATYPE_FLOAT64,
    0, 0,
)
```

### ClearDataDefinition

Removes a previously created data definition.

```go
client.ClearDataDefinition(PositionDefID)
```

### RegisterDataset

Registers a pre-built dataset from the `datasets` package.

```go
import "github.com/mrlm-net/simconnect/pkg/datasets/aircraft"

client.RegisterDataset(PositionDefID, aircraft.Position)
```

### RegisterFacilityDataset

Registers a pre-built facility dataset from the `datasets/facilities` package. Unlike `RegisterDataset` which uses `AddToDataDefinition`, this uses `AddToFacilityDefinition` internally.

```go
import "github.com/mrlm-net/simconnect/pkg/datasets/facilities"

client.RegisterFacilityDataset(AirportDefID, facilities.NewAirportFacilityDataset())
client.RegisterFacilityDataset(RunwayDefID, facilities.NewRunwayFacilityDataset())
client.RegisterFacilityDataset(ParkingDefID, facilities.NewParkingFacilityDataset())
client.RegisterFacilityDataset(FrequencyDefID, facilities.NewFrequencyFacilityDataset())
```

Available facility dataset constructors in `pkg/datasets/facilities`:

| Constructor | Data |
|-------------|------|
| `NewAirportFacilityDataset()` | Airport info (name, ICAO, position, tower, transition) |
| `NewRunwayFacilityDataset()` | Runway dimensions, heading, surface, lighting |
| `NewParkingFacilityDataset()` | Parking spots, gates, ramps |
| `NewFrequencyFacilityDataset()` | COM/NAV frequencies |
| `NewTaxiPointFacilityDataset()` | Taxiway intersection points |
| `NewTaxiPathFacilityDataset()` | Taxiway paths and routes |
| `NewTaxiNameFacilityDataset()` | Taxiway names |
| `NewHelipadFacilityDataset()` | Helipad locations and properties |
| `NewJetwayFacilityDataset()` | Jetway data |
| `NewDepartureFacilityDataset()` | SID procedures |
| `NewApproachFacilityDataset()` | Approach procedures |
| `NewVORFacilityDataset()` | VOR navaid data |
| `NewNDBFacilityDataset()` | NDB navaid data |
| `NewWaypointFacilityDataset()` | Waypoint data |

## Requesting Data

### RequestDataOnSimObject

Requests data for a specific sim object with optional periodic updates.

```go
const PositionReqID = 1001

// Request once
client.RequestDataOnSimObject(
    PositionReqID,
    PositionDefID,
    types.SIMCONNECT_OBJECT_ID_USER,     // User aircraft
    types.SIMCONNECT_PERIOD_ONCE,        // Single request
    types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
    0, 0, 0,
)

// Request every second
client.RequestDataOnSimObject(
    PositionReqID,
    PositionDefID,
    types.SIMCONNECT_OBJECT_ID_USER,
    types.SIMCONNECT_PERIOD_SECOND,
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,  // Only when changed
    0, 0, 0,
)
```

### RequestDataOnSimObjectType

Requests data for all objects of a specific type within a radius.

```go
// Get all aircraft within 50 nautical miles
client.RequestDataOnSimObjectType(
    TrafficReqID,
    TrafficDefID,
    50 * 1852,  // Radius in meters (50nm)
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT,
)
```

### SetDataOnSimObject

Sends data to the simulator to modify object state.

```go
type AircraftPosition struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}

pos := AircraftPosition{
    Latitude:  47.4647,
    Longitude: -122.3078,
    Altitude:  10000,
}

client.SetDataOnSimObject(
    PositionDefID,
    types.SIMCONNECT_OBJECT_ID_USER,
    types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
    0,
    unsafe.Sizeof(pos),
    &pos,
)
```

## Processing Messages

### Message Structure

The `Stream()` channel returns `Message` structs containing the raw SimConnect data.

```go
type Message struct {
    DwSize    uint32  // Size of the message
    DwVersion uint32  // Protocol version
    DwID      uint32  // Message type (SIMCONNECT_RECV_ID)
    Raw       []byte  // Raw message data
}
```

### Casting Message Data

Use `CastDataAs` to convert raw data to typed structs:

```go
type AircraftPosition struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}

for msg := range client.Stream() {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
        data := msg.AsSimObjectData()
        if data.DwRequestID == PositionReqID {
            pos := engine.CastDataAs[AircraftPosition](&data.DwData)
            fmt.Printf("Position: %.4f, %.4f @ %.0fft\n",
                pos.Latitude, pos.Longitude, pos.Altitude)
        }
    }
}
```

**Signature**: `func CastDataAs[T any](dwData *types.DWORD) *T`

The struct layout must exactly match the SimConnect data definition order and types. Use `float64` for `SIMCONNECT_DATATYPE_FLOAT64`, `int32`/`uint32` for `INT32`, and fixed-size byte arrays for strings (`[256]byte` for `STRING256`).

### Message Type Methods

The `Message` struct provides helper methods to cast to specific types:

| Method | Returns | Use Case |
|--------|---------|----------|
| `AsOpen()` | Connection open data | Initial connection info |
| `AsQuit()` | Quit notification | Simulator shutdown |
| `AsException()` | Exception details | Error handling |
| `AsEvent()` | Event data | System events |
| `AsEventEx1()` | Extended event data | Extended event info |
| `AsSimObjectData()` | Object data | Data definition responses |
| `AsSimObjectDataByType()` | Object type data | Type-based queries |
| `AsFacilityData()` | Facility data | Airport/waypoint info |
| `AsFacilitiesList()` | Facilities list | Facility enumerations |
| `AsAssignedObjectId()` | Assigned ID | AI object creation |

### Parsing Strings

SimConnect returns null-terminated strings. Use `ParseNullTerminatedString` to convert:

```go
title := engine.ParseNullTerminatedString(data.SzTitle[:])
```

## System State

### RequestSystemState

Requests simulator system state information.

```go
client.RequestSystemState(
    StateReqID,
    types.SIMCONNECT_SYSTEM_STATE_SIM,  // Simulation running state
)
```

Response arrives as `SIMCONNECT_RECV_ID_SYSTEM_STATE` message.

## Events

### SubscribeToSystemEvent

Subscribes to simulator system events.

```go
const PauseEventID = 2000

client.SubscribeToSystemEvent(PauseEventID, "Pause")
client.SubscribeToSystemEvent(2001, "Sim")
client.SubscribeToSystemEvent(2002, "SimStart")
client.SubscribeToSystemEvent(2003, "SimStop")
```

### UnsubscribeFromSystemEvent

Removes a system event subscription.

```go
client.UnsubscribeFromSystemEvent(PauseEventID)
```

### MapClientEventToSimEvent

Maps a client event ID to a simulator event name.

```go
const GearToggleEventID = 3000

client.MapClientEventToSimEvent(GearToggleEventID, "GEAR_TOGGLE")
```

### TransmitClientEvent

Sends an event to the simulator.

```go
// Toggle landing gear
client.TransmitClientEvent(
    types.SIMCONNECT_OBJECT_ID_USER,
    GearToggleEventID,
    0,  // Event data
    types.SIMCONNECT_GROUP_PRIORITY_HIGHEST,
    types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY,
)
```

### TransmitClientEventEx1

Extended version with additional data parameters.

```go
client.TransmitClientEventEx1(
    types.SIMCONNECT_OBJECT_ID_USER,
    eventID,
    types.SIMCONNECT_GROUP_PRIORITY_HIGHEST,
    types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY,
    data0, data1, data2, data3, data4,
)
```

## Notification Groups

### AddClientEventToNotificationGroup

Groups events for priority-based handling.

```go
const FlightControlsGroup = 4000

client.AddClientEventToNotificationGroup(
    FlightControlsGroup,
    GearToggleEventID,
    false,  // maskable
)
```

### SetNotificationGroupPriority

Sets the priority for a notification group.

```go
client.SetNotificationGroupPriority(
    FlightControlsGroup,
    types.SIMCONNECT_GROUP_PRIORITY_HIGHEST,
)
```

### ClearNotificationGroup

Removes all events from a notification group.

```go
client.ClearNotificationGroup(FlightControlsGroup)
```

## AI Traffic

### AICreateParkedATCAircraft

Creates an AI aircraft parked at an airport.

```go
client.AICreateParkedATCAircraft(
    "Airbus A320 Neo Asobo",
    "N12345",
    "KSEA",      // ICAO code
    CreateReqID,
)
```

### AICreateEnrouteATCAircraft

Creates an AI aircraft following a flight plan.

```go
client.AICreateEnrouteATCAircraft(
    "Boeing 747-8 Asobo",
    "UAL123",
    0,                    // Flight number
    "KSEA_KLAX.pln",      // Flight plan file
    0.5,                  // Progress (0.0-1.0)
    false,                // Ground clamped
    CreateReqID,
)
```

### AISetAircraftFlightPlan

Assigns a flight plan to an existing AI aircraft.

```go
client.AISetAircraftFlightPlan(
    objectID,
    "KSEA_KLAX.pln",
    FlightPlanReqID,
)
```

### AIRemoveObject

Removes an AI object from the simulation.

```go
client.AIRemoveObject(objectID, RemoveReqID)
```

### AIReleaseControl

Releases control of an AI object back to the simulator.

```go
client.AIReleaseControl(objectID, ReleaseReqID)
```

### AICreateNonATCAircraft

Creates a non-ATC-controlled aircraft at a specific position.

```go
initPos := types.SIMCONNECT_DATA_INITPOSITION{
    Latitude:  47.4502,
    Longitude: -122.3088,
    Altitude:  5000,
    Pitch:     0,
    Bank:      0,
    Heading:   270,
    OnGround:  0,
    Airspeed:  120,
}
client.AICreateNonATCAircraft("Cessna Skyhawk Asobo", "N12345", initPos, CreateReqID)
```

### AICreateSimulatedObject

Creates a non-aircraft simulated object (ground vehicle, boat, etc.).

```go
client.AICreateSimulatedObject("Generic Ground Vehicle", initPos, CreateReqID)
```

### EX1 Variants (Livery Selection)

The `EX1` variants of AI creation methods add a `livery` parameter:

```go
client.AICreateParkedATCAircraftEX1("Boeing 787-10 Asobo", "House", "N12345", "KSEA", ReqID)
client.AICreateEnrouteATCAircraftEX1("Airbus A320 Neo Asobo", "Asobo", "UAL123", 0, "plan.pln", 0.5, false, ReqID)
client.AICreateNonATCAircraftEX1("Cessna Skyhawk Asobo", "Blue", "N12345", initPos, ReqID)
```

### EnumerateSimObjectsAndLiveries

Lists available aircraft/objects and their liveries.

```go
client.EnumerateSimObjectsAndLiveries(EnumReqID, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

## Facilities

### RequestFacilitiesList

Requests a list of facilities (airports, waypoints, etc.).

```go
client.RequestFacilitiesList(
    types.SIMCONNECT_FACILITY_LIST_TYPE_AIRPORT,
    FacilitiesReqID,
)
```

### RequestFacilityData

Requests detailed data for a specific facility.

```go
client.RequestFacilityData(
    FacilityDefID,
    FacilityDataReqID,
    "KSEA",
    "",  // Region (optional)
    types.SIMCONNECT_FACILITY_DATA_AIRPORT,
)
```

### RequestFacilityDataEX1

Extended version that explicitly specifies the facility type.

```go
client.RequestFacilityDataEX1(
    FacilityDefID,
    FacilityDataReqID,
    "KSEA",
    "",                                        // Region (optional)
    types.SIMCONNECT_FACILITY_DATA_AIRPORT,    // Explicit facility type
)
```

### RequestJetwayData

Requests jetway data for a specific airport.

```go
indexes := []int32{0, 1, 2}
client.RequestJetwayData("KSEA", uint32(len(indexes)), &indexes[0])
```

### RequestAllFacilities

Requests all facilities of a specific type.

```go
client.RequestAllFacilities(
    types.SIMCONNECT_FACILITY_LIST_TYPE_AIRPORT,
    AllFacilitiesReqID,
)
```

### SubscribeToFacilitiesEX1

Extended subscription with separate request IDs for facilities entering and leaving range.

```go
client.SubscribeToFacilitiesEX1(
    types.SIMCONNECT_FACILITY_LIST_TYPE_AIRPORT,
    NewInRangeReqID,
    OldOutRangeReqID,
)
```

### AddFacilityDataDefinitionFilter

Filters facility data requests.

```go
client.AddFacilityDataDefinitionFilter(
    FacilityDefID,
    "filterPath",
    filterDataPtr,
    filterDataSize,
)
```

### ClearAllFacilityDataDefinitionFilters

Removes all filters from a facility definition.

```go
client.ClearAllFacilityDataDefinitionFilters(FacilityDefID)
```

### SubscribeToFacilities

Subscribes to facility updates within a radius.

```go
client.SubscribeToFacilities(
    types.SIMCONNECT_FACILITY_LIST_TYPE_AIRPORT,
    FacilitySubReqID,
)
```

### AddToFacilityDefinition

Defines the data fields to receive for facilities.

```go
client.AddToFacilityDefinition(FacilityDefID, "OPEN AIRPORT")
client.AddToFacilityDefinition(FacilityDefID, "NAME")
client.AddToFacilityDefinition(FacilityDefID, "LATITUDE")
client.AddToFacilityDefinition(FacilityDefID, "LONGITUDE")
```

## Flight Operations

### FlightLoad

Loads a saved flight file.

```go
client.FlightLoad("C:/Users/Pilot/Flights/MyFlight.flt")
```

### FlightPlanLoad

Loads a flight plan file.

```go
client.FlightPlanLoad("C:/Users/Pilot/FlightPlans/KSEA_KLAX.pln")
```

### FlightSave

Saves the current flight state.

```go
client.FlightSave(
    "C:/Users/Pilot/Flights/SavedFlight.flt",
    "My Saved Flight",
    "Saved at KSEA runway 16L",
)
```

## Example: Complete Data Loop

```go
package main

import (
    "fmt"
    "os"
    "os/signal"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

type AircraftData struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
    Heading   float64
}

const (
    DataDefID = 1000
    DataReqID = 1001
)

func main() {
    client := engine.New("DataLoop")

    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    // Define data structure
    client.AddToDataDefinition(DataDefID, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    client.AddToDataDefinition(DataDefID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    client.AddToDataDefinition(DataDefID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    client.AddToDataDefinition(DataDefID, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)

    // Request periodic updates
    client.RequestDataOnSimObject(
        DataReqID, DataDefID,
        types.SIMCONNECT_OBJECT_ID_USER,
        types.SIMCONNECT_PERIOD_SECOND,
        types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
        0, 0, 0,
    )

    // Handle shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)

    stream := client.Stream()
    for {
        select {
        case <-sigChan:
            fmt.Println("Shutting down...")
            return
        case msg, ok := <-stream:
            if !ok {
                return
            }
            if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
                data := msg.AsSimObjectData()
                if data.DwRequestID == DataReqID {
                    aircraft := engine.CastDataAs[AircraftData](&data.DwData)
                    fmt.Printf("Pos: %.4f, %.4f | Alt: %.0fft | Hdg: %.1f°\n",
                        aircraft.Latitude, aircraft.Longitude,
                        aircraft.Altitude, aircraft.Heading)
                }
            }
        }
    }
}
```

## See Also

- [Client Configuration](config-client.md) — All configuration options
- [Manager Usage](usage-manager.md) — Automatic connection lifecycle management
- [Examples](../examples) — Working code samples
