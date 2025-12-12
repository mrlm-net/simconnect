# Read Messages Example

## Overview

This example demonstrates how to define data structures and read simulation variables from Microsoft Flight Simulator using the SimConnect SDK. It showcases data definition patterns, periodic data requests, and handling SimObject data for both the user aircraft and nearby traffic.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Defines data structures** - Creates data definitions for camera state and aircraft information
3. **Requests periodic data** - Retrieves camera state updates every second
4. **Monitors nearby aircraft** - Requests detailed data for all aircraft within 10km radius
5. **Processes data messages** - Handles SimObject data and data-by-type messages
6. **Handles reconnection** - Automatically reconnects if the simulator disconnects
7. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see data updates)

## Running the Example

```bash
cd examples/read-messages
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
  SimConnect Version: 12.0
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA
  => Received SimObject data event
     Request ID: 2001, Define ID: 2000, Object ID: 1
     Camera State: 2, Camera Substate: 0, Category: Airplane
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Request ID: 4001, Define ID: 3000, Object ID: 1
     Aircraft Title: Boeing 747-8i Asobo, Livery Name: Boeing House, ...
📴 Stream closed (simulator disconnected)
⏳ Waiting 5 seconds before reconnecting...
```

## Code Explanation

### Data Definitions

**Camera Data (Definition ID 2000)**:
```go
type CameraData struct {
    CameraState    int32      // Current camera view (2=cockpit, 3=external, etc.)
    CameraSubstate int32      // Camera sub-state
    Category       [260]byte  // Aircraft category string
}
```

Requested periodically:
```go
client.RequestDataOnSimObject(2001, 2000, types.SIMCONNECT_OBJECT_ID_USER, 
    types.SIMCONNECT_PERIOD_SECOND, ...)
```

**Aircraft Data (Definition ID 3000)**:
- Title, livery information
- Position (lat/lon/alt)
- Orientation (heading, pitch, bank)
- Speed (ground, indicated, true airspeed)
- Status (on ground, runway, surface type)
- ATC information

Requested for nearby aircraft:
```go
client.RequestDataOnSimObjectType(4001, 3000, 10000, 
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

### Message Processing

The example uses a switch statement to handle different message types:

- **SIMCONNECT_RECV_ID_OPEN** - Connection confirmation with version info
- **SIMCONNECT_RECV_ID_SIMOBJECT_DATA** - Periodic data updates (camera)
- **SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE** - Enumerated object data (nearby aircraft)

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic
2. Defines data structures
3. Requests periodic and one-time data
4. Processes messages in a loop
5. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

## Use Cases

This pattern demonstrates:
- Reading simulation variables periodically
- Tracking user's camera view
- Monitoring nearby traffic
- Building situational awareness tools
- Creating flight tracking applications
- Querying aircraft information by type

## Related Examples

- **subscribe-events** - For subscribing to system events (pause, sim state, sound)
- **emit-events** - For triggering simulator events
- **set-variables** - For writing simulation variables

## Notes

- Data definition order must match struct field order
- Use `engine.CastDataAs[T]()` to safely cast data buffers
- The 10km radius can be adjusted in `RequestDataOnSimObjectType`
- Periodic requests automatically send updates at the specified interval
- Use appropriate data types: `SIMCONNECT_DATATYPE_INT32`, `SIMCONNECT_DATATYPE_FLOAT64`, `SIMCONNECT_DATATYPE_STRING*`
