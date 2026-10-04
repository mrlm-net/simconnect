# Airport Details Example

## Overview

This example demonstrates how to query detailed information about specific airports, including parking places, taxi paths, taxi points and taxiway names. It shows how to retrieve comprehensive airport facility data with multi-packet response handling.

## What It Does

1. **Connects to the simulator** — Establishes connection and waits for simulator to be ready
2. **Queries airport data** — Requests facility information for a specific ICAO code (default: EDDM Munich)
3. **Retrieves parking places** — Gets available parking positions with heading and dimensions
4. **Reads the taxiways** — Taxi paths, taxi points and taxiway names
5. **Parses facility details** — Handles multi-packet responses for large datasets
6. **Displays results** — Shows airport name, coordinates, and available parking

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- SimConnect SDK libraries for facility queries

## Running the Example

```bash
cd examples/airport-details
go run main.go
```

To query a different airport, modify the `airport` variable at the top of `main.go`:

```go
var airport = "KJFK"  // JFK Airport
var airport = "EGLL"  // London Heathrow
var airport = "LFPG"  // Paris Charles de Gaulle
```

## Expected Output

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect...
📋 Requesting airport data for EDDM...

🏢 Airport: Munich (EDDM)
   Latitude: 48.3521°N
   Longitude: 11.7861°E
   Altitude: 1,487 ft
   
🅿️  Parking Places:
   1. Gate A01 - Heading: 270° - Type: 0
   2. Gate A02 - Heading: 270° - Type: 0
   3. Gate B01 - Heading: 90° - Type: 1
   
🛣️  Taxi Paths: 47 paths configured
```

## Code Explanation

### Key APIs Used

- **RequestFacilitiesList()** — Query airport facilities
- **RequestFacilityData()** — Get detailed airport information
- **SIMCONNECT_RECV_ID_FACILITY_DATA** — Parse multi-packet facility responses

### Airport Data Structure

The example defines custom structures to parse facility data:

```go
type AirportData struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
    ICAO      [8]byte
    Name      [32]byte
    Name64    [64]byte
}

type ParkingPlace struct {
    Name             uint32
    Number           uint32
    Heading          float32
    Type             uint32
    BiasX            float32
    BiasZ            float32
    NumberOfAirlines uint32
}
```

### Parsing Multi-Packet Responses

Large facility datasets are split across multiple SimConnect messages. The example handles packet sequencing:

```go
case SIMCONNECT_RECV_ID_FACILITY_DATA:
    // Handle first packet
    // Parse subsequent packets
    // Combine data when complete
```

### Airport Information Available

| Data Point | Type | Description |
|-----------|------|-------------|
| ICAO Code | String | 4-character airport identifier |
| Name | String | Full airport name |
| Latitude | Float64 | Position in degrees |
| Longitude | Float64 | Position in degrees |
| Altitude | Float64 | Elevation in feet |
| Parking Places | Array | Gate positions with headings and types |
| Runways | Array | Runway configurations and surface types |
| Taxi Paths | Array | Ground movement routes |

## Use Cases

- **Airport selection** — Browse available parking and gate information
- **Flight planning** — Check runway configurations before flight
- **AI traffic management** — Get parking positions for AI aircraft
- **Real-world accuracy** — Compare MSFS airport data with real airports
- **Route planning** — Understand airport layout before arrival

## Related Examples

- [`locate-airport`](../locate-airport) — Find nearby airports using geolocation
- [`read-facility`](../read-facility) — Retrieve single facility by ICAO
- [`read-facilities`](../read-facilities) — Query multiple facilities with filtering
- [`all-facilities`](../all-facilities) — Enumerate complete facility database
- [`manage-traffic`](../manage-traffic) — Use airport data for AI traffic positioning

## See Also

- [Facilities Data](../../pkg/datasets/facilities) — Pre-built facility data definitions
- [SimConnect Facilities API](../../docs/config-client.md) — Request facilities documentation
