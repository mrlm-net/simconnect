---
title: "Injected Ground Movement"
description: "Drive AI aircraft on the ground by position injection: smooth turns, realistic speed changes and lights that stay as set."
order: 4
section: "traffic"
---

# Injected Ground Movement

MSFS AI taxis an aircraft along waypoints, but it keeps control of the lights (it switches taxi and landing lights back within about a second of any change) and its ground movement is jerky. `pkg/traffic` can instead move an aircraft itself, 60 times a second, by *position injection*:

- `GroundPath` and `GroundMover` compute the motion. They are pure computation with no SimConnect, so you can test them.
- `Injector` takes the aircraft away from the AI, freezes it and writes each position to the sim.

Both approaches can be combined per phase. For example, the AI flies the approach, landing and rollout, then the injector takes over clear of the runway for taxi-in and parking with correct lights.

## Motion model

```go
prof := traffic.DefaultMotionProfile()          // A320 family
path, err := traffic.NewGroundPath(route.Points, prof)
mover := traffic.NewGroundMover(path, prof)

pose := mover.Step(1.0 / 60)                    // every frame
// pose.Position, pose.Heading, pose.GroundSpeedKts, pose.Arrived
```

- **Corners** are rounded within `CornerMeters` (25 m) of each route point, with `GroundPathSmoothingPasses` (6) Chaikin passes.
- **Speed** is planned from the turn radius, `v = √(LateralAccel · r)`, with braking planned ahead of each turn. The aircraft is already at a turn's speed `TurnLookaheadMeters` before it.
- **Acceleration is jerk-limited** (`Jerk`), so every speed change starts and ends softly. The mover brakes exactly onto the end of the path.
- **Steering:** the nose gear follows the path and the main gear trails it at `WheelbaseMeters`, like a towed trailer. The fuselage points from the main gear to the nose, so the heading eases into and out of every turn, and the main gear cuts inside the turn as on a real jet. The geometry uses local metres; repeated bearing and displacement round trips drift and make the aircraft slide.
- **Holds:** `HoldAt(d)` stops the nose gear `d` metres along the path, for a hold-short line, traffic ahead or a stop bar. `ClearHold()` lets the aircraft continue.

| `MotionProfile` | A320 default |
|---|---|
| `WheelbaseMeters` | 12.6 |
| `RefAheadMeters` (sim reference point ahead of the main gear) | 1.0 |
| `CruiseKts` / `MinTurnKts` | 15 / 3 |
| `LateralAccel` | 0.6 m/s² |
| `Accel` / `Decel` / `Jerk` | 0.45 m/s² / 0.5 m/s² / 0.2 m/s³ |
| `SpanMeters` / `TailMeters` (main gear to the tail end; clearance checks such as the pushback swing past neighbouring stands) | 35.8 / 20.5 |

## Driving the aircraft

```go
inj := traffic.NewInjector(client)
inj.Takeover(objectID)                  // AI released, position/altitude/attitude frozen
inj.SetLights(objectID, traffic.LightsTaxi)

// every 1/InjectHz seconds:
pose := mover.Step(dt)
inj.Place(objectID, pose)               // ErrGroundUnknown until the ground height arrives

// in the message loop:
if ok, err := inj.Handle(msg); ok && err != nil { log.Print(err) }

inj.Release(objectID)                   // unfreeze
```

`Place` puts the aircraft on the ground, resting on its gear: ground altitude + `STATIC CG TO GROUND` at `STATIC PITCH` (requested every sim frame), or the height and pitch the sim showed it resting at before the first placement, and `MovingPitchDeg` (1°) nose down while moving, faded in up to `MovingPitchFullKts` (5 kt). `SetLights` sends only the lights that change. Presets: `LightsParked`, `LightsPushback`, `LightsTaxi`, `LightsRunway`.

