# AI Traffic Example

## Overview

This example demonstrates how to create and manage AI traffic in Microsoft Flight Simulator using the SimConnect SDK. It shows how to spawn parked aircraft at airports and create enroute IFR traffic with flight plans, along with continuous monitoring of all aircraft in the simulation.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Loads aircraft configuration** - Reads aircraft definitions from `planes.json`
3. **Spawns parked aircraft** - Creates AI aircraft parked at specified airports
4. **Creates enroute traffic** - Spawns AI aircraft following IFR flight plans with configurable delays
5. **Monitors all aircraft** - Periodically requests and displays data for all aircraft within 25km radius
6. **Handles reconnection** - Automatically reconnects if the simulator disconnects
7. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- Flight plan files (`.pln` format) in the `plans/` directory

## Configuration

### planes.json Format

The example uses a JSON file to define aircraft to spawn:

```json
[
  {
    "airport": "LKPR",
    "plane": "FSLTL A320 VLG Vueling",
    "number": "N12345"
  },
  {
    "plane": "FSLTL A320 Air France SL",
    "number": "N12347",
    "plan": "LKPRLFPG.pln",
    "clearance": 10,
    "phase": 0.0
  }
]
```

**Fields:**
- `airport` - ICAO code for parked aircraft
- `plane` - Aircraft title (must match installed aircraft)
- `number` - Call sign/tail number
- `plan` - Flight plan filename (optional, for enroute aircraft)
- `clearance` - Delay in seconds before spawning enroute aircraft (optional)
- `phase` - Initial flight plan phase (optional, 0.0 = start of route)

## Running the Example

```bash
cd examples/ai-traffic
go run main.go
```

## Expected Output

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
ℹ️  (Press Ctrl+C to exit)
📝 Adding parked plane - plane=FSLTL A320 VLG Vueling number=N12345
📝 Assigning flight plan for FSLTL A320 Air France SL (N12347) after 10s
✈️  Ready for plane spotting???
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Aircraft Title: Boeing 747-8i Asobo, Category: Airplane, ...
📴 Stream closed (simulator disconnected)
⏳ Waiting 5 seconds before reconnecting...
```

## Code Explanation

### Data Structure

The example defines an `AircraftData` struct that maps to SimConnect variables:
- Position (latitude, longitude, altitude)
- Orientation (heading, pitch, bank)
- Speed (ground, indicated, true airspeed)
- Aircraft details (title, category, livery)
- Status (on ground, surface type, runway status)
- ATC information (ID, airline)

### Connection Lifecycle

The `runConnection()` function handles a single connection session:
1. Connects with retry logic
2. Registers data definitions for aircraft monitoring
3. Spawns AI traffic from JSON configuration
4. Sets up periodic data requests (every 5 seconds)
5. Processes incoming messages in a loop
6. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

### Traffic Management

- **Parked aircraft**: Created immediately at specified airports
- **Enroute aircraft**: Created with configurable delays using `time.AfterFunc()`
- **Flight plans**: Loaded from `.pln` files in the `plans/` directory

### Data Requests

The example requests data for all aircraft within 25km using:
```go
client.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

This triggers `SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE` messages with detailed aircraft information.

## Notes

- Aircraft titles must exactly match installed aircraft in the simulator
- Flight plan files must be valid `.pln` format
- The 25km radius can be adjusted by changing the third parameter in `RequestDataOnSimObjectType`
- The example demonstrates both parked and enroute AI traffic creation patterns
