---
title: "Traffic Guide"
description: "Create and manage AI aircraft using the pkg/traffic package."
order: 1
section: "traffic"
---

# Traffic Guide

> **MVP notice:** `pkg/traffic` reflects current knowledge of the SimConnect AI aircraft API.
> Ground routing (taxiway graph, pathfinding) is not yet implemented — callers supply
> explicit waypoint coordinates. The API surface will grow in later milestones as
> simulator behaviour is better understood.

The `pkg/traffic` package provides a typed, thread-safe abstraction over SimConnect's AI
aircraft creation and management API. It handles the async create→acknowledge lifecycle
and groups active aircraft into a `Fleet` that can be queried and cleaned up at once.

## Aircraft Kinds

SimConnect supports three creation modes, represented by `traffic.AircraftKind`:

| Kind | How created | Movement |
|---|---|---|
| `KindParked` | `AICreateParkedATCAircraft[EX1]` | ATC-controlled, placed at a gate |
| `KindEnroute` | `AICreateEnrouteATCAircraft[EX1]` | ATC-controlled, following a flight plan |
| `KindNonATC` | `AICreateNonATCAircraftEX1` | Waypoint chain — caller controls movement |

## Fleet

`Fleet` is the central collection. Create one once and reuse it across
the lifetime of your application. The manager creates and owns one automatically —
access it via `mgr.Fleet()`.

```go
import "github.com/mrlm-net/simconnect/pkg/traffic"

// standalone (without manager)
fleet := traffic.NewFleet(engineClient)

// via manager — fleet is created and lifecycle-managed internally
fleet := mgr.Fleet()
```

### Thread safety

All `Fleet` methods are safe to call from concurrent goroutines (message handler,
ticker, signal handler, etc.). Internally a `sync.RWMutex` guards the maps.

### Lifecycle across reconnects

SimConnect ObjectIDs are invalidated whenever the connection drops. `Fleet.SetClient`
handles this: it replaces the internal client reference **and** discards all pending
and member state. The manager calls this automatically on every connect and disconnect.

If you create a `Fleet` outside the manager you must call it yourself:

```go
// on connect
fleet.SetClient(newClient)

// on disconnect
fleet.SetClient(nil)
```

## Creation Pattern (async)

Aircraft creation in SimConnect is asynchronous. You issue a create request with a
`reqID` of your choice, then receive `SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID` back
with the same `reqID` and the new `ObjectID`. `Fleet` models this as two steps:

### Step 1 — Request

```go
const reqSpawn uint32 = 5001

err := mgr.TrafficParked(traffic.ParkedOpts{
    Model:   "FSLTL A320 Air France SL",
    Livery:  "",       // "" selects the model default
    Tail:    "AFR123",
    Airport: "LFPG",
}, reqSpawn)
if err != nil {
    log.Println("spawn failed:", err)
}
```

### Step 2 — Acknowledge

