---
title: "Arrivals & Parking"
description: "Land an AI aircraft, roll out to a runway exit and taxi it to a stand with the pkg/traffic ArrivalController: MSFS AI, hybrid or fully injected."
order: 3
section: "traffic"
---

# Arrivals & Parking

`traffic.ArrivalController` lands one AI aircraft and taxis it to a stand: spawn on final, approach, touchdown, rollout to a runway exit, a stop clear of the runway, taxi-in along a route from [`pkg/airport`](airport-layout.md) and parking on the stop mark. It is the counterpart of the [departure `TaxiController`](traffic-taxi.md) and shares its injected ground driving ([Injected Ground Movement](traffic-motion.md)).

## Three modes

| Mode | Option | Who flies / drives |
|---|---|---|
| MSFS AI | — | MSFS AI flies one waypoint chain from final to the vacate stop, then a taxi-in chain to the stand |
| Hybrid | `ArrivalWithInjector(inj)` | MSFS AI flies and lands; the injector takes over on the runway (≤ 70 kt) or, failing that, clear of it, and drives the rollout, exit, taxi-in and parking |
| Injected | `ArrivalWithInjector(inj)` + `ArrivalRequest.InjectApproach` | The injector flies the approach, flare and touchdown too; MSFS AI never flies the aircraft |

Only the injected modes keep the lights as set, drive smooth turns and stop exactly on the mark; MSFS AI switches lights itself and taxis at 6–9 kt. `InjectApproach` without an injector fails `Start` with `ErrBadTaxiRequest`.

## Lifecycle

```
Start ─► spawning ─► approaching ─► landing ─► rollout ─► vacating ─► awaiting taxi ─► taxiing ─► parking ─► parked
                                                                           ▲             │ ▲
                                                             ClearToTaxi / ClearUpTo     ▼ │ ClearToCross
                                                                                    holding short
                                                                     Cancel (any time) ─► cancelled
                                                                        errors ─► failed
```

| State | Meaning |
|---|---|
| `ArrivalSpawning` | `AICreateNonATCAircraft` sent, waiting for the object ID |
| `ArrivalApproaching` | On final |
| `ArrivalLanding` | Below `LandingAGLFt` (200 ft) |
| `ArrivalRollout` | On the runway after touchdown; `Touchdown` and `TouchdownFpm` are set |
| `ArrivalVacating` | Off the runway, rolling to the vacate stop |
| `ArrivalAwaitingTaxi` | Stopped clear of the runway, waiting for the taxi clearance or the after-landing dwell |
| `ArrivalTaxiing` | Taxiing to the stand |
| `ArrivalHoldingShort` | Stopped short of a runway crossing (`HoldingShortOf`), waiting for `ClearToCross` |
| `ArrivalParking` | On the last stretch onto the stand |
| `ArrivalParked` | Stopped on the stand; the controller stops tracking the aircraft |
| `ArrivalCancelled` / `ArrivalFailed` | Terminal. `ErrNoTouchdown` if the aircraft passes the runway end + 300 m without touching down |

`ArrivalState.Terminal()` is true from `ArrivalParked` on, and `Events()` is closed then.

## Usage

```go
fleet := traffic.NewFleet(client)
inj := traffic.NewInjector(client)
ctl := traffic.NewArrivalController(fleet, traffic.ArrivalWithInjector(inj))

g, _ := cache.Graph("LKPR")
stand, _ := g.Layout.ParkingIndex("C22")
err := ctl.Start(traffic.ArrivalRequest{
    Graph:            g,
    Runway:           "24",
    Parking:          stand,
    Model:            "FSLTL A320 Air France SL", // container title of an installed aircraft
    Tail:             "CSA456",
    InjectApproach:   true, // fly the approach by injection too
    HoldForClearance: true, // wait clear of the runway for ClearToTaxi / ClearUpTo
    HoldAtCrossings:  true, // stop short of every runway crossing
})
plan := ctl.Plan() // exit, taxi-in route, vacate stop, stop mark

for {
    select {
    case msg := <-client.Stream():
        if ok, err := inj.Handle(msg); ok {
            if err != nil { log.Print(err) }
            continue
        }
        ctl.Handle(msg)
    case ev, ok := <-ctl.Events():
        if !ok {
            return // parked, cancelled or failed
        }
        switch ev.State {
        case traffic.ArrivalAwaitingTaxi:
            ctl.ClearToTaxi() // or ctl.ClearUpTo(node) for a progressive taxi
        case traffic.ArrivalHoldingShort:
            ctl.ClearToCross()
        }
    }
}
```

