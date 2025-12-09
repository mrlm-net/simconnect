# Read Facility Example

## Overview

This example demonstrates how to read facility data from Microsoft Flight Simulator using SimConnect's facility data API. It showcases how to define facility data structures, request specific airport information, and process facility data responses.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Defines facility data structure** - Sets up airport data fields to retrieve
3. **Requests specific facility** - Retrieves detailed data for LKPR (Prague Václav Havel Airport)
4. **Processes facility data messages** - Handles facility data responses with structured parsing
5. **Handles reconnection** - Automatically reconnects if the simulator disconnects
6. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- The simulator can be at any state (airport data is available regardless of loaded scenery)

## Running the Example

```bash
cd examples/read-facility
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
📨 Message received - SIMCONNECT_RECV_ID_OPEN
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator'
  Application Version: 1.0
  Application Build: 1.0
  SimConnect Version: 12.0
  SimConnect Build: 62651.0
📨 Message received - SIMCONNECT_RECV_ID_FACILITY_DATA
🏗️  Received SIMCONNECT_RECV_ID_FACILITY_DATA message!
  UserRequestId: 123
  UniqueRequestId: ...
  ParentUniqueRequestId: ...
  Type: 0
  IsListItem: false
  ItemIndex: 0
  ListSize: 1
  Data:
    Latitude: 50.100833
    Longitude: 14.260000
    Altitude: 1247.375000
    ICAO: 'LKPR'
    Name: 'Vaclav Havel Airport Prague'
    Name64: 'Vaclav Havel Airport Prague'
📨 Message received - SIMCONNECT_RECV_ID_FACILITY_DATA_END
🏁 Received SIMCONNECT_RECV_ID_FACILITY_DATA_END message!
📴 Stream closed (simulator disconnected)
⏳ Waiting 5 seconds before reconnecting...
```

## Code Explanation

### Facility Data Definition

The example defines an airport data structure to receive facility information:

```go
type AirportData struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
    ICAO      [8]byte
    Name      [32]byte
    Name64    [64]byte
}
```

### Adding to Facility Definition

The facility definition uses special OPEN/CLOSE markers and field names:

```go
client.AddToFacilityDefinition(3000, "OPEN AIRPORT")
client.AddToFacilityDefinition(3000, "LATITUDE")
client.AddToFacilityDefinition(3000, "LONGITUDE")
client.AddToFacilityDefinition(3000, "ALTITUDE")
client.AddToFacilityDefinition(3000, "ICAO")
client.AddToFacilityDefinition(3000, "NAME")
client.AddToFacilityDefinition(3000, "NAME64")
client.AddToFacilityDefinition(3000, "CLOSE AIRPORT")
```

**Important Notes**:
- Must start with `OPEN AIRPORT` and end with `CLOSE AIRPORT`
- Field names must match SimConnect's facility data field names
- The order of fields must match the struct definition
- See [SimConnect_AddToFacilityDefinition docs](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm#remarks) for usage details

### Requesting Facility Data

```go
client.RequestFacilityData(3000, 123, "LKPR", "")
```

Parameters:
- **3000** - Definition ID (matches the facility definition)
- **123** - User Request ID (used to identify the response)
- **"LKPR"** - ICAO code of the airport to retrieve
- **""** - Region (empty for default)

### Message Processing

The example processes two facility-related messages:

**SIMCONNECT_RECV_ID_FACILITY_DATA**:
```go
msg := msg.AsFacilityData()
// Contains:
// - UserRequestId: The request ID you specified (123)
// - UniqueRequestId: System-generated unique identifier
// - Type: Facility type (0 = Airport)
// - IsListItem: Whether this is part of a list
// - Data: Buffer containing the facility data
```

Use `engine.CastDataAs[T]()` to safely cast the data buffer to your struct:
```go
data := engine.CastDataAs[AirportData](&msg.Data)
```

**SIMCONNECT_RECV_ID_FACILITY_DATA_END**:
- Signals that all facility data for the request has been sent
- Important for list-based requests where multiple items may be returned

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic until simulator is available
2. Sets up facility data definition
3. Requests specific facility data
4. Processes messages in a loop
5. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

## Use Cases

This pattern demonstrates:
- Retrieving airport information programmatically
- Building airport databases from simulator data
- Creating navigation tools with airport details
- Validating flight plans against airport locations
- Building scenery analysis tools
- Creating airport information displays

## Facility Types

SimConnect supports various facility types beyond airports:
- Airports (OPEN AIRPORT / CLOSE AIRPORT)
- Waypoints (OPEN WAYPOINT / CLOSE WAYPOINT)
- NDBs (OPEN NDB / CLOSE NDB)
- VORs (OPEN VOR / CLOSE VOR)

Each facility type has its own set of available fields.

## Notes

- Facility data is available even when not near the requested location
- The ICAO code must be valid for the simulator's database
- String fields are fixed-size byte arrays (need conversion with `engine.BytesToString()`)
- Multiple facilities can be requested with different request IDs
- Facility definitions can be reused for multiple requests
- The simulator must be running but doesn't need a loaded flight