In your `OnMessage` handler, watch for the assigned-object message and call
`Fleet.Acknowledge` (or the manager's wrapper):

```go
mgr.OnMessage(func(msg engine.Message) {
    if msg.Err != nil { return }
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {
    case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
        assigned := msg.AsAssignedObjectID()
        aircraft, ok := mgr.Fleet().Acknowledge(assigned.DwRequestID, assigned.DwObjectID)
        if ok {
            log.Printf("spawned: %s (objectID=%d)", aircraft.Tail, aircraft.ObjectID)
        }
    }
})
```

`Acknowledge` returns `(nil, false)` when the `reqID` was not issued by this fleet,
so it is safe to call for every assigned-object message even if other subsystems also
create objects.

## Parked Aircraft

Parked aircraft are placed at an airport gate and managed by the simulator's ATC.

```go
err := mgr.TrafficParked(traffic.ParkedOpts{
    Model:   "FSLTL B737 Ryanair SL",
    Livery:  "",
    Tail:    "EIN400",
    Airport: "EIDW",
}, 5002)
```

To assign a flight plan after spawning:

```go
err := mgr.TrafficSetFlightPlan(objectID, "C:/Plans/EIDW-EGLL.pln", 5003)
```

## Enroute Aircraft

Enroute aircraft follow a `.PLN` flight plan file from a given phase offset.

```go
err := mgr.TrafficEnroute(traffic.EnrouteOpts{
    Model:        "FSLTL A321 Iberia SL",
    Livery:       "",
    Tail:         "IBE001",
    FlightNumber: 1,
    FlightPlan:   "C:/Plans/LEMD-LEBL.pln",
    Phase:        0.0,   // 0.0 = start, 1.0 = end
    TouchAndGo:   false,
}, 5010)
```

## Non-ATC Aircraft with Waypoints

Non-ATC aircraft ignore ATC and follow an explicit waypoint chain. This is the only
mode that supports ground movement sequences (pushback → taxi → takeoff).

### Spawn at an explicit position

```go
err := mgr.TrafficNonATC(traffic.NonATCOpts{
    Model:  "FSLTL A320 SAS SL",
    Livery: "",
    Tail:   "SAS202",
    Position: types.SIMCONNECT_DATA_INITPOSITION{
        Latitude:  50.1008,
        Longitude: 14.2600,
        Altitude:  1247,       // ft MSL (Václav Havel elevation)
        Heading:   258,
        OnGround:  1,
        Airspeed:  0,
    },
}, 5020)
```

### Release control, then set waypoints

After `Acknowledge`, you must release simulator control before the aircraft will
follow your waypoints:

```go
// release — objectID came from Acknowledge
if err := mgr.TrafficReleaseControl(objectID, 5021); err != nil {
    log.Println("release failed:", err)
}

// build waypoint chain
wps := []types.SIMCONNECT_DATA_WAYPOINT{
    traffic.PushbackWaypoint(50.1008, 14.2595, 1247, 3),
    traffic.TaxiWaypoint(50.1000, 14.2580, 1247, 15),
    traffic.LineupWaypoint(50.0982, 14.2560, 1247),
}
wps = append(wps, traffic.TakeoffClimb(50.0982, 14.2560, 258)...)

// defID must be registered for "AI Waypoint List" (see below)
if err := mgr.TrafficSetWaypoints(objectID, defWaypoints, wps); err != nil {
    log.Println("waypoints failed:", err)
}
```

The `defWaypoints` define ID must be registered once at connect time:

```go
mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    if new == manager.StateConnected {
        mgr.AddToDataDefinition(defWaypoints, "AI Waypoint List",
            "", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, defWaypoints)
    }
})
```

## Waypoint Helpers

All helpers are in `pkg/traffic`. Flags are set correctly for each manoeuvre type —
do not compose waypoints by hand unless you need something not covered here.

| Helper | Flags | Typical use |
|---|---|---|
| `PushbackWaypoint(lat, lon, alt, kts)` | `ON_GROUND \| REVERSE \| SPEED_REQUESTED` | Reverse from gate |
| `TaxiWaypoint(lat, lon, alt, kts)` | `ON_GROUND \| SPEED_REQUESTED` | Forward ground roll |
| `LineupWaypoint(lat, lon, alt)` | `ON_GROUND \| SPEED_REQUESTED` at 5 kts | Final runway threshold node |
| `ClimbWaypoint(lat, lon, altAGL, kts, throttle%)` | `SPEED_REQUESTED \| THROTTLE_REQUESTED \| COMPUTE_VERTICAL_SPEED \| ALTITUDE_IS_AGL` | Airborne climb point |
| `TakeoffClimb(rwyLat, rwyLon, hdgDeg)` | — | Returns 3 `ClimbWaypoint`s at 1.5/5/12 nm |

The transition from the last `ON_GROUND` waypoint to the first airborne waypoint
triggers the simulator's takeoff roll.

## Fleet Management

```go
// count active (acknowledged) aircraft
n := mgr.Fleet().Len()

// snapshot — safe to iterate outside the lock
for _, a := range mgr.Fleet().List() {
    fmt.Printf("%s  objectID=%d  kind=%d\n", a.Tail, a.ObjectID, a.Kind)
}

// look up by object ID
if a, ok := mgr.Fleet().Get(objectID); ok {
    fmt.Println(a.Tail)
}

// remove one
mgr.TrafficRemove(objectID, reqID)

// remove all — reqIDBase is incremented per aircraft
mgr.Fleet().RemoveAll(9000)
```

## Manager Integration

All `Fleet` operations are available as thin wrappers on the manager so you never
need to import `pkg/traffic` directly in simple applications:

```go
mgr.TrafficParked(opts, reqID)
mgr.TrafficEnroute(opts, reqID)
mgr.TrafficNonATC(opts, reqID)
mgr.TrafficRemove(objectID, reqID)
mgr.TrafficReleaseControl(objectID, reqID)
mgr.TrafficSetWaypoints(objectID, defID, waypoints)
mgr.TrafficSetFlightPlan(objectID, planPath, reqID)
mgr.Fleet()   // direct fleet access for List/Get/Len/RemoveAll
```

The manager keeps the fleet's client reference in sync with the connection state —
you do not need to call `SetClient` yourself.

## Error Reference

| Error | Meaning |
|---|---|
| `traffic.ErrNotConnected` | No active engine client (not connected) |
| `traffic.ErrObjectNotFound` | ObjectID is not tracked in the fleet |
| `traffic.ErrCreationFailed` | SimConnect creation call returned an error |
| `traffic.ErrEmptyWaypoints` | `SetWaypoints` called with nil or zero-length slice |

## Known Limitations

- **No ground routing:** Waypoint coordinates must be supplied by the caller.
  There is no taxiway graph or pathfinding — that is deferred to a later milestone
  once the full facility dataset for taxiways is understood.
- **No arrival sequencing:** Enroute aircraft land and park autonomously via ATC;
  custom arrival sequencing is not yet supported.
- **ObjectIDs reset on reconnect:** Any aircraft spawned before a disconnect are
  lost. Re-spawn after reconnect if persistence is required.
