# SimConnect State Example

## Overview

This example demonstrates how to track simulator state and access connection state information using the Manager interface. It shows how to monitor simulator pause/resume events, camera state changes, and aircraft data updates.

## What It Does

1. **Manages connection lifecycle** — Uses Manager for automatic connection and reconnection
2. **Tracks connection states** — Monitors state transitions (disconnected, connecting, connected, available)
3. **Subscribes to system events** — Monitors Pause, Sim, and Sound events
4. **Reads simulator state** — Accesses current camera state and pause status
5. **Requests periodic data** — Updates camera state every second
6. **Monitors nearby aircraft** — Requests detailed data for all aircraft within 10km radius

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see data updates)

## Running the Example

```bash
cd examples/simconnect-state
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
🔄 State changed: disconnected -> connecting
⏳ Connecting to simulator...
🔄 State changed: connecting -> connected
✅ Connected to SimConnect, simulator is loading...
✅ Setting up data definitions and event subscriptions...
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator'
  Application Version: 1.0
  Application Build: 1.0
🔄 State changed: connected -> available
🚀 Simulator connection is AVAILABLE. Ready to process messages...
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1001, Data: 1
  🏁 Simulator SIM STARTED
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA
  => Received SimObject data event
     Camera State: 2, Camera Substate: 0
```

## Code Explanation

### Key APIs Used

- **Manager** — Automatic connection lifecycle and state tracking
- **SubscribeToSystemEvent()** — Listen to system events (Pause, Sim, Sound)
- **AddToDataDefinition()** — Define data structures to request
- **RequestDataOnSimObject()** — Request periodic data updates
- **RequestDataOnSimObjectType()** — Request data for all objects of a type

### Event IDs

The example registers system events with specific IDs:

| ID | Event | Values | Description |
|----|-------|--------|-------------|
| `1000` | `Pause` | 0=unpaused, 1=paused | Simulator pause state |
| `1001` | `Sim` | 0=stopped, 1=started | Simulator running state |
| `1002` | `Sound` | 0=off, 1=on | Master sound state |

### Data Definitions

**Camera Data (Definition ID 2000):**
```
CAMERA STATE (int32)
CAMERA SUBSTATE (int32)
CATEGORY (string260)
```
Requested every second for the user aircraft.

**Aircraft Data (Definition ID 3000):**
```
Position, heading, altitude, speed, livery, and status information
```
Requested for all aircraft within 10km radius.

### Managing State Changes

```go
mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    fmt.Printf("State changed: %v -> %v\n", old, new)
})
```

## Related Examples

- [`simconnect-manager`](../simconnect-manager) — Basic manager setup with less data complexity
- [`simconnect-subscribe`](../simconnect-subscribe) — Channel-based subscriptions with Manager
- [`subscribe-events`](../subscribe-events) — Event subscription using direct engine client

## See Also

- [Manager Configuration](../../docs/config-manager.md) — Detailed configuration options
- [Manager & SimState Tracking](../../docs/config-manager.md#simulator-state-tracking) — SimState API documentation