Other calls on a taken-over aircraft: `SetGear`, `HoldGearDown`, `SetFlaps`, `SetSpoilers`, `SetEngines(id, n, on)` and `SetThrottle(id, n, percent)` for engines 1 to n (at most 4). An injected aircraft's engines follow the throttle and are heard as they spool: the controllers set `TakeoffThrottlePct` (90 %) for the take-off roll, `ApproachThrottlePct` (45 %) on an injected final and idle from the flare. `SetModel(id, title)` tells the injector the aircraft's title: rolling at 30 kt or more after a few placements, it learns the height the sim rests that model at and uses it for every aircraft of that title in the air (`PlaceAir`), so lift-off and touchdown do not jump. Until then a model is taken to rest `RestAboveStaticShare` (8.5 %) of its static CG height higher.

`Injector` uses 14 definition IDs, 2 request IDs per aircraft (up to 96 aircraft and tugs, see [Many aircraft](#many-aircraft)) and 10 event IDs from `DefaultInjectDefinitionBase` (7700), `DefaultInjectRequestBase` (7800) and `DefaultInjectEventBase` (7900); move them with `InjectorWithIDs`.

## Pushback

A pushback moves the aircraft tail first, and a tug swings the tail through a steady arc rather than steering by the nose gear:

```go
push := traffic.DefaultMotionProfile()
push.CruiseKts = traffic.PushbackSpeedKts             // 3 kt
path, err := traffic.NewArcPath(points, push, traffic.PushbackMinArcMeters)
mover := traffic.NewPushbackMover(path, push, standHeading)
```

- `NewArcPath(points, p, radius)` replaces each corner with a circular arc of `radius` (smaller where the segments are too short), instead of the Chaikin rounding of `NewGroundPath`.
- `NewPushbackMover(path, p, heading)` starts with the main gear at the start of the path and the nose along `heading` (the stand heading). The main gear follows the path backwards, and the fuselage lies along the path.
- The injected departure fits the push points to each stand (straight back, the widest arc the distances and the neighbouring stands allow, aligned on the taxiway); see [Injected pushback](traffic-taxi.md#injected-pushback). The clearance check uses `MotionProfile.SpanMeters` and `TailMeters`.

## Hybrid arrival

`ArrivalController` combines both approaches with `ArrivalWithInjector(inj)`. Feed every message to both the controller and the injector. [Arrivals & Parking](traffic-arrival.md) is the full guide (exits, clearances, progressive taxi, stands); this section covers the injected ground phase.

```go
inj := traffic.NewInjector(client)
ctl := traffic.NewArrivalController(fleet, traffic.ArrivalWithInjector(inj))
ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: model})
// message loop: inj.Handle(msg); ctl.Handle(msg)
```

1. **MSFS AI flies** the approach, touchdown and the first part of the rollout. From touchdown the aircraft is read every sim frame and the injector watches the ground height under it.
2. **Takeover on the runway:** once the aircraft has been on the ground for `TakeoverAfterTouchdown` (2 s) and slowed to `TakeoverKts` (70 kt), at least `TakeoverBeforeExitMeters` before the exit. The mover starts at the aircraft's nose gear with its heading and speed, so nothing jumps at the switch. If the aircraft reaches the exit first, the takeover happens once it is clear of the runway.
3. **Rollout and exit** (`ArrivalRequest.Rollout`, a `RolloutProfile` per aircraft type; A320 defaults). The aircraft brakes hard (2.5 m/s²) to 80 kt, then slows gently and evenly, reaching the exit speed at the exit: 32 kt at a high-speed exit, 12 kt at any other. Clear of the runway it slows to taxi speed. This is how crews fly it.
4. **Vacate stop:** the aircraft stops there and waits for `ClearToTaxi` (`HoldForClearance`) or the after-landing dwell, which varies by ±10 %. With `RollThroughChance` (default 30 %, only without `HoldForClearance`) it only slows to 0.5 kt and taxis on, like a rolling clearance.
5. **Runway crossings:** with `HoldAtCrossings` the aircraft stops with its nose gear `HoldShortStopMeters` before the hold-short line of every runway it crosses, reports `ArrivalHoldingShort` (with `ArrivalEvent.HoldingShortOf`), and waits for `ClearToCross()`. Runway lights stay off while it holds. A clearance given earlier means it does not stop. Without `HoldAtCrossings`, crossings count as cleared in advance. `ClearUpTo(node)` gives a progressive taxi: the aircraft holds at a route node until cleared further ([Progressive taxi](traffic-arrival.md#progressive-taxi)). Departure gates (pushback, taxi, line-up, take-off) are in [Injected departure](traffic-taxi.md#injected-departure).
6. **Taxi-in and parking:** the path ends straight along the stand axis, the last 30 m at 5 kt, with the reference point on the stop mark. On arrival the engines are shut down (`SetEngines`). The aircraft stays frozen on the stand; `Release` hands it back to MSFS AI.

Lights, all set by the controller once it has taken over:

| Phase | Lights |
|---|---|
| Rollout on the runway | nav, beacon, strobes, landing |
| Clear of the runway | strobes off |
| Vacate stop (or slowest point when rolling through) | landing off, taxi on `TaxiLightDelay` (1.5 s) later |
| Crossing a runway | strobes and landing on from just past the hold-short line before it until a moment after the tail has passed the opposite one |
| Parked | nav only (beacon and taxi off), engines off |

Logo and wing lights stay as the aircraft had them. `ArrivalEvent.Lights` reports what the sim shows. [`examples/ai-arrival`](../examples/ai-arrival) runs it with `-inject`; `-roll-through 1` forces a rolling clearance.

### Self-manoeuvring stands

Some stands face the taxilane: the lead-in junction the route uses lies *ahead* of the parked aircraft (LKPR N50–N58 and the S stands). A stand can have lead-ins on both sides, so this is decided per route.

- **Arrivals** take a custom turn-around route: they come in off the lead-in and swing out to the side with fewer neighbouring stands. They loop round behind the stop mark (scaled by `TurnAroundMeters`) and come back along the centreline, facing out, with about three wheelbases of straight so the main gear lines up.
- **Departures** from such stands start without a pushback: after the start-up approval (`ClearPushback`) the aircraft taxis straight out.

The sweep tests fly 44 injected arrivals across LKPR stands and runways. All park within 3° of the stand heading and 1 m of the stop mark.

## Injected approach

With `ArrivalRequest.InjectApproach` (and `ArrivalWithInjector`) MSFS AI does not fly at all. MSFS AI flies finals at a fixed ~165 kt, with no pitch and no flare, and its touchdowns measured −54 to −1214 fpm.

- The aircraft spawns on the injected glide path and is taken over at once, with gear down (`Injector.SetGear`), flaps full (`Injector.SetFlaps`) and approach lights.
- `ApproachMover` flies it:
  - a 3° glide path crossing the threshold at 50 ft;
  - speed easing from `StartKts` to `ApproachKts` by 1 nm;
  - a flare from 30 ft, with the sink rate easing to `TouchdownFpm` (−120) while the pitch rises from 2.5° to 5.5°;
  - after touchdown, the nose coming down over 4 s.
- Flaps are at `ApproachFlapsPct` (flaps 3) on final and run to full over 5 s when passing `FlapsFullFt` (1400 ft), set before the 1000 ft stabilised-approach gate.
- Ground spoilers come out over `SpoilerDeploySeconds` at main-gear touchdown (`Injector.SetSpoilers`).
- With the nose wheel down, the injected rollout takes over from exactly that pose.
- Once clear of the runway the spoilers stow and the flaps retract over `FlapsRetractSeconds`.
- Thrust reversers cannot be shown on an AI aircraft. The reverser nozzle SimVar is not settable, and the reverse-thrust events are ignored.

`Injector.PlaceAir` places an `ApproachPose`: the main wheels `HeightFt` above the ground, pitched nose up `PitchDeg`, and on the ground from touchdown. MSFS AI objects ignore the flaps handle and `FLAPS_*` events, so `SetFlaps` writes the flap surface positions directly. Ramp the percentage for a visible movement. Gear animates on a frozen aircraft.

Measured live at LKPR runway 24: touchdown 486 m past the threshold at −120 fpm. Pitch readback equals the command.

## Natural timing

Every wait and duration with a real-world counterpart varies a little from aircraft to aircraft (#343), so traffic never looks scripted. Each aircraft draws one factor per spread when its controller starts and keeps it for the whole flight — one crew is a little quicker than the next, not erratic. A spread of 0 gives exactly the tunable; `TaxiWithSeed` / `ArrivalWithSeed` make the draws reproducible.

| Spread | Default | Varies |
|---|---|---|
| `DwellJitter` | ±10 % | each gate wait (pushback, taxi, line-up, take-off) and the after-landing dwell |
| `BeaconLeadSpread` | ±30 % | `BeaconLeadTime`: beacon on to the push |
| `TaxiLightSpread` | ±30 % | `TaxiLightDelay`: taxi light to moving; after landing, landing lights off to taxi light on |
| `TugDisconnectSpread` | ±30 % | `TugDisconnectSeconds`: push done to the tug backing off (`SetDisconnectDelay`) |
| `FlapsSpread` | ±20 % | `FlapsSetSeconds`, `FlapsRetractClimbSeconds`, `FlapsRetractSeconds`, `FlapsFullSeconds` |
| `GearUpSpread` | ±20 % | `GearUpDelaySeconds` after lift-off |
| `TaxiSpeedSpread` | ±8 % | the taxi speed (never above the airport's `TaxiMaxKts`) |
| `PushbackSpeedSpread` | ±10 % | the pushback pace |

Rolling take-offs (`DefaultRollingTakeoffChance`) and roll-through vacates (`DefaultRollThroughChance`) are random per flight too; the airport map's turnaround dwell varies ±20 % and the ATC game's traffic interval ±30 %.

## Many aircraft

Dozens of injected aircraft at once (#370) cost little CPU. The load is the message traffic to the simulator: each aircraft answers its monitor and is placed once per frame it is driven.

`BenchmarkDepartureTaxiFrame` measures one sim frame of one taxiing departure: the monitor answer, the mover step, the look at the traffic ahead and the placement. It takes about **0.6 µs and 1.2 SimConnect writes**. At 60 frames a second, 40 aircraft cost 1.4 ms of CPU a second (0.14 % of a core), but make about 5 000 SimConnect messages a second. **Level of detail** cuts the messages.

### Level of detail

`traffic.Detail` decides how often each injected aircraft is driven. Share one Detail between the controllers (`TaxiWithDetail`, `ArrivalWithDetail`), and keep its viewer current with `SetViewer`, e.g. the user aircraft:

| Aircraft | Driven |
|---|---|
| on the runway (lining up, take-off, landing roll, vacating) | every frame |
| moving within `NearMeters` (8 km) of the viewer: the whole airport and its short finals | every frame |
| moving within `MidMeters` (20 km) | every 2nd frame (`MidInterval`) |
| moving farther away | every 4th frame (`FarInterval`, 15 Hz) |
| standing still (on the stand, holding, lined up to wait) | every 30th frame (`StillInterval`, 2 Hz) |

An aircraft speeds up at once, but only slows down after asking for fewer frames for `SlowerAfter` (2 s). The tug driving in, and the seconds before the push, count as moving. Standing still, an aircraft reacts to a clearance or to traffic ahead within half a second.

`Load()` reports the aircraft driven, their updates a second together, and how many run at every frame. The map shows it in Layers → Traffic picture.

### IDs for a long session

Every controller takes a block of data definition and request IDs. `IDBlocks` hands blocks out and takes them back when the aircraft is gone, so a session of hundreds of flights reuses a fixed range:

```go
ids := traffic.NewIDBlocks(20000, 30000, 10, 128) // 128 aircraft at once
def, req, err := ids.Acquire()                  // ErrNoIDs when all are in use
ctl := traffic.NewTaxiController(fleet, traffic.TaxiWithIDs(def, req), …)
// … when the aircraft is gone:
ids.Release(def)
```

A controller on a reused block clears its definitions before adding to them: the Fleet remembers which it defined on the connection. The injector drives up to 96 aircraft and tugs.

## Traffic time

MSFS AI flies its parts of a flight (STAR, SID, en route, holds) in simulator time. That follows the simulation rate (time acceleration, or slower) and stops while the simulator is paused. Injected motion and every timer must run on the same time, or at 2× an injected final lags the STAR before it and the landing sequence's times are wrong (#413).

`SimClock` is that time. It follows the wall clock at the simulation rate and stands still while paused, and a change of either takes effect without a jump:

```go
clock := traffic.NewSimClock()
clock.SetRate(rate)     // the simulator's SIMULATION RATE
clock.SetPaused(paused) // the "Pause" system event

taxi := traffic.NewTaxiController(fleet, traffic.TaxiWithClock(clock.Now) /* , … */)
arr := traffic.NewArrivalController(fleet, traffic.ArrivalWithClock(clock.Now) /* , … */)
manager.Tick(clock.Now()) // and the sequencer, the tower, the picture: pass it the same time
```

Call `clock.Frame()` on every simulator frame (`EVENT_FRAME`): a gap between frames longer than `SimHitch` (100 ms) is the simulator standing still, loading a model for example, and the clock counts only `SimHitch` of it. Driven by the wall clock, every injected aircraft jumped ahead after such a hitch (5–14 m at approach speed, measured live) while MSFS's own traffic paused with the simulator. `Now` never goes back: a time read during the gap is the least it gives after it.

A controller moves by the clock's time between frames, at most `MaxFrameStepSeconds` (1 s) a frame. At a high rate with fewer frames far away (level of detail) a frame can be a quarter of a second or more; a longer gap, a stall, is not made up at once.

The airport map reads `SIMULATION RATE` with the user aircraft every second and subscribes to "Pause". All its traffic runs on the clock: controllers, the schedule, sequencing, the tower, conflicts, spawn separation and the runway in use. The aircraft line shows "sim 2×" or "⏸ sim paused", and markers glide at the rate. Logs keep the wall clock.

## Measured in MSFS 2024

Live runs at LKPR (FSLTL A320, 1.3 km with three turns and a stop):

| | AI waypoints | Injection |
|---|---|---|
| Taxi light | turned off by the AI within ~1 s | on in every sample (15 874 of 15 874) |
| Position error | — | mean 0.04 m, max 0.27 m |
| Height over ground | bounces of about 1 m reported | constant (0.00 ft spread) |
| Speed | capped at about 6–9 kt, abrupt | planned: eased acceleration, slowing into turns, exact stop |

Events such as `FREEZE_*_SET` and `*_LIGHTS_SET` reach AI objects only with `SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY` = `0x10`. Earlier SDK versions had the wrong value (#310).

## Service vehicles under ATC

Tugs and fuel trucks drive the manoeuvring area as any other traffic on a controlled airport (#752). The vehicle roads and the aprons need no clearance. Driving along taxiways (a stretch of 30 m or more; nearby ones together), a vehicle holds 5 m short and calls ground: "Ruzyne Ground, Tug 3, request proceed via H1, H". It goes once told "Tug 3, proceed via H1, H" and has read it back. A road only crossing a taxiway needs no call; the vehicle gives way there. Before a runway it holds 40 m outside the edge and calls the tower ("request cross runway 24"). It waits in the tower's crossing queue as an aircraft does and crosses after "Tug 3, cross runway 24". Off the runway it reports "Tug 3, runway vacated". The library side is `traffic.VehicleATC` (`SetATC` on a tug or fuel truck); the World answers as ground and tower. Vehicles are called "Tug n" and "Fuel n".
