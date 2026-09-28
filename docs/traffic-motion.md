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

`Place` puts the aircraft on the ground (ground altitude + `STATIC CG TO GROUND`, requested every sim frame). `SetLights` sends only the lights that change. Presets: `LightsParked`, `LightsPushback`, `LightsTaxi`, `LightsRunway`. `Injector` uses 2 definition IDs, 2 request IDs per aircraft and 10 event IDs; move them with `InjectorWithIDs`.

## Hybrid arrival

`ArrivalController` combines both approaches with `ArrivalWithInjector(inj)`. Feed every message to both the controller and the injector.

```go
inj := traffic.NewInjector(client)
ctl := traffic.NewArrivalController(fleet, traffic.ArrivalWithInjector(inj))
ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: model})
// message loop: inj.Handle(msg); ctl.Handle(msg)
```

1. **MSFS AI flies** the approach, touchdown and the first part of the rollout. From touchdown the aircraft is read every sim frame and the injector watches the ground height under it.
2. **Takeover on the runway:** once the aircraft has been on the ground for `TakeoverAfterTouchdown` (2 s) and slowed to `TakeoverKts` (70 kt), at least `TakeoverBeforeExitMeters` before the exit. The mover starts at the aircraft's nose gear with its heading and speed, so nothing jumps at the switch. If the aircraft reaches the exit first, the takeover happens once it is clear of the runway.
3. **Rollout and exit:** braking at `RolloutDecel` to `InjectExitHighSpeedKts` (30 kt) through a high-speed exit (`InjectExitKts`, 12 kt, otherwise), then to taxi speed clear of the runway.
4. **Vacate stop:** the aircraft stops there and waits for `ClearToTaxi` (`HoldForClearance`) or the after-landing dwell, which varies by ±10 %. With `RollThroughChance` (default 30 %, only without `HoldForClearance`) it only slows to 0.5 kt and taxis on, like a rolling clearance.
5. **Runway crossings:** with `HoldAtCrossings` the aircraft stops with its nose gear `HoldShortStopMeters` before the hold-short line of every runway it crosses, reports `ArrivalHoldingShort` (with `ArrivalEvent.HoldingShortOf`), and waits for `ClearToCross()`. Runway lights stay off while it holds. A clearance given earlier means it does not stop. Without `HoldAtCrossings`, crossings count as cleared in advance. Departure gates (pushback, taxi, line-up, take-off) are #320.
6. **Taxi-in and parking:** the path ends straight along the stand axis, the last 30 m at 5 kt, with the reference point on the stop mark. The aircraft stays frozen on the stand; `Release` hands it back to MSFS AI.

Lights, all set by the controller once it has taken over:

| Phase | Lights |
|---|---|
| Rollout on the runway | nav, beacon, strobes, landing |
| Clear of the runway | strobes off |
| Vacate stop (or slowest point when rolling through) | landing off, taxi on `TaxiLightDelay` (1.5 s) later |
| Crossing a runway | strobes and landing on from just past the hold-short line before it until a moment after the tail has passed the opposite one |
| Parked | nav only (beacon and taxi off) |

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
- Flaps are at `ApproachFlapsPct` (flaps 3) on final and run to full over 5 s when passing `FlapsFullFt` (1000 ft), the stabilised-approach gate.
- Ground spoilers come out over `SpoilerDeploySeconds` at main-gear touchdown (`Injector.SetSpoilers`).
- With the nose wheel down, the injected rollout takes over from exactly that pose.
- Once clear of the runway the spoilers stow and the flaps retract over `FlapsRetractSeconds`.
- Thrust reversers cannot be shown on an AI aircraft. The reverser nozzle SimVar is not settable, and the reverse-thrust events are ignored.

`Injector.PlaceAir` places an `ApproachPose`: the main wheels `HeightFt` above the ground, pitched nose up `PitchDeg`, and on the ground from touchdown. MSFS AI objects ignore the flaps handle and `FLAPS_*` events, so `SetFlaps` writes the flap surface positions directly. Ramp the percentage for a visible movement. Gear animates on a frozen aircraft.

Measured live at LKPR runway 24: touchdown 486 m past the threshold at −120 fpm. Pitch readback equals the command.

## Measured in MSFS 2024

Live runs at LKPR (FSLTL A320, 1.3 km with three turns and a stop):

| | AI waypoints | Injection |
|---|---|---|
| Taxi light | turned off by the AI within ~1 s | on in every sample (15 874 of 15 874) |
| Position error | — | mean 0.04 m, max 0.27 m |
| Height over ground | bounces of about 1 m reported | constant (0.00 ft spread) |
| Speed | capped at about 6–9 kt, abrupt | planned: eased acceleration, slowing into turns, exact stop |

Events such as `FREEZE_*_SET` and `*_LIGHTS_SET` reach AI objects only with `SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY` = `0x10`. Earlier SDK versions had the wrong value (#310).
