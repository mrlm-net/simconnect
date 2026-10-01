---
title: "Departure Taxi"
description: "Spawn an AI aircraft at a stand, push it back and taxi it to a runway with the pkg/traffic TaxiController."
order: 2
section: "traffic"
---

# Departure Taxi

`traffic.TaxiController` drives one AI aircraft from a parking stand to a runway: spawn, pushback, taxi along a route from [`pkg/airport`](airport-layout.md) to the hold-short point, and — once cleared — line-up and take-off. The way back in is [Arrivals & Parking](traffic-arrival.md).

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

`TaxiEvent` carries the state, object ID, position, heading, ground speed, on-ground flag, distance `Remaining` to the hold-short along the route, the current `Taxiway` name, `HoldingShortOf`, the progressive-taxi `LimitNode` / `AtLimit`, `HeightFt` during the take-off, the `Lights` the sim reports, and `Err` for failures and warnings. Progress updates are dropped rather than blocking `Handle` if you stop reading `Events()`; state changes use reserved buffer space. A warning event with `ErrTaxiStuck` is sent if the aircraft stands still for `StuckTimeout` (90 s) during pushback or taxi.

`Cancel()` removes the aircraft at any point, also after the departure completed and MSFS AI flies it. A controller is single use; run several aircraft with one controller each and distinct IDs via `TaxiWithIDs(defBase, reqBase)` (each uses 2 definition IDs and 4 request IDs; defaults 7300 / 7400). For a long session, hand the blocks out with `IDBlocks` ([IDs for a long session](traffic-motion.md#ids-for-a-long-session)); for many aircraft at once, share a `Detail` with `TaxiWithDetail` ([Level of detail](traffic-motion.md#level-of-detail)). The airport map does both for every aircraft it drives.

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
| `TaxiPushback` | beacon on, then pushed tail first 3 s later on a push [fitted to the stand](#injected-pushback), ending aligned on the taxiway and facing the way it will taxi; face-out stands skip it | |
| `TaxiAwaitingTaxi` | pushed back, engines starting | `ClearToTaxi` |
| `TaxiTaxiing` | taxi light on, moving 1.5 s later; take-off flaps set; stops short of runway crossings (`ClearToCross`) | |
| `TaxiHoldingShort` | nose 7 m before the departure runway's hold-short line, no strobes (`HoldingShortOf`) | `ClearToLineUp`, `ClearForTakeoff` |
| `TaxiLiningUp` | strobes on; along the entry taxiway's own path onto the runway, aligned 80 m down the centreline | |
| `TaxiLinedUp` | line up and wait | `ClearForTakeoff` |
| `TaxiDeparting` | landing lights on; take-off roll, rotation at Vr with a small pull (the pitch stays `TailstrikeMarginDeg` below `TakeoffProfile.TailstrikePitch` on the runway), lift-off, pitch held until a positive climb (`PositiveClimbFt`) then up to the climb pitch no faster than the tail clears the runway; gear up on a positive climb (above 50 ft, `GearUpDelaySeconds` after lift-off, climbing at `GearUpFpm`: about 4 s and 100–200 ft up; taxi light off); climb speed (V2 + 10) to the acceleration altitude (`TakeoffProfile.AccelFt`, default `TakeoffAccelFt` 1000 ft), then acceleration to the clean speed (`CleanKts`, default climb speed + 50 kt) with the flaps retracting on the speed schedule (`TakeoffProfile.FlapsShare`); handed to MSFS AI clean, above `ClimbHandoverFt`. `TaxiController.Sequence()` lists the take-off step by step (roll, rotation, lift-off, gear, flaps, hand-over) with time, height and speed. `TakeoffProfileFor(model)` gives per-family figures (777-300 tail strike 8.5°, A320 11.5°, …) | |
| `TaxiComplete` | handed to MSFS AI at 1500 ft with climb waypoints | |

- **Gates:** with `HoldForClearances` every gate holds until its clearance. Without it, each gate clears itself after a short, varied wait (`PushbackDelay`, `TaxiAfterPushDelay`, `LineUpDelay`, `TakeoffDelay`). A clearance given before its gate means no stop: `ClearForTakeoff` while taxiing gives a rolling take-off.
- **Runway gates only:** `TaxiRequest.HoldForRunway` holds only at the gates onto a runway (line-up, take-off and runway crossings) until their clearance; pushback and taxi go by themselves. It is for a runway controller that clears the runway's users (#393, see [Airborne Separation](traffic-separation.md)).
- **Pushback time:** `TaxiRequest.PushbackAt` (without `HoldForClearances`) keeps the aircraft on its stand until then, e.g. its scheduled departure time ([Traffic Schedules](traffic-schedules.md)); zero pushes after the usual short wait. `HoldPushback(true)` keeps it on the stand, even when cleared, until `HoldPushback(false)`; the airport map uses it for the [Traffic Manager's](traffic-manager.md#situation-checks) ground stop. Once the beacon is on, the push goes ahead.
- **Rolling take-off:** without held gates, `RollingTakeoffChance` (default 30%) of departures get line-up and take-off together.
- **Runway entry:** `TaxiRequest.Entry` departs from a runway entry ("24 at B", see [runway entries](airport-layout.md)); empty means full length. An entry with less runway ahead than the aircraft needs is refused (`ErrEntryTooShort`): `RequiredTakeoffRun(profile, conditions)` flies the type's take-off (`TakeoffProfile`) to 35 ft, lengthens it about 10% per 1000 ft of elevation and 1% per °C above ISA (`TakeoffConditions`), and adds the 15% certification margin (`TakeoffRunMargin`); at LKPR an A320 needs about 2330 m (06 at E, not at C or D) and a 777-300 about 3680 m (full length only).
- **Custom route:** `TaxiRequest.Options` is an `airport.RouteOptions`, so `Options.Via` (nodes to pass, in order) and `Options.Taxiways` ("via B, A") give a [custom taxi route](airport-layout.md#custom-routes-via-points-and-taxiways). The pushback planning keeps them: the taxi-out it replans from the push junction drops only the via points and taxiways the push already passed (`Graph.RemainingOptions`). A route that cannot follow them, or that the aircraft does not fit, fails `Start` with the `*airport.RouteError`. `ArrivalRequest.Options` (and `ArrivalOptions.Route` for `PlanArrival`) does the same for the taxi-in from the runway exit.
- **Take-off model:** `TakeoffMover` (`TakeoffProfile`; A320 defaults lift off after about 1550 m at 146 kt).
- **Face-out stands:** when the route's lead-in junction lies ahead of the parked aircraft's nose gear (within 60° of its heading), there is no pushback: after `ClearPushback` (the start-up approval) the aircraft taxis straight out. A junction ahead of the stand's reference point but under the aircraft (EDDF B10, KJFK A15) is not: that stand is pushed. It is decided once, when the departure starts.

### Injected pushback

The push is planned to where it ends, not from the shape of the stand's lead-in line: lead-ins are the way in (at EDDF every one is one-way onto the stand), and a push is neither their reverse nor their mirror. The planner first chooses the **pose** the push ends in — the nose gear on a taxiway, the aircraft aligned with it, facing the way it taxis out — then the path the tug pushes to it. The aircraft moves at a tug's walking pace (`PushbackSpeedKts`, 3 kt) with gentle speed changes, the fuselage along the path (`NewPushbackMover`, see [Injected Ground Movement](traffic-motion.md#pushback)).

- **Poses:** every 5 m along every taxiway edge within 150 m of the stand, both ways, and up to 20 m short of an edge's start on its line (the nose before the lane begins, the tail on the apron behind: LKPR A7, C17, C31). Never on runways, paths along a runway, stand lead-ins, or the branches of a forked lead-in (a Y onto one taxiway, LFPG M stands). A taxiway too narrow for the type (`KnownTaxiwayMaxSpan`, a 777 not on LKPR JO) is skipped. A pose on a single unnamed stem off a stand costs 400 more, so a real taxiway wins where one is reachable (LFPG M15 onto U). A pose on a lane without wingtip clearance from the stands beside it costs 500 more.
- **The push to a pose:** `PushStraightMeters` (6 m) straight back off the stand, then the shortest path with a turn radius from `PushbackArcMeters` (45 m) down in 4 m steps while above `PushbackMinArcMeters` (14 m), so 17 m is the tightest tried; the cheapest radius per pose. It must be at most 150 m long; turn the aircraft at most 200° in all and no more than 60° beyond the turn from the stand's heading to the pose's (no loops); keep the main gear within 3 m of the pavement (stand circles, taxi path strips, and 45 m of apron past the end of a lane), else within 8 m if no push does; and keep the tail and wingtips no deeper into a neighbouring stand (or the terminal zone) than the parked aircraft already reaches, or 1 m into it. Stands that overlap the aircraft's own (`Layout.ParkingConflicts`) are no neighbours: nobody parks there while it is taken.
- **Choosing:** of the pushes that fit, the cheapest by the push (each meter costs 3 m of taxiing), the taxi-out from the pose (`Graph.RouteToRunwayFrom`, not turning back), 400 per junction of another taxiway under the aircraft at the pose (not its own stand's), 1000 for a taxi-out that turns back within 200 m, and 2000 for one that turns off the nose at the start (more than 20° within 10 m, or 45° within 20 m: a turn from a standstill). The departure route then starts on the pose's taxiway edge.
- **Push and pull:** where no push alone leaves the aircraft facing its way out (none, or the taxi-out would turn off the nose or back on itself), the tug pushes it to a pose and tows it forward to another. The tow is at most 80 m and ends 20 m straight so the main gear lines up, with the same stand and pavement checks (KJFK D70). A push onto the taxiway facing the runway is never turned round by a tow.
- **Stands around:** `TaxiRequest.StandOccupied` (e.g. `StandAllocator.Occupant`) tells which neighbouring stands are taken. The push may swing through an empty one (EHAM U26: straight back onto C facing south when the small stands in front are empty), never through a taken one or the terminal ahead of a gate. The push is planned at `Start` and planned again when it begins if a neighbouring stand has been taken or freed since. Without it, every neighbour counts as taken.
- **Manual pushback:** the same path generator serves a pose given by ATC ("push onto L facing west").

Where no pose is reachable (a few percent of stands, e.g. remote stands more than 120 m from a taxiway), the older plans below apply.

The older push is fitted to the stand's surroundings. The main gear starts at the stand's stop mark (`StandPoint`), goes straight back along the stand axis, turns onto the taxiway on an arc and follows the taxiway centreline until the aircraft is aligned (`NewArcPath`).

- **Which way the tail goes:** decided by where the aircraft can go from there. For every taxiway branch at the junction the push can swing onto (at most 100° off the push direction), the taxi-out is planned from the junction facing away from that branch (`Graph.RouteToRunwayFrom`); the cheapest wins and the departure route becomes stand → junction → that taxi-out. (The route planned from the stand could continue straight ahead of the push, which left LKPR C17 facing away from its route.) Up to 120 m of the chosen taxiway is used, following its straightest continuation. A junction more than 3 m off the stand axis where no arc fits is pushed to abeam, straight.
- **Up the alley:** where the only taxiway at the junction is the way out (a dead-end stand: LKPR A7, B9, C26), the tug pushes the aircraft back out along that taxilane to the next taxiway and swings the tail there, as at a real pier. The push turns off the stand onto the lane's first straight stretch on the widest Dubins curve that fits (bends right beside the stand leave no room for arcs cut into the short segments), follows the lane (its general line: points within 2 m of it are dropped), then swings onto the branch. Each meter pushed costs 3 m of taxiing when choosing where to swing, so short pushes win where they work.
- **Pavement and terminal:** every push stays on the pavement, the stand circles and the taxi path strips (half their `WIDTH` either side), because buildings and grass are not in the scenery data; and out of the terminal zone, the 40 m beyond the parked noses of the gates, which face the building.
- **Push and turn:** the last resort where no alley push fits: `PushStraightMeters` straight back, then a Dubins path to a pose facing along the taxi-out, preferably past the junction on the taxiway itself, otherwise short of it (penalised); radii from `PushbackArcMeters` down to `PushbackMinArcMeters`, the cheapest push clear of neighbours, pavement and terminal wins.
- **Straight part:** at least `PushStraightMeters` (6 m) straight back along the stand axis before the turn.
- **Arc:** the widest radius between `PushbackMinArcMeters` (14 m) and `PushbackArcMeters` (45 m) that both the push line up to the taxiway and the straight run of the taxiway beyond the junction allow. The straight run is the longest stretch the centreline stays within 1.5 m of a straight line, so a jog at the junction does not hide a long straight taxiway.
- **Neighbouring stands:** the tail (`MotionProfile.TailMeters` behind the main gear) and the wingtips (`SpanMeters` / 2 either side) are checked against every other stand's circle along the push. While the swing would reach more than 1 m deeper into a neighbouring stand than the parked aircraft already does, the arc tightens in 2 m steps; if no radius clears, the least intrusive one is used.
- **End:** aligned on the straight taxiway, `PushAlignMeters` (10 m) past the arc and at least a wheelbase plus `PushTailMeters` (20 m) beyond the junction, but never into the bend after the straight run.
- **Dead-end stands** (no branch behind the junction to swing onto) push straight back along the stand axis to abeam the junction; the taxi then starts with the turn onto the taxiway.
- A push turning less than 3°, or a corner that does not fit (the stand axis meets the taxiway line less than 2 m or more than 150 m behind the gear, or beyond the straight run), goes through the junction and on along the taxiway.

**Tug.** `TaxiRequest.Tug` shows a pushback tug. The departure calls `Attach` while the aircraft waits for its pushback (the tug connects before the clearance), `Update` on every frame (with `pushing` while on the stand and during the push, then with `pushing` false until `Done`) and `Remove` on cancel or failure. `SimObjectTug` is the built-in one: `NewSimObjectTug(client, inj, DefaultTugTitle, reqID, profile)` spawns the ground vehicle model (`DefaultTugTitle`, `FSDT_Pushback_03`; other `FSDT_Pushback_*` titles and liveries such as `…_CZ` work too) `TugAheadMeters` ahead of the nose gear, lets the injector freeze and place it on the nose gear through the push, waits `TugDisconnectSeconds`, drives off (`TugDriveOffMeters` forward, then `TugDriveOffTurnDeg` to the side) and removes it. Any other implementation of `PushbackTug` (a GSX integration, for example) can take its place.

**Tug from its depot.** With `SimObjectTug.Layout` set (the airport map does), the tug appears at the vehicle parking spot nearest the stand (`airport.Layout.VehicleDepots`, `TAXI_PARKING_TYPE_VEHICLE`). It drives along the vehicle roads at `TugRoadKts` (15 kt) to a point `TugApproachMeters` (12 m) in front of the nose, then straight onto the nose gear, facing the aircraft. The push waits until it is there (`Connected`, `TugArriveTimeout` 4 min, then the push goes on without it). After the push it backs off, drives back over the stand the aircraft has left and onto the vehicle road behind it, then home to the depot, where it is removed. It does not drive out along the taxiway among the aircraft. `airport.Layout.VehicleRoute(a, b)` finds the way. Each end joins the closest point of the vehicle road nearest it, straight across the apron, when one is within `VehicleRoadReachM` (150 m); otherwise it joins at the nearest node. In between, VEHICLE and ROAD paths count at their length, and aprons and taxiways (PATH, TAXI, PARKING) at `vehicleOffRoad` (4) times theirs. It never uses a runway or a closed path. `NearVehicleRoad(p)` says whether a road is in reach. At LKPR, 76 stands reach a road this way, and the way from the nearest depot to B9 is 514 m (388 m straight). An airport without a depot or a vehicle road (EDDM) keeps the tug at the nose and the drive-off to the side.

## SID after take-off

`TaxiRequest.Departure` (e.g. `airport.Procedures.ResolveSID(name, runway, "", departureEnd, elevation)`, optionally followed by the rest of a flight plan) is flown by MSFS AI after the injected climb hands over at `ClimbHandoverFt` (#315). `DepartureWaypoints` skips points behind the aircraft, climbs `ProcedureClimbFtPerNm` up to `ProcedureTopFt` (or the highest constraint) within every point's constraints at 250 kt, and continues along the last track so MSFS AI does not turn back after the last fix. Without it the aircraft climbs straight ahead (`TakeoffClimb`).

**Airport limits.** `TaxiRequest.Airport` (e.g. `airport.LimitsFor(layout, &procedures)`, see [Airport limits](airport-layout.md#airport-limits)) replaces `ClimbHandoverFt` with the airport's hand-over height from its SIDs' initial climb, and caps the injected taxi speed at `TaxiMaxKts` and at `ApronMaxKts` along the edges at stand junctions where the motion profile is faster; `ArrivalRequest.Airport` does the same for the taxi-in.

## Turnaround

`TaxiRequest.ObjectID` adopts an aircraft already on the stand instead of spawning one (#293) — e.g. one an `ArrivalController` parked: the departure takes it over from there (pushback, taxi, take-off). The airport map chains both as a turnaround (#296): an arrival with *Turnaround* departs again after its dwell (±20 %) or the *Depart now* action, with the same call sign and stand.

## De-icing

`TaxiRequest.Deice` de-ices the departure (#323):

- **On the stand** (`Deicing{}` without a pad): once cleared to push, the aircraft is treated on the stand first (`TaxiEvent.Deicing`), then the beacon comes on and it pushes back.
- **At a pad** (`Deicing{Pad: &airport.DeicingPad{…}}`): the route passes the pad's taxi node (a via point); the aircraft stops there with engines running and the taxi light off, is treated, and taxis on. The stop is its own hold: a taxi clearance does not skip it.

The treatment takes `Dwell` (default `DefaultDeicingDwell`, 6 min, varied by `DwellJitter`). MSFS facility data has no de-icing pads, so they come from `airport.Limits.DeicingPads` (the airport map lets you pick them from the taxi points: Airport → De-icing pads); `nav.IcingConditions(weather)` says when de-icing is due (at or below +3 °C with visible moisture).

## Ground traffic

Injected aircraft share a `GroundPicture` (#334): `TaxiWithGroundPicture(p)` and `ArrivalWithGroundPicture(p)` with one picture for all the controllers at an airport. Every aircraft reports its reference point, heading and airframe (`MotionProfile`) each frame while on the ground; others can be added with `GroundPicture.Report` (the airport map adds the sim's own AI and the user's aircraft from its traffic scan).

While taxiing (a departure) or off the runway (an arrival), an aircraft looks `TrafficLookMeters` (200 m) ahead along its path every `TrafficCheckEvery` (0.1 s). When another aircraft's body (nose to tail, sampled every 5 m) lies within its half span of the path, it brakes to a stop with its nose `TrafficGapMeters` (15 m) behind that body, and moves on as the other moves. So aircraft queue at a holding point one behind the other instead of on top of each other, follow slower traffic at a gap, and wait for an aircraft pushed back onto their taxiway. Reports older than `TrafficStaleAfter` (3 s) are ignored; a departure leaves the picture on its take-off roll, a cancelled one at once.

Where two taxi routes cross or merge, each aircraft reports where it will drive next (up to its next stop, `GiveWayLookMeters`); where the paths come within both half-spans plus `GiveWayMarginMeters`, the aircraft closer to the conflict goes and the other stops short of it. A pushback waits, even when cleared, while another aircraft's fuselage is within its half-span plus `PushClearMarginMeters` of the corridor the push sweeps (the push path and the tail beyond its end), or while another aircraft's taxi path crosses it; under way it has priority: it reports what it still has to sweep (`GroundPicture.ReportPush`), taxiing traffic whose path crosses that gives way to it, and the push stops only for an aircraft actually in the way. `TaxiEvent.PushbackHeld` reports it (#334).

Facing an aircraft coming the other way, an aircraft does not queue up to the gap behind it: it stops where its body keeps the last junction before the other aircraft clear, a half-span plus `GiveWayMarginMeters` from the junction's other branches, so the other can turn off there (#444).

## Progressive taxi

`ClearUpTo(node airport.NodeID) error` clears an injected departure to taxi up to a node of its route and hold there ("taxi via A, hold short of B"). [Arrivals](traffic-arrival.md#progressive-taxi) have the same call.

- The node must be on `Route().Nodes` after the stand; otherwise `ErrNotOnRoute`. Without `TaxiWithInjector`: `ErrNotInjected`.
- **Before the taxi starts** (`TaxiAwaitingPushback`, `TaxiPushback`, `TaxiAwaitingTaxi`) it is the taxi clearance with a limit. A node that is no longer ahead when the taxi starts holds the aircraft where it is (see the last point below).
- **While taxiing or holding short** it moves the limit. The node must lie ahead on the current path; a node behind the aircraft returns `ErrNotOnRoute`.
- The nose gear stops on the node, or `HoldShortStopMeters` (7 m) before it when the node is a hold-short. The aircraft stops at the nearer of the limit and the next uncleared runway crossing. The state stays `TaxiTaxiing`.
- `TaxiEvent.LimitNode` is the current limit (−1 for none) and `AtLimit` is set while the aircraft holds there; an event is sent when it arrives and when it moves on.
- A later `ClearUpTo` moves the limit on; `ClearToTaxi()` removes it and clears the aircraft to the runway.
- A limit given before the taxi starts (during the pushback) that is no longer ahead when it starts holds the aircraft where it is and is reported as a `TaxiEvent` with `Err` wrapping `ErrNotOnRoute`: give a new `ClearUpTo` or `ClearToTaxi`.

```go
r := ctl.Route()
ctl.ClearUpTo(r.Nodes[5]) // taxi, hold at the 5th route node
// ... later
ctl.ClearToTaxi()        // on to the departure runway
```

## Example

[`examples/ai-taxi`](../examples/ai-taxi) runs the whole sequence (LKPR C22 → runway 24 by default) and prints progress. Press Enter or pass `-takeoff-after` to clear the aircraft for take-off.

Options:
- `-inject` drives it by injection.
- `-gates` holds at every gate; Enter gives the next clearance.
- `-entry B` departs from an entry.