`ArrivalEvent` carries the state, object ID, position, height above ground (`AGL`), heading, ground speed, on-ground flag, `Touchdown` (metres past the threshold) and `TouchdownFpm`, distance `Remaining` to the stand, the current `Taxiway`, `HoldingShortOf`, the progressive-taxi `LimitNode` / `AtLimit`, the `Lights` the sim reports, and `Err`. From touchdown (hybrid) or from the start (injected) the aircraft is read every sim frame; progress events are then limited to one a second, while state and light changes are always sent. A warning with `ErrTaxiStuck` is sent if the aircraft stands still for `StuckTimeout` (90 s) on the ground.

`ArrivalRequest` options not shown above: `Livery`, `SpawnNm` (default `DefaultSpawnNm`, 5 nm; below 2 nm the default is used), `Exit` (force a `*airport.RunwayExit`), `Options` (`airport.RouteOptions`; `Via` and `Taxiways` give a [custom taxi-in](airport-layout.md#custom-routes-via-points-and-taxiways)), `NoseOffset`, `AfterLandingDwell`, `RollThroughChance`, `Profile` (`MotionProfile`), `Rollout` (`RolloutProfile`), `Approach` (`ApproachProfile`), `Procedure` (see [STAR and approach](#star-and-approach)), `Aircraft` (an `AircraftProfile`, see [Aircraft Profiles](traffic-profiles.md)), `Airport` (`airport.Limits`, see [Airport limits](airport-layout.md#airport-limits)), and for MSFS AI comparisons `GroundAGL` and `NoStopWaypoint`.

`Cancel()` removes the aircraft at any point, also once it has parked. A controller is single use; each uses 4 definition IDs and 4 request IDs (`ArrivalWithIDs(defBase, reqBase)`, defaults 7500 / 7600). One `Injector` serves all controllers. The other options are `ArrivalWithSeed` ([Natural timing](traffic-motion.md#natural-timing)), `ArrivalWithGroundPicture` ([Ground traffic](traffic-taxi.md#ground-traffic)) and `ArrivalWithDetail` ([Level of detail](traffic-motion.md#level-of-detail)); for many arrivals, take the ID blocks from `IDBlocks` ([IDs for a long session](traffic-motion.md#ids-for-a-long-session)).

## Plan

`PlanArrival(graph, runwayEnd, parking, ArrivalOptions)` builds the plan `Start` uses and can be called on its own (the airport map's arrival mode does):

- **Exit:** among the exits the aircraft can reach at its exit speed (`Along ≥ RequiredRollout(ExitSpeed(e))`: touchdown `TouchdownMeters` (300 m) past the threshold at 125 kt, then braking at 1.5 m/s² — about 1.6 km for a high-speed exit, 1.7 km for any other), the one with the lowest cost: taxi-in length, plus 0.3 × the distance down the runway, plus 3 m for each degree of every turn sharper than 30°. The last exit always counts as reachable.
- **Route:** `Graph.RouteFromRunway(exit, parking, opts)`: the exit path off the runway, then on in the direction the exit leaves it to the stand. The runway being vacated is not a crossing.
- **Vacate stop** (`VacateIndex`): just past the hold-short behind the exit when the route passes it, else the first route point `VacateOffsetMeters` (100 m) from the centreline.
- **Stop mark** (`Stop`): `StandPoint` for the stand (see [Stands](#stands)).
- **Waypoints** for MSFS AI: approach points every nautical mile on a 3° path crossing the threshold at 50 ft, an on-ground touchdown point, rollout points every 250 m slowing to the exit speed at the exit, the roll clear to the vacate stop (`Waypoints`), and the taxi-in chain (`TaxiWaypoints`).

## Runway exits

`Graph.RunwayExits("24")` lists the taxiways leaving a runway for landings on that end, nearest the threshold first; `Graph.ExitFor("24", rollout)` returns the first at least `rollout` metres down it (the last exit when none is).

| `RunwayExit` field | Meaning |
|---|---|
| `RunwayNode`, `Node`, `Path` | Where the exit leaves the centreline, its first node off the runway surface, the node path between |
| `Along` | Metres from the landing threshold to `RunwayNode` |
| `Angle`, `Side` | Turn-off angle from the landing direction (0–180°) and side (`ExitLeft` / `ExitRight`) |
| `HighSpeed` | `Angle ≤ HighSpeedExitAngle` (45°) |
| `Taxiway` | The exit's name |
| `HoldShort` | Hold-short node for this runway behind the exit (within 300 m), −1 if none |

- Exits turning more than `MaxExitAngle` (90°) point back towards the threshold and are left out.
- Taxiway segments can run along the runway before turning off, so several runway nodes can lead to the same exit node. The shortest path is kept: its runway node is where the taxiway really leaves the centreline.
- **Naming:** the last named edge on the exit path. An unnamed connector takes the name of the named taxiway it continues onto most straight (EDDM: an unnamed link joins A4 to 08L/26R).

## MSFS AI arrival

Without an injector, the controller spawns the aircraft on final, sets its gear handle down (without it MSFS AI never touches down), sends the landing chain, and watches it once a second:

- **Vacating** once the aircraft is off the runway surface (half-width + `RunwayClearMeters`) near or past the planned exit.
- **Vacate stop:** near it the chain is replaced by a single waypoint where the aircraft is, so the AI does not creep on. Once stopped within `VacateArriveMeters` it waits for `ClearToTaxi` (`HoldForClearance`) or `AfterLandingDwell` (default 15 s), then gets the taxi-in chain.
- **Stand:** the aircraft is stopped the same way within `StandStopMeters` (2 m) of the stop mark, or once it passes it. It counts as parked when stationary within `ParkedMeters` (15 m); stopped further away for 10 s it parks with an error giving the distance.
- "Stopped" means the position moved less than 1 m in 3 s: MSFS AI keeps reporting its last commanded ground speed.

## Hybrid arrival

With `ArrivalWithInjector(inj)` MSFS AI flies the approach and touchdown. From touchdown the aircraft is read every sim frame and the injector watches the ground height under it.

- **Takeover on the runway** once the aircraft has been on the ground `TakeoverAfterTouchdown` (2 s), has slowed to `TakeoverKts` (70 kt) and is still at least `TakeoverBeforeExitMeters` (150 m) before the exit. The mover starts at the nose gear with the aircraft's heading and speed, so nothing jumps.
- **Takeover clear of the runway** if the aircraft reaches the exit first. If the takeover fails, a warning is sent and MSFS AI carries on.

## Injected approach

With `InjectApproach` the aircraft spawns exactly where the injected approach starts and is taken over at once: gear down (`Injector.SetGear`), flaps at `ApproachFlapsPct` (flaps 3), approach lights. `ApproachMover` (pure computation, `ApproachProfile`) flies it, and `Injector.PlaceAir` places each `ApproachPose`.

| `ApproachProfile` | A320 default |
|---|---|
| `GlideSlopeDeg` / `ThresholdHeightFt` | 3° / 50 ft (main wheels over the threshold) |
| `StartKts` → `ApproachKts` from `ApproachSpeedNm` | 150 → 135 kt from 1 nm, lag `SpeedTimeSeconds` 3 s |
| `TouchdownKts` | 130 |
| `FlareFt` / `TouchdownFpm` | 30 ft / −120 fpm |
| `ApproachPitchDeg` / `FlarePitchDeg` | 2.5° / 5.5° |
| `DerotateSeconds` / `DerotateDecel` | 4 s / 0.5 m/s² |

- **Phases** (`ApproachPhase`): `ApproachFinal` on the glide path; `ApproachFlare` below `FlareFt`, sink rate easing to `TouchdownFpm` while the pitch rises; `ApproachDerotate` on the main wheels, the nose coming down; `ApproachDone` with all wheels down.
- **Flaps** run to full over `FlapsFullSeconds` (5 s) when passing `FlapsFullFt` (1400 ft), set before the 1000 ft stabilised-approach gate. Once clear of the runway they retract over `FlapsRetractSeconds` (20 s), with the spoilers stowing and the strobes and landing lights off (the after-landing flow); the taxi light follows `TaxiLightDelay` later. `ArrivalController.Sequence()` lists the landing step by step (takeover on final, gear, flaps, flare, touchdown, nose down, vacated) with time, height and speed.
- **Spoilers** deploy over `SpoilerDeploySeconds` (1 s) at main-gear touchdown (`Injector.SetSpoilers`) and stow once clear of the runway.
- **Hand-over:** at `ApproachDone` the injected rollout continues from exactly that pose.
- Thrust reversers cannot be shown on an AI aircraft.

Measured live at LKPR runway 24: touchdown 486 m past the threshold at −120 fpm.

## STAR and approach

With `InjectApproach` and `Procedure` (e.g. `airport.Procedures.Arrival(runway, entryFix)`: the STAR from its entry fix and the best approach via the transition where the STAR ends) the arrival starts in the terminal area (#315):

```go
route, _ := procs.Arrival("06", "GOLOP") // GOLO4T → ILS 06 via KUVIX
ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: "06", Parking: stand, Model: model,
    InjectApproach: true, Procedure: route})
```

- The aircraft appears at the STAR's first fix at 250 kt and MSFS AI flies the procedure as waypoints. Altitudes descend 3° (`ProcedureDescentFtPerNm`) back from the join point, at most `ProcedureTopFt` (10 000 ft) unless the procedure asks for more, within every point's constraints (the approach's 4000 ft minimums at LKPR). Speed 250 kt, 180 kt from the IAF.
- The procedure's points on the final are replaced by two centreline points: aligned `ProcedureAlignNm` before the join point, and the join point `ProcedureJoinNm` (8 NM) out.
- At the join point (within `JoinCaptureMeters`, or established on the centreline abeam it) the injected approach takes over from where the aircraft is; the offset between MSFS AI's position and the injected glide path fades out over `JoinBlendSeconds`, so nothing jumps.
- From there on it is the injected approach below.

**Sequencing and delays.** On its STAR the arrival can be spaced behind the others: an `ApproachSequencer` gives it a landing time and a delay, `AbsorbDelay(delay)` loses the delay by flying slower and a dog-leg, and `HoldFix` / `EnterHold` / `LeaveHold` send it round a hold for what is left. `GoAround()` sends an injected arrival on final round the circuit ([ATC Commands](traffic-commands.md)). All of this is in [Airborne Separation](traffic-separation.md).

## Rollout and exit

`ArrivalRequest.Rollout` is a `RolloutProfile` per aircraft type (zero = `DefaultRolloutProfile()`, A320). The aircraft brakes hard to `SlowKts`, then slows gently and evenly, reaching the exit speed exactly at the exit; clear of the runway it slows to taxi speed.

| `RolloutProfile` | A320 default |
|---|---|
| `BrakeDecel` | 2.5 m/s² (down to `SlowKts`; jerk `RolloutJerk` 0.6 m/s³) |
| `SlowKts` | 80 |
| `HighSpeedExitKts` / `ExitKts` | 32 / 12 |
| `ExitLateralAccel` | 1.5 m/s² (cornering through the exit) |

## After landing

- **Vacate stop:** the aircraft stops at the vacate point (or as soon as comfortably possible when already past it) and waits for `ClearToTaxi` / `ClearUpTo` (`HoldForClearance`) or the after-landing dwell, which varies by ±10 % (`DwellJitter`). A `ClearToTaxi` given earlier takes effect as soon as the aircraft has stopped.
- **Roll-through:** without `HoldForClearance`, `RollThroughChance` of arrivals (default `DefaultRollThroughChance`, 30 %; negative never) only slow to `RollThroughKts` (0.5 kt) there and taxi on, like a rolling clearance.
- **Runway crossings:** with `HoldAtCrossings` the aircraft stops its nose gear `HoldShortStopMeters` (7 m) before the hold-short line of every runway it crosses and reports `ArrivalHoldingShort` with `HoldingShortOf`. `ClearToCross()` sends it on. Given earlier, it clears the next crossing ahead, so the aircraft does not stop; each call clears one crossing. Without `HoldAtCrossings` crossings count as cleared in advance.

## Progressive taxi

`ClearUpTo(node airport.NodeID) error` clears an injected arrival to taxi up to a node of its route and hold there, as a controller would give "taxi via A, hold short of B".

- The node must be on `Plan().Route.Nodes`; otherwise `ErrNotOnRoute`. Without an injector: `ErrNotInjected`.
- **Before the taxi-in** (from the approach to `ArrivalAwaitingTaxi`) it is the taxi clearance with a limit. The vacate stop still comes first; at `ArrivalAwaitingTaxi` the taxi starts at once. A node that is not ahead of the aircraft when the taxi-in starts (for example on the exit path) holds it where it is (see the last point below).
- **While taxiing or holding short** it moves the limit. The node must lie ahead on the current path (within 15 m of it); a node behind the aircraft returns `ErrNotOnRoute`. In `ArrivalParking` and later it always does.
- The nose gear stops on the node, or `HoldShortStopMeters` before it when the node is a hold-short. The aircraft stops at the nearer of the limit and the next uncleared crossing.
- `ArrivalEvent.LimitNode` is the current limit (−1 for none) and `AtLimit` is set while the aircraft holds there; an event is sent when it arrives and when it moves on.
- A later `ClearUpTo` moves the limit on; `ClearToTaxi()` removes it.
- A limit given before the taxi-in starts (during the approach or rollout) that is behind the aircraft by then (on the exit path) holds it clear of the runway and is reported as an `ArrivalEvent` with `Err` wrapping `ErrNotOnRoute`.

## Stands

`StandPoint(parking, noseOffset)` is where an aircraft stands on a parking spot. MSFS stands are circles sized for the largest aircraft allowed, and aircraft park with the nose at the front of the circle (by the jetway and stop mark). The reference point is therefore `Radius − noseOffset` ahead of the circle's centre along the stand heading, not at the centre. `NoseOffset` is the reference-point-to-nose distance (default `DefaultNoseOffsetMeters`, 17 m, A320 family). Arrivals stop there and departures spawn there.

- **Nose-in stands:** the injected path ends with 25 m straight along the stand axis, the last `StandSlowMeters` (30 m) at `StandTaxiSpeedKts` (5 kt), with the reference point on the stop mark. The state is `ArrivalParking` over the last 55 m.
- **Face-out (self-manoeuvring) stands:** when the lead-in junction lies *ahead* of the parked aircraft (LKPR N50–N58 and the S stands), the aircraft comes off the lead-in, swings out to the side with fewer other stands within 60 m, loops round behind the stop mark (scaled by `TurnAroundMeters`, 18 m) and comes back along the centreline facing out, with about three wheelbases of straight so the main gear lines up. This is decided per route.
- **Parked:** the aircraft stays frozen on the stand under the injector. `Injector.Release` hands it back to MSFS AI; `Cancel` removes it.

The sweep tests fly 44 injected arrivals across LKPR stands and runways; all park within 3° of the stand heading and 1 m of the stop mark.

### Stand allocation

`StandAllocator` assigns stands at one airport and tracks who is on them (#292):

```go
stands := traffic.NewStandAllocator(client, g)          // feed stands.Handle(msg); call stands.Scan() every ~10 s
s, err := stands.Assign(traffic.StandRequirements{
    Owner: "DLH1394", Airline: "DLH", Runway: "24",     // HalfSpan 0 = A320
})
// … ArrivalRequest{Parking: s, …}; stands.ReleaseOwner("DLH1394") when the aircraft is gone
```

- **Reservations:** `Occupy(stand, owner, halfSpan)` fails with `ErrStandTaken` when someone else holds the stand or an aircraft on an overlapping stand (`Layout.ParkingConflicts`) is in the way: two aircraft clash when their half spans plus `StandWingtipClearanceMeters` (3 m) exceed the distance between the stand centres. `Release`, `ReleaseOwner`.
- **Detection:** `Scan` requests every aircraft around the airport (AI and the user). One on the ground below `StandDetectKts` within a stand's RADIUS holds that stand, with its real `WING SPAN`. A reservation and a detection on the same stand merge (the owner stays, the object ID and span come from the scan).
- **Assign:** suitable stands (`Layout.SuitableStands` for the span, optional `Types`), the airline's own stands first, then stands open to every airline; ranked by taxi-in length from the arrival runway's best exit. `ErrNoStand` when none is free.
- **Taxi routes:** `ReserveRoute(owner, nodes)` returns the owners whose reserved routes share a node (a warning; spacing on the ground is the `GroundPicture`, see [Ground traffic](traffic-taxi.md#ground-traffic)). `ReleaseRoute`.
- `Occupancy()` is a snapshot for maps and ATC.

## Lights

Injected (hybrid after the takeover, injected throughout):

| Phase | Lights |
|---|---|
| Injected approach, rollout on the runway | nav, beacon, strobes, landing (logo on) |
| Clear of the runway | strobes off |
| Vacate stop (or the slowest point when rolling through) | landing off, taxi on `TaxiLightDelay` (1.5 s) later |
| Crossing a runway | strobes and landing on from `CrossingOnMeters` (10 m) past the first hold-short line until the main gear is `CrossingTailMeters` (40 m) past the opposite one |
| Parked | nav (beacon and taxi off) |

Logo and wing lights otherwise stay as the aircraft had them. With MSFS AI the controller sets approach, rollout, after-landing (taxi, beacon, nav) and parked (beacon, nav) lights once per phase; the AI overrides them within about a second (see [Injected Ground Movement](traffic-motion.md)). `ArrivalEvent.Lights` always reports what the sim shows.

## Example

[`examples/ai-arrival`](../examples/ai-arrival) lands an aircraft (LKPR runway 24 → C22 by default), prints the plan (exit, taxi-in route, crossings), each state change, light changes and progress. Ctrl+C removes the aircraft.

```bash
go run ./examples/ai-arrival -inject -inject-approach -hold-crossings
```

| Flag | Default | Description |
|---|---|---|
| `-icao` / `-runway` / `-stand` | `LKPR` / `24` / `C22` | Airport, runway end to land on, stand label |
| `-model` / `-livery` / `-tail` | `FSLTL A320 Air France SL` / default / `CSA456` | Aircraft |
| `-spawn-nm` | 5 | Distance out on final to spawn |
| `-inject` | `false` | Hybrid: position injection takes over during the rollout |
| `-inject-approach` | `false` | With `-inject`: fly the approach, flare and touchdown by injection too |
| `-hold` | `false` | Hold clear of the runway until Enter (taxi clearance) |
| `-dwell` | 0 (15 s) | After-landing stop before taxiing on |
| `-roll-through` | 0 (0.3) | With `-inject`: chance of a rolling clearance at the vacate point; negative never |
| `-hold-crossings` | `false` | With `-inject`: hold short of runway crossings; the demo clears after `-cross-after` |
| `-cross-after` | 15 s | Demo crossing clearance delay |
| `-nose` | 0 (17 m) | Reference-point-to-nose distance |
| `-keep` | `false` | Leave the parked aircraft in the sim on exit |
| `-ground-agl` / `-no-stop` | `false` | MSFS AI comparisons: ground waypoints at 0 ft AGL; no active stop at the stand |

[`cmd/airport-map`](../cmd/airport-map) spawns arrivals and departures from the map and gives every clearance, including progressive taxi by clicking a route point.
