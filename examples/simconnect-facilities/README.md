# SimConnect Facilities Example

## Overview

This example demonstrates how to use pre-defined facility datasets from the `pkg/datasets/facilities` package to retrieve detailed airport information from Microsoft Flight Simulator. It queries airport, runway, parking, and frequency data using the dataset-based API instead of manual field definitions.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Dataset-based facility queries** - Uses pre-defined datasets instead of manual `AddToFacilityDefinition` calls
3. **Nested facility data** - Retrieves airport, runway, parking, and frequency data in separate requests
4. **Structured data parsing** - Handles complex facility data structures with type-safe Go structs
5. **Command-line ICAO override** - Allows querying any airport via command-line argument
6. **Handles reconnection** - Automatically reconnects if the simulator disconnects
7. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- The simulator can be at any state (facility data is available regardless of loaded scenery)

## Running the Example

```bash
cd examples/simconnect-facilities
go run main.go           # Queries LKPR (Prague) by default
go run main.go KJFK      # Query specific airport by ICAO code
```

## Expected Output

```
SimConnect Facilities Example - Querying LKPR
(Press Ctrl+C to exit)

Waiting for simulator to start...
Connected to SimConnect

SimConnect: Microsoft Flight Simulator v1.0

=== Airport: Vaclav Havel Airport Prague ===
  ICAO:     LKPR
  Region:   CZ
  Position: 50.100833, 14.260000 @ 1247.4 ft
  MagVar:   3.78

--- Runway 1 ---
  Position: 50.098056, 14.254722 @ 1247.4 ft
  Heading:  119.7
  Size:     12191 x 150 ft
  Surface:  Asphalt
  Pattern:  2247 ft
  Slope:    0.05 / True: 0.05

  Parking 1: Gate Medium #205  Heading=181.3  Radius=20.0
  Parking 2: Gate Medium #206  Heading=181.3  Radius=20.0
  ...

  Frequency 1: ATIS         125.905 MHz  ATIS
  Frequency 2: Clearance    121.255 MHz  Clearance
  ...

=== Facility data complete ===
  Runways:     3
  Parking:     156
  Frequencies: 7

Done. Press Ctrl+C to exit or wait for reconnection.
```

## Code Explanation

### Using Pre-Defined Datasets

Instead of manually calling `AddToFacilityDefinition` for each field, this example uses pre-defined datasets from `pkg/datasets/facilities`:

```go
// Register airport dataset (includes all standard airport fields)
client.RegisterFacilityDataset(defAirport, facilities.NewAirportFacilityDataset())

// Register parking dataset nested under airport
client.AddToFacilityDefinition(defParking, "OPEN AIRPORT")
client.RegisterFacilityDataset(defParking, facilities.NewParkingFacilityDataset())
client.AddToFacilityDefinition(defParking, "CLOSE AIRPORT")
```

### Nested Facility Queries

Runways, parking, and frequencies are nested within airports. Wrap nested datasets with OPEN/CLOSE brackets:

```go
client.AddToFacilityDefinition(defRunway, "OPEN AIRPORT")
client.RegisterFacilityDataset(defRunway, runwayDataset)
client.AddToFacilityDefinition(defRunway, "CLOSE AIRPORT")
```

### Requesting Facility Data

```go
client.RequestFacilityData(defAirport, reqAirport, "LKPR", "")
client.RequestFacilityData(defRunway, reqRunway, "LKPR", "")
client.RequestFacilityData(defParking, reqParking, "LKPR", "")
client.RequestFacilityData(defFrequency, reqFrequency, "LKPR", "")
```

Parameters:
- **defAirport/defRunway/etc.** - Definition ID (must match `RegisterFacilityDataset` call)
- **reqAirport/reqRunway/etc.** - Request ID (used to identify responses)
- **"LKPR"** - ICAO code of facility to query
- **""** - Region filter (empty = any region)

### Handling Responses

Responses arrive as `SIMCONNECT_RECV_ID_FACILITY_DATA` messages with type-specific data:

```go
case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
    fd := msg.AsFacilityData()
    switch uint32(fd.UserRequestId) {
    case reqAirport:
        data := engine.CastDataAs[AirportData](&fd.Data)
    case reqRunway:
        data := engine.CastDataAs[RunwayData](&fd.Data)
    }
```

### Completion Detection

SimConnect sends `SIMCONNECT_RECV_ID_FACILITY_DATA_END` when all data has been delivered:

```go
case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
    fmt.Println("=== Facility data complete ===")
```

## Use Cases

This pattern demonstrates:
- Building airport information displays for flight planning tools
- Creating navigation databases from simulator data
- Analyzing runway configurations and parking availability
- Building frequency reference guides for ATC communication
- Generating airport reports and documentation

## Comparison with Other Facility Examples

| Example | Purpose | API Used | Data Scope |
|---------|---------|----------|------------|
| **simconnect-facilities** | Dataset-based detailed queries | `RequestFacilityData` + datasets | Single facility, detailed |
| read-facilities | List all facilities | `RequestFacilitiesListEX1` | All facilities, basic info |
| read-facility | Manual detailed query | `RequestFacilityData` (manual defs) | Single facility, custom fields |
| airport-details | Airport-specific lookup | `RequestFacilityData` | Single airport |
| all-facilities | Enumerate all types | `RequestFacilitiesListEX1` (multiple) | All facilities, basic info |

## Important Notes

- **Facility definition required** - Unlike list requests, data requests need definitions
- **One facility per request** - Each `RequestFacilityData` returns one facility
- **Nested queries need OPEN/CLOSE** - Runways, parking, frequencies are nested in airports
- **ICAO must be exact** - Case-sensitive, must match simulator database
- **Region is optional** - Empty string matches any region
- **Struct alignment matters** - Go struct fields must match SimConnect binary layout

## Further Reading

- [SimConnect_RequestFacilityData docs](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilityData.htm)
- [SimConnect_AddToFacilityDefinition docs](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm)
- Read Facilities Example (for facility list enumeration)
- Airport Details Example (for airport-specific queries)
