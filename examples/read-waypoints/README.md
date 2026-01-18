# Read Waypoints Example

## Overview

This example demonstrates how to query waypoint data from the simulator's facilities database. It shows how to request and parse waypoint information including coordinates, magnetic variation, and regional classification.

## What It Does

1. **Connects to the simulator** — Establishes connection and waits for simulator readiness
2. **Requests waypoint list** — Queries all waypoints from the facilities database
3. **Parses waypoint data** — Handles facility responses containing waypoint details
4. **Extracts coordinates** — Retrieves latitude, longitude, and altitude for each waypoint
5. **Displays results** — Shows waypoint identifiers, names, and positions

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- SimConnect SDK libraries for facility queries

## Running the Example

```bash
cd examples/read-waypoints
go run main.go
```

## Expected Output

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect...
📍 Requesting waypoint data...
✅ Waypoints loaded. Press Ctrl+C to exit...

📍 Waypoints Found:
   1. ABD - Latitude: 45.2341°N, Longitude: 10.1234°E
   2. ABE - Latitude: 46.5234°N, Longitude: 9.8765°E
   3. ABR - Latitude: 47.1234°N, Longitude: 11.2345°E
   ...
   
Processing 2,847 waypoints...
✅ Done
```

## Code Explanation

### Key APIs Used

- **RequestFacilitiesList()** — Query facilities of type WAYPOINT
- **RequestFacilityData()** — Get detailed waypoint information
- **Facilities event parsing** — Handle multi-packet facility responses

### Waypoint Data Structure

Waypoints contain navigation information used in flight planning:

```go
type AirportData struct {
    Latitude  float64   // Position in degrees
    Longitude float64   // Position in degrees
    Altitude  float64   // Elevation in feet
    ICAO      [8]byte   // Waypoint identifier
    Name      [32]byte  // Full waypoint name
    Name64    [64]byte  // Extended name
}
```

### Facility Types

SimConnect distinguishes several facility types:

| Type | Description |
|------|-------------|
| `AIRPORT` | Landing facilities with runways and parking |
| `WAYPOINT` | Navigation points used in flight planning |
| `NDB` | Non-Directional Beacons (radio navigation) |
| `VOR` | VHF Omnidirectional Range (radio navigation) |

### Requesting Waypoint Data

```go
// Query all waypoints from facilities database
client.RequestFacilitiesList(requestID, types.SIMCONNECT_FACILITY_LIST_TYPE_WAYPOINT)

// Handle facility responses
for msg := range messageChan {
    if msg.Type == SIMCONNECT_RECV_ID_FACILITY_DATA {
        // Parse waypoint data
    }
}
```

### Parsing Facility Packets

Large datasets are split across multiple packets:

```
Packet 1 (Size: 65KB)
├── Waypoint 1
├── Waypoint 2
└── ...

Packet 2 (Size: 45KB)
├── Waypoint N
└── ...
```

Each packet must be parsed and combined to get the complete dataset.

## Use Cases

- **Navigation planning** — Access all available waypoints for flight planning
- **Route building** — Create waypoint sequences for autopilot
- **Map displays** — Show waypoint locations on moving map displays
- **Flight log analysis** — Track flights through waypoint network
- **SID/STAR procedures** — Access procedure waypoints

## Data Characteristics

- **Total waypoints** — Thousands of waypoints worldwide
- **Coverage** — Complete coverage of default and addon scenery
- **Coordinates** — High-precision lat/lon in decimal degrees
- **Altitude** — Recommended flight levels and minimum altitudes
- **Magnetic variation** — Regional magnetic declination values

## Related Examples

- [`read-facility`](../read-facility) — Retrieve single facility by ICAO
- [`read-facilities`](../read-facilities) — Query facilities with filtering
- [`locate-airport`](../locate-airport) — Find airports by geolocation
- [`all-facilities`](../all-facilities) — Enumerate complete facility database

## See Also

- [Facilities Data](../../pkg/datasets/facilities) — Pre-built facility data definitions
- [SimConnect Facilities API](../../docs/config-client.md) — Request facilities documentation
