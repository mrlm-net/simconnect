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

## Injected departure

With `TaxiWithInjector(inj)` the controller drives the whole departure by position injection instead of MSFS AI waypoints: pushback, taxi, line-up, take-off and the initial climb. It uses the same injected ground driving as arrivals ([Injected Ground Movement](traffic-motion.md)), so turns, speeds, runway-crossing holds and lights follow the same rules. Feed every message to both the injector and the controller.

```go
inj := traffic.NewInjector(client)
ctl := traffic.NewTaxiController(fleet, traffic.TaxiWithInjector(inj))
ctl.Start(traffic.TaxiRequest{Graph: g, Parking: c22, Runway: "24", Entry: "B", Model: model})
// message loop: inj.Handle(msg); ctl.Handle(msg)
```

| State | What happens | Gate |
|---|---|---|
| `TaxiAwaitingPushback` | parked on the stand (nav lights) | `ClearPushback` |
| `TaxiPushback` | beacon on, then pushed tail first 3 s later: to the taxiway junction and on along the taxiway away from the taxi direction, so it ends facing the way it will taxi | |
| `TaxiAwaitingTaxi` | pushed back, engines starting | `ClearToTaxi` |
| `TaxiTaxiing` | taxi light on, moving 1.5 s later; take-off flaps set; stops short of runway crossings (`ClearToCross`) | |
| `TaxiHoldingShort` | nose 7 m before the departure runway's hold-short line, no strobes (`HoldingShortOf`) | `ClearToLineUp`, `ClearForTakeoff` |
| `TaxiLiningUp` | strobes on; along the entry taxiway's own path onto the runway, aligned 80 m down the centreline | |
| `TaxiLinedUp` | line up and wait | `ClearForTakeoff` |
| `TaxiDeparting` | landing lights on; take-off roll, rotation at Vr, lift-off, climb; gear up above 50 ft (taxi light off); flaps retract from 1000 ft | |
| `TaxiComplete` | handed to MSFS AI at 1500 ft with climb waypoints | |

- **Gates:** with `HoldForClearances` every gate holds until its clearance. Without it, each gate clears itself after a short, varied wait (`PushbackDelay`, `TaxiAfterPushDelay`, `LineUpDelay`, `TakeoffDelay`). A clearance given before its gate means no stop: `ClearForTakeoff` while taxiing gives a rolling take-off.
- **Rolling take-off:** without held gates, `RollingTakeoffChance` (default 30%) of departures get line-up and take-off together.
- **Runway entry:** `TaxiRequest.Entry` departs from a runway entry ("24 at B", see [runway entries](airport-layout.md)); empty means full length.
- **Take-off model:** `TakeoffMover` (`TakeoffProfile`; A320 defaults lift off after about 1550 m at 146 kt).

## Example

[`examples/ai-taxi`](../examples/ai-taxi) runs the whole sequence (LKPR C22 → runway 24 by default) and prints progress. Press Enter or pass `-takeoff-after` to clear the aircraft for take-off.

Options:
- `-inject` drives it by injection.
- `-gates` holds at every gate; Enter gives the next clearance.
- `-entry B` departs from an entry.
