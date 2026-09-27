---
title: "Departure Taxi"
description: "Spawn an AI aircraft at a stand, push it back and taxi it to a runway with the pkg/traffic TaxiController."
order: 2
section: "traffic"
---

# Departure Taxi

`traffic.TaxiController` drives one AI aircraft from a parking stand to a runway: spawn, pushback, taxi along a route from [`pkg/airport`](airport-layout.md) to the hold-short point, and — once cleared — line-up and take-off.

## Lifecycle

```
Start ─► spawning ─► pushback ─► taxiing ─► holding short ─ ClearForTakeoff ─► lining up ─► departing ─► complete
                                                                     Cancel (any time) ─► cancelled
                                                                        errors ─► failed
```

| State | Meaning |
|---|---|
| `TaxiSpawning` | `AICreateNonATCAircraft` sent, waiting for the object ID |
| `TaxiPushback` | Waypoints sent; being pushed back from the stand |
| `TaxiTaxiing` | Moving forward along the route |
| `TaxiHoldingShort` | Stopped within `HoldShortArrivalMeters` of the hold-short point |
| `TaxiLiningUp` | Entering the runway after `ClearForTakeoff` |
| `TaxiDeparting` | Take-off roll (ground speed above 30 kt) |
| `TaxiComplete` | Airborne; the controller stops tracking the aircraft, which keeps flying its climb waypoints |
| `TaxiCancelled` / `TaxiFailed` | Terminal |

## Usage

Like `Fleet` and `airport.Loader`, the controller never reads the engine stream: your loop passes every message to `Handle`, and progress arrives on `Events()`.

```go
fleet := traffic.NewFleet(client)
ctl := traffic.NewTaxiController(fleet)

g, _ := cache.Graph("LKPR")
stand, _ := g.Layout.ParkingIndex("C22")
err := ctl.Start(traffic.TaxiRequest{
    Graph:   g,
    Parking: stand,
    Runway:  "24",
    Model:   "FSLTL A320 Air France SL", // container title of an installed aircraft
    Tail:    "CSA123",
})

for {
    select {
    case msg := <-client.Stream():
        ctl.Handle(msg) // returns true when the message was the controller's
    case ev, ok := <-ctl.Events():
        if !ok {
            return // terminal state reached
        }
        fmt.Println(ev.State, ev.Taxiway, ev.Remaining, ev.GroundSpeed)
        if ev.State == traffic.TaxiHoldingShort {
            ctl.ClearForTakeoff()
        }
    }
}
```

With `pkg/manager`, use its fleet and forward messages from a handler:

```go
ctl := traffic.NewTaxiController(mgr.Fleet())
mgr.OnMessage(func(msg engine.Message) { ctl.Handle(msg) })
```

`TaxiEvent` carries the state, object ID, position, heading, ground speed, on-ground flag, distance `Remaining` to the hold-short along the route, the current `Taxiway` name, and `Err` for failures and warnings. Progress updates are dropped rather than blocking `Handle` if you stop reading `Events()`; state changes use reserved buffer space. A warning event with `ErrTaxiStuck` is sent if the aircraft stands still for `StuckTimeout` (90 s) during pushback or taxi.

`Cancel()` removes the aircraft at any point. A controller is single use; run several aircraft with one controller each and distinct IDs via `TaxiWithIDs(defBase, reqBase)` (each uses 2 definition IDs and 4 request IDs; defaults 7300 / 7400).

## Waypoints

The controller sends two waypoint chains, which you can also build yourself:

- `TaxiWaypoints(graph, route)`: pushback and taxi to the hold-short.
- `LineUpWaypoints(graph, route)`: onto the runway centreline abeam the hold-short, `LineUpAlignMeters` down the runway, then `TakeoffClimb`.

Taxi legs are simplified (collinear points dropped), split to at most `MaxWaypointSpacingMeters`, and slowed before turns (`TurnSpeedKts` from 30°, `SharpTurnSpeedKts` from 60°) and over the last `HoldShortApproachMeters`. All tunables are in `pkg/traffic/tunables.go`.

## MSFS AI behaviour

Found by running the controller in MSFS 2024 at LKPR:

- **AI cannot steer while reversing.** A reverse leg that bends makes the aircraft spin or turn round and drive forward to the waypoint. Pushback is therefore a single straight `REVERSE` leg along the stand axis to the taxiway junction; the aircraft then turns onto the taxiway going forward. The first forward waypoint is at least `TurnInMeters` (50 m) away — a closer one makes the AI circle to reach it.
- **Taxi speed is capped by the AI** at roughly 6–9 kt, even when waypoints request 15 kt.
- **The spawn heading** is the stand's own `HEADING`, so the aircraft appears correctly parked.
- The line-up enters the runway abeam the hold-short, which is an intersection departure when the hold-short is down the runway.

## Example

[`examples/ai-taxi`](../examples/ai-taxi) runs the whole sequence (LKPR C22 → runway 24 by default) and prints progress; press Enter or pass `-takeoff-after` to clear the aircraft for take-off.
