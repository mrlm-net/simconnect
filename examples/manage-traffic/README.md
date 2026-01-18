# Manage Traffic Example

## Overview

This comprehensive example demonstrates advanced AI traffic management, including spawning parked and in-flight aircraft, assigning flight plans, updating aircraft positions, and controlling AI behavior. It shows how to create realistic traffic scenarios and manipulate aircraft state in real-time.

## What It Does

1. **Loads configuration** — Reads `dataset.json` file containing aircraft definitions
2. **Connects to simulator** — Establishes connection with automatic reconnection
3. **Spawns parked aircraft** — Creates aircraft at airport gates with parking clearances
4. **Spawns IFR aircraft** — Launches aircraft in flight with assigned flight plans
5. **Assigns flight plans** — Associates procedural flight plans to aircraft
6. **Tracks aircraft** — Monitors position, speed, heading, and other telemetry
7. **Updates positions** — Moves and manipulates aircraft state dynamically
8. **Handles lifecycle** — Manages creation, movement, and removal of AI traffic

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- `dataset.json` file with aircraft definitions (provided with example)
- Flight plan `.pln` files for IFR aircraft (located in `plans/` folder)

## Running the Example

```bash
cd examples/manage-traffic
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
✅ Loaded 5 parked aircraft and 3 IFR aircraft

🎯 Spawning Parked Aircraft...
   ✅ Created: Boeing 747-8i (D-ABYT) at EDDM Gate A01
   ✅ Created: Airbus A380 (D-AIMA) at EDDM Gate A02
   ✅ Created: Airbus A350 (D-AIXA) at EDDF Gate 1

🛫 Spawning IFR Aircraft...
   ✅ Created: Cessna 172 (N12345) at flight phase 0.3
   ✅ Assigned flight plan: KJFK_EGLL.pln
   ✅ Created: Beechcraft Bonanza (N67890) at flight phase 0.5
   ✅ Assigned flight plan: KLAX_KSFO.pln

📡 Managing Aircraft...
   Boeing 747-8i:
      Position: 48.3521°N, 11.7861°E
      Altitude: 1,500 ft | Heading: 270°
      Speed: 0 kts | On Ground: Yes
      Status: Parked at Gate A01

   Cessna 172:
      Position: 40.6413°N, -73.7781°W
      Altitude: 5,500 ft | Heading: 087°
      Speed: 125 kts | On Ground: No
      Status: Following flight plan (0.35 progress)

📊 Performance Metrics:
   Total Aircraft: 8
   Parked: 5 | In Flight: 3
   CPU Usage: 12.5% | Memory: 256 MB
```

## Code Explanation

### Configuration File Format

The `dataset.json` defines AI traffic with full control:

```json
{
  "parked": [
    {
      "airport": "EDDM",
      "plane": "Boeing 747-8 Asobo",
      "number": "D-ABYT",
      "plan": "optional_pushback.pln",
      "clearance": 1
    }
  ],
  "ifr": [
    {
      "plane": "Cessna 172 G1000",
      "number": "N12345",
      "plan": "KJFK_EGLL.pln",
      "phase": 0.3
    }
  ]
}
```

### Key APIs Used

- **AICreateParkedATCAircraft()** — Spawn parked aircraft at gates
- **AICreateEnrouteATCAircraft()** — Spawn aircraft in flight
- **AISetAircraftFlightPlan()** — Assign flight plan to aircraft
- **RequestDataOnSimObjectType()** — Get periodic aircraft data
- **SetDataOnSimObject()** — Update aircraft state/position
- **TransmitClientEvent()** — Trigger aircraft actions

### Aircraft Data Structure

```go
type AircraftData struct {
    Title             [128]byte  // Aircraft type
    Category          [128]byte  // Category
    LiveryName        [128]byte  // Livery specification
    LiveryFolder      [128]byte  // Livery path
    Lat               float64    // Latitude
    Lon               float64    // Longitude
    Alt               float64    // Altitude (feet)
    Head              float64    // True heading
    HeadMag           float64    // Magnetic heading
    Vs                float64    // Vertical speed (ft/min)
    Pitch             float64    // Pitch angle (degrees)
    Bank              float64    // Bank angle (degrees)
    GroundSpeed       float64    // Ground speed (knots)
    AirspeedIndicated float64    // IAS
    AirspeedTrue      float64    // TAS
    OnAnyRunway       int32      // On runway flag
    SurfaceType       int32      // Surface type
    SimOnGround       int32      // Gear down flag
    AtcID             [32]byte   // ATC identifier
    AtcAirline        [32]byte   // Airline code
}
```

