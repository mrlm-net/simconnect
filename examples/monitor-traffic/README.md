# Monitor Traffic Example

## Overview

This example demonstrates how to monitor AI traffic (both parked and in-flight aircraft) with configuration loaded from JSON. It shows how to subscribe to aircraft data, handle periodic updates, and track multiple AI aircraft in real-time.

## What It Does

1. **Loads configuration** — Reads `dataset.json` file containing parked and IFR aircraft definitions
2. **Connects to simulator** — Establishes connection and waits for simulator readiness
3. **Spawns AI aircraft** — Creates parked aircraft at gates and IFR aircraft in flight
4. **Subscribes to data** — Requests periodic position and state updates for all aircraft
5. **Monitors traffic** — Receives and processes aircraft data every second
6. **Tracks positions** — Displays current position, heading, altitude, and speed for each aircraft
7. **Handles updates** — Updates aircraft state continuously as they move

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- `dataset.json` file with aircraft definitions (provided with example)

## Running the Example

```bash
cd examples/monitor-traffic
go run main.go
```

Or specify a custom dataset file:

```bash
go run main.go -dataset "/path/to/custom-dataset.json"
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
✅ Connected to SimConnect...
📥 Loading aircraft dataset...
✅ Loaded 3 parked aircraft and 2 IFR aircraft

🎯 Spawning AI traffic...
✅ Parked Aircraft:
   - EDDM Gate A01: Boeing 747-8i (D-ABYT)
   - EDDM Gate A02: Airbus A380 (D-AIMA)
   - EDDF Gate 1: Airbus A350 (D-AIXA)

🛫 IFR Aircraft:
   - Cessna 172 (N12345) - Flight Plan: KJFK→EGLL
   - Beechcraft Bonanza (N67890) - Flight Plan: KLAX→KSFO

📡 Monitoring Traffic (updates every second)...
   Boeing 747-8i (EDDM):
      Position: 48.3521°N, 11.7861°E
      Altitude: 1,500 ft | Heading: 270°
      Speed: 0 kts (parked)

   Cessna 172 (Airborne):
      Position: 40.6413°N, -73.7781°W
      Altitude: 5,500 ft | Heading: 087°
      Speed: 95 kts
```

## Code Explanation

### Configuration File Format

The `dataset.json` file defines AI traffic:

```json
{
  "parked": [
    {
      "airport": "EDDM",
      "plane": "AIRCRAFT_TYPE_STRING",
      "number": "N12345",
      "plan": "optional_flight_plan.pln"
    }
  ],
  "ifr": [
    {
      "plane": "AIRCRAFT_TYPE_STRING",
      "number": "D-ABCD",
      "plan": "flight_plan.pln",
      "phase": 0.5
    }
  ]
}
```

### Data Structures

```go
type AircraftData struct {
    Title             [128]byte  // Aircraft name
    Category          [128]byte  // Category (e.g., "Airplane")
    LiveryName        [128]byte  // Livery designation
    LiveryFolder      [128]byte  // Livery folder path
    Lat               float64    // Latitude
    Lon               float64    // Longitude
    Alt               float64    // Altitude (feet)
    Head              float64    // True heading
    HeadMag           float64    // Magnetic heading
    Vs                float64    // Vertical speed
    Pitch             float64    // Pitch angle
    Bank              float64    // Bank angle
    GroundSpeed       float64    // Ground speed (knots)
    AirspeedIndicated float64    // Indicated airspeed
    AirspeedTrue      float64    // True airspeed
    OnAnyRunway       int32      // On runway flag
    SurfaceType       int32      // Surface type
    SimOnGround       int32      // On ground flag
    AtcID             [32]byte   // ATC identifier
    AtcAirline        [32]byte   // Airline code
}
```

### Key APIs Used

- **AICreateParkedATCAircraft()** — Spawn parked aircraft at gates
- **AISetAircraftFlightPlan()** — Assign flight plan to aircraft
- **RequestDataOnSimObjectType()** — Request data for all aircraft
- **JSON unmarshaling** — Load aircraft definitions from file

### Data Subscription

Aircraft data is requested periodically (typically every second):

```go
client.RequestDataOnSimObjectType(
    requestID,           // Request ID
    defID,               // Data definition ID
    searchRadius,        // Search radius in meters (e.g., 500km)
    SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT,  // Object type
)
```

### Handling Multiple Aircraft

The example manages data from multiple aircraft:

```go
// Parse response containing all aircraft within radius
for aircraftIndex := 0; aircraftIndex < count; aircraftIndex++ {
    // Extract data for each aircraft
    // Update display or state tracking
}
```

## Use Cases

- **Traffic monitoring** — Watch AI aircraft movement in real-time
- **Flight schools** — Simulate multi-aircraft training scenarios
- **Air traffic control simulation** — Manage traffic flow from control tower
- **Photogrammetry** — Capture screenshots of traffic scenarios
- **Testing** — Validate flight plans and procedures with AI aircraft
- **Entertainment** — Add realistic airport activity to flights

## Configuration Tips

1. **Use realistic aircraft types** — Specify actual MSFS aircraft names
2. **Valid airports** — Use ICAO codes of airports in your scenery
3. **Flight plans** — Provide .pln files from examples or create custom ones
4. **Balancing** — Don't spawn too many aircraft (impacts performance)
5. **Liveries** — Use default liveries unless custom ones are installed

## Performance Considerations

- Each AI aircraft adds overhead to simulation
- Data requests should use appropriate radius filtering
- Update frequency affects frame rate (1 second = reasonable balance)
- Monitor system resources with large traffic volumes

## Related Examples

- [`manage-traffic`](../manage-traffic) — Create and control AI aircraft
- [`ai-traffic`](../ai-traffic) — Execute flight plans using dataset files
- [`subscribe-events`](../subscribe-events) — Monitor system events
- [`simconnect-state`](../simconnect-state) — Track simulator state

## See Also

- [AI Traffic API](../../docs/config-client.md) — SimConnect AI traffic methods
- [Datasets Package](../../pkg/datasets) — Pre-built data definitions
