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
| `Accel` / `Decel` / `Jerk` | 0.35 m/s² / 0.5 m/s² / 0.2 m/s³ |

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

## Measured in MSFS 2024

Live runs at LKPR (FSLTL A320, 1.3 km with three turns and a stop):

| | AI waypoints | Injection |
|---|---|---|
| Taxi light | turned off by the AI within ~1 s | on in every sample (15 874 of 15 874) |
| Position error | — | mean 0.04 m, max 0.27 m |
| Height over ground | bounces of about 1 m reported | constant (0.00 ft spread) |
| Speed | capped at about 6–9 kt, abrupt | planned: eased acceleration, slowing into turns, exact stop |

Events such as `FREEZE_*_SET` and `*_LIGHTS_SET` reach AI objects only with `SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY` = `0x10`. Earlier SDK versions had the wrong value (#310).