### Creating Parked Aircraft

```go
// Spawn at gate with optional flight plan
client.AICreateParkedATCAircraft(
    aircraftType,        // MSFS aircraft name
    tailNumber,          // Registration
    airport,             // ICAO code
    parkingID,           // Parking position
    client,              // Parking category
)

// Assign optional flight plan (pushback, boarding, etc.)
client.AISetAircraftFlightPlan(objectID, flightPlanPath)
```

### Creating IFR Aircraft

```go
// Spawn in flight at cruise phase
client.AICreateEnrouteATCAircraft(
    aircraftType,        // Aircraft name
    tailNumber,          // Registration
    initPhase,           // Flight phase (0.0-1.0)
    flightPlanPath,      // Flight plan file
    heading,             // Initial heading
    altitude,            // Initial altitude
    speedKnots,          // Initial speed
)
```

### Flight Plan Files

Flight plans (.pln) define aircraft routes:

```
- LKPRLFPG.pln - Prague to Prague with procedures
- LKPRZSPD.pln - Prague to Žamberk
- Custom plans: Create with MSFS flight planner
```

### Advanced Features

**Dynamic Position Updates:**
```go
// Update aircraft position in real-time
client.SetDataOnSimObject(defID, objectID, flags, &newPosition)
```

**Flight Phase Control:**
```go
// Progress through flight plan phases (0.0 = gate, 1.0 = destination)
phase := 0.3  // 30% through flight plan
client.AISetAircraftFlightPlan(objectID, flightPlan, phase)
```

**Aircraft Interactions:**
```go
// Transmit events to aircraft (doors, lights, engines, etc.)
client.TransmitClientEvent(objectID, eventID, data, eventFlag)
```

## Use Cases

- **Air traffic control training** — Manage realistic traffic flow
- **Flight schools** — Create multi-aircraft training scenarios
- **Photography/video** — Generate traffic for cinematic content
- **Route testing** — Validate procedures with AI traffic
- **Multiplayer simulation** — Create shared traffic scenarios
- **Performance testing** — Load test with many aircraft
- **Procedural validation** — Test SID/STAR procedures

## Configuration Tips

1. **Aircraft selection** — Use exact MSFS aircraft names
2. **Parking positions** — Use valid gate IDs from airport data
3. **Flight plans** — Place .pln files in `plans/` folder
4. **Spacing** — Space aircraft appropriately to avoid collisions
5. **Performance** — Limit aircraft count based on system specs

## Advanced Techniques

### Custom Aircraft Types

```json
{
  "plane": "Asobo Airbus A320 Neo",
  "number": "N1234A",
  "plan": "custom_route.pln"
}
```

### Animated Flight Phases

```go
// Progress aircraft through phases over time
for phase := 0.0; phase <= 1.0; phase += 0.05 {
    client.AISetAircraftFlightPlan(objectID, plan, phase)
    time.Sleep(100 * time.Millisecond)
}
```

### Event Sequencing

```go
// Trigger events in sequence (doors, gear, etc.)
client.TransmitClientEvent(objectID, doorOpenEvent, 0, eventGroupPriority)
time.Sleep(2 * time.Second)
client.TransmitClientEvent(objectID, doorCloseEvent, 0, eventGroupPriority)
```

## Performance Considerations

- **Aircraft limit** — 50-100 aircraft typical before performance impact
- **Update frequency** — Less frequent updates = better performance
- **Positioning** — Distant aircraft use fewer resources
- **Systems** — Full failure simulation adds overhead
- **Rendering** — Visible aircraft render slower than distant ones

## Related Examples

- [`monitor-traffic`](../monitor-traffic) — Monitor AI traffic without control
- [`ai-traffic`](../ai-traffic) — Execute simple flight plan scenarios
- [`airport-details`](../airport-details) — Query parking positions
- [`simconnect-state`](../simconnect-state) — Track simulator state

## See Also

- [AI Traffic API](../../docs/config-client.md) — Complete AI traffic methods
- [Datasets Package](../../pkg/datasets/traffic) — Pre-built traffic data definitions
- [MSFS Flight Plans](https://www.microsoft.com/en-us/p/microsoft-flight-simulator/9nxbk56z6h0t) — Flight plan format documentation
