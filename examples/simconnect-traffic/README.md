# SimConnect Traffic Example (Manager)

## Overview

This example spawns AI aircraft through the manager's traffic fleet (`mgr.Fleet()`, `pkg/traffic`): a parked ATC aircraft at LFPG and a non-ATC aircraft at LKPR that is released from AI control and sent a pushback → taxi → line-up → take-off waypoint chain. The manager handles connection and reconnection; Ctrl+C removes every aircraft the fleet created.

## What It Does

1. **Connects** — `manager.New` with auto-reconnect, a 512-message buffer and a 6 Hz heartbeat
2. **Registers the waypoint definition** — On `StateConnected`, adds data definition `2000` for `AI Waypoint List` (needed by `TrafficSetWaypoints`)
3. **Spawns two aircraft**
   - `mgr.TrafficParked` — `FSLTL A320 Air France SL`, tail `AFR001`, parked at LFPG
   - `mgr.TrafficNonATC` — `FSLTL A320 CSA SL`, tail `CSA100`, placed on the ground at LKPR (50.1008, 14.2600, heading 258)
   - An en route aircraft (`mgr.TrafficEnroute` with a `.pln` flight plan) is in the code, commented out: give it a plan path to try it
4. **Acknowledges the spawns** — On `ASSIGNED_OBJECT_ID`, `mgr.Fleet().Acknowledge` matches the request ID to the aircraft and prints its object ID
5. **Drives the non-ATC aircraft** — `mgr.TrafficReleaseControl`, then `mgr.TrafficSetWaypoints` with `traffic.PushbackWaypoint`, `TaxiWaypoint`, `LineupWaypoint` and `TakeoffClimb` for LKPR runway 24
6. **Logs the fleet** — Every 10 seconds, `mgr.Fleet().List()`: tail, object ID and kind of each aircraft
7. **Cleans up** — On Ctrl+C, `mgr.Fleet().RemoveAll` removes the aircraft, then the manager stops

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- The FSLTL A320 models named above, or edit the model titles in `spawnAircraft` to aircraft you have installed

## Running the Example

```bash
go run ./examples/simconnect-traffic
```

## Expected Output

```
SimConnect Traffic — pkg/traffic demo
Press Ctrl+C to remove all aircraft and exit
connection: Disconnected → Connecting
connection: Connecting → Connected
✅ spawned  tail=AFR001     objectID=...
✅ spawned  tail=CSA100     objectID=...
waypoints set  objectID=...  count=...
fleet: 2 aircraft
  tail=AFR001     objectID=... kind=...
  tail=CSA100     objectID=... kind=...
^C
shutting down — removing all AI aircraft...
goodbye
```

## Code Explanation

### Spawning through the fleet

```go
mgr.TrafficParked(traffic.ParkedOpts{
    Model:   "FSLTL A320 Air France SL",
    Tail:    "AFR001",
    Airport: "LFPG",
}, reqParked)

mgr.TrafficNonATC(traffic.NonATCOpts{
    Model: "FSLTL A320 CSA SL",
    Tail:  "CSA100",
    Position: types.SIMCONNECT_DATA_INITPOSITION{
        Latitude: 50.1008, Longitude: 14.2600, Altitude: 1247,
        Heading: 258, OnGround: 1,
    },
}, reqNonATC)
```

Request IDs (`5001`–`5005`) are kept below the manager's reserved range.

### Acknowledging and driving

```go
case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
    assigned := msg.AsAssignedObjectID()
    a, ok := mgr.Fleet().Acknowledge(uint32(assigned.DwRequestID), uint32(assigned.DwObjectID))
    if ok && a.Kind == traffic.KindNonATC {
        mgr.TrafficReleaseControl(a.ObjectID, reqRelease)
        sendWaypoints(mgr, a.ObjectID)
    }
```

The waypoints use rough LKPR coordinates. For real taxi routes from the airport's layout see [`ai-taxi`](../ai-taxi) and the [airport map](../airport-map).

## Related Examples

- [`ai-traffic`](../ai-traffic) — Parked and en route aircraft with the raw client
- [`manage-traffic`](../manage-traffic) — Parked and airborne aircraft on waypoints with the raw client
- [`monitor-traffic`](../monitor-traffic) — Read every aircraft within 25 km
- [`ai-taxi`](../ai-taxi) — A departure taxied on a real route with `pkg/traffic`

## See Also

- [Traffic Guide](../../docs/traffic-guide.md) — `Fleet`, spawn options and waypoints
- [Manager Usage](../../docs/usage-manager.md) — Manager lifecycle and callbacks
