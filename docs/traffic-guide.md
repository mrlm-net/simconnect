---
title: "Traffic Guide"
description: "Create and manage AI aircraft using the pkg/traffic package."
order: 1
section: "traffic"
---

# Traffic Guide

> **Ground routing:** taxi routes between stands and runways come from
> [pkg/airport](airport-layout.md), and [Departure Taxi](traffic-taxi.md) drives an AI
> aircraft along them (pushback, taxi, hold short, take-off). This guide covers the
> underlying `Fleet` and waypoint helpers. For the raw Engine `AICreate*` calls the
> Fleet wraps, see [Engine AI Objects](engine-ai-objects.md).

The rest of `pkg/traffic` builds on the `Fleet`:

| API | Does | Chapter |
|---|---|---|
| `TaxiController` | one departure: stand to take-off | [Departure Taxi](traffic-taxi.md) |
| `ArrivalController` | one arrival: approach to stand | [Arrivals & Parking](traffic-arrival.md) |
| `Injector`, `GroundMover`, `Detail`, `IDBlocks` | injected movement, level of detail, IDs for many aircraft | [Injected Ground Movement](traffic-motion.md) |
| `ApproachSequencer`, `HoldStack` | landing order, spacing, delays and holds | [Airborne Separation](traffic-separation.md) |
| `TrafficPicture` | all traffic around a centre, ours and the simulator's | [Traffic Picture](traffic-picture.md) |
| `Schedule()`, `Overflights()` (functions over a `ScheduleConfig`) | timetables for the focus airports, flights crossing the area | [Traffic Schedules](traffic-schedules.md) |
| `TrafficManager` | runs the timetable through your `Spawner`: spawns, turnarounds, situation checks | [Traffic Manager](traffic-manager.md) |

The runway in use comes from [`nav.RunwaySelector`](nav-weather.md#keeping-the-runway-in-use).

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

`Phase` (the SDK's `dFlightPlanPosition`) is the waypoint index plus the fraction along the next leg: `2.5` starts halfway between waypoints 2 and 3. It is not a 0–1 fraction of the route. In MSFS 2024 the phase does not take effect: tested live (#369), every enroute ATC aircraft appeared on the ground at its plan's departure airport, whatever the phase, and the simulator refused one (`CREATE_OBJECT_FAILED`) whose departure airport it had not loaded. To have an aircraft appear airborne mid-route, create it with `RequestNonATC` at the position and give it the rest of the route as waypoints: `traffic.EnrouteStart` (see [Traffic Manager](traffic-manager.md)).

```go
err := mgr.TrafficEnroute(traffic.EnrouteOpts{
    Model:        "FSLTL A321 Iberia SL",
    Livery:       "",
    Tail:         "IBE001",
    FlightNumber: 1,
    FlightPlan:   "C:/Plans/LEMD-LEBL.pln",
    Phase:        0.0,   // waypoint index + fraction along the next leg
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

// forget everything without asking the simulator (ObjectIDs already stale)
mgr.Fleet().Clear()
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

- **The Fleet itself does no routing:** `SetWaypoints` flies the coordinates you
  give it. Taxi routes come from the taxiway graph in [pkg/airport](airport-layout.md)
  (sized to the aircraft, runway entries and exits), and the
  [departure](traffic-taxi.md) and [arrival](traffic-arrival.md) controllers turn
  them into movement.
- **Ground traffic awareness is for injected aircraft:** they queue behind each other,
  give way where routes cross or merge and hold a pushback while traffic passes
  behind the stand (`GroundPicture`, see [Ground traffic](traffic-taxi.md#ground-traffic)).
  Aircraft MSFS AI taxis do not take part.
- **Arrival sequencing is opt-in:** `ApproachSequencer` gives each arrival its
  landing order and delay, and an `ArrivalController` on its STAR loses the delay when
  told to (`AbsorbDelay`, `EnterHold`; see [Airborne Separation](traffic-separation.md)).
  A plain `Fleet` aircraft is not sequenced.
- **ObjectIDs reset on reconnect:** Any aircraft spawned before a disconnect are
  lost. Re-spawn after reconnect if persistence is required.
