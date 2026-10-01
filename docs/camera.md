---
title: "Camera"
description: "The MSFS 2024 add-on camera: bindings, poses, drone moves, a director that plays shots, and scripted scenes on the airport map."
order: 14
section: "traffic"
---

# Camera

MSFS 2024 lets an add-on take the camera: acquire it, place it relative to the world, an aircraft or the pilot's eyepoint, and give it back. The SDK calls are bound on the engine, `pkg/camera` turns them into shots, and the airport map films its traffic with them.

## Bindings

`engine.Engine` (MSFS 2024 only):

| Call | What it does |
|------|--------------|
| `CameraAcquire(clientID)` | Takes the camera; a `SIMCONNECT_RECV_CAMERA_STATUS` follows (`Message.AsCameraStatus`). |
| `CameraSet(data, mask)` | Places it: `types.SIMCONNECT_DATA_CAMERA` with the members in `mask` (position, rotation, target, field of view). |
| `CameraGet(referential)` | Asks where it is; `Message.AsCameraData` decodes the answer. |
| `CameraRelease(def)` | Gives it back. |
| `RequestCameraWorldLocker(pos, ref, id)`, `DeleteCameraWorldLocker()` | Keeps the terrain, scenery and objects around a point loaded while the camera is away from the user aircraft. |
| `CameraGetStatus`, `SubscribeToCameraStatusUpdate`, `CameraEnableFlag`, `EnumerateCameraDefinitions`, `CameraSetUsingCameraDefinition` | The rest of the API. |

SimConnect packs `SIMCONNECT_DATA_CAMERA` to one byte (84 bytes), so it crosses the API as bytes: `Bytes()` and `types.CameraDataFrom`.

### Conventions

These were measured live with SDK 1.7.3; the SDK documentation leaves them out.

- **World referential:** x is the latitude and y the longitude, both in degrees; z is the altitude in meters above sea level.
- **Relative to an aircraft:** x points to its **left**, y up, z forward, in meters.
- **Rotation:** what SimConnect returns does not follow the header's pitch, bank, heading order. `pkg/camera` aims with a point to look at (targeted) instead of angles.

`camera.On(object, right, up, forward)` takes an offset to the right and flips it for SimConnect.

## Poses and shots

```go
p := camera.Pose{
	Eye:    camera.On(obj, 40, 6, -60), // right, up, forward of the aircraft
	Target: camera.On(obj, 0, 2, 0),    // look at it
	FovDeg: 45,
}
```

A `Shot` is the camera over time: `Length()` and `PoseAt(t)`. No shot runs longer than `camera.MaxShot` (10 s).

- `Hold` keeps one pose.
- `Move` goes from one pose to another, eased (`Smooth`: no jerk at either end).
- `Path` flies through keyframes on a Catmull-Rom curve. This is where combined moves come from: rising while circling while pushing in.
- `Orbit`, `Chase`, `Track` (a camera fixed in the world that follows an aircraft) and `TrackZoom` (the same, zooming).

### Drone moves

Each move is a `MoveFunc(object, size, side, duration)`, scaled to the aircraft's span and length.

| Kind | Moves |
|------|-------|
| Wide | `RevealRise`, `Flyover`, `SpiralDescend`, `LeadChase`, `ParallaxTrack`, `HeroLowPushIn`, `PullBackReveal`, `TopDown`, `TopOrbit`, `SideDolly`, `HeadOnPass`, `ChaseRise` |
| Detail | `EngineDetail`, `NoseGearDetail`, `MainGearDetail`, `CockpitDetail`, `TailDetail`, `WingtipAlong`, `LightsDetail` |

## Director

```go
d := camera.NewDirector(engine, "my-addon")
d.LockWorld(camera.At(lat, lon, altM)) // optional: no scenery loading on cuts
d.Play(shotA, shotB)                    // a cut; Then queues
// in the connection's goroutine, 30–60 times a second:
d.Tick(time.Now())
// and for every message:
d.Handle(msg)
```

The director acquires the camera on the first `Tick`. It sets the current shot's pose on every tick and moves to the next shot when one ends; with nothing queued it holds the last pose. `Release` gives the camera back and deletes the world locker. Always release it: a camera left acquired stays stuck for the user.

## On the airport map

The camera button in the status strip opens the director:

- **🎥 Camera**
  - *Auto director* cuts to the aircraft on the radio as you hear the call. It plays a short sequence for what that aircraft is doing: wide, a detail, wide again; on the runway and on final it opens with a camera beside the runway.
  - *Follow selected* stays on the selected card's aircraft.
- **View** holds a fixed view of the selected aircraft, or of your own when none is selected: chase, cockpit, wing, front, top or tower. ◀ ▶ step through our aircraft, and selecting another aircraft moves the view to it.
  - *Tower* looks from the airport's tower and turns with the aircraft. The position is the facility data's `TOWER_LATITUDE` and `TOWER_LONGITUDE`, 45 m above `TOWER_ALTITUDE`, which is the ground at the tower (`towerHeightM`; without a tower, over the airport reference point). With no aircraft selected, the tower turns to whoever is on the radio, or otherwise the busiest aircraft, checking every 3 s.
- **🎬 Scene** plays a scripted film. A scene spawns its cast, listens to one of them on the radio, and plays **beats**: each waits for its cue, then cuts to its shots. If a beat's shots end before the next cue, more shots of the same aircraft fill the gap.

Scenes are JSON files in `-scenes` (default `scenes/`), read on every play, so a scene can be edited and played again without a restart.

```json
{
  "name": "DEMO007 departure",
  "cast": [{ "role": "dep", "kind": "departure", "tail": "DEMO007", "models": "csa-lines, a320", "tug": true, "procedure": true, "pushAfterSec": 35 }],
  "listen": "dep",
  "beats": [
    { "cue": "start", "shots": [{ "move": "crane", "who": "dep", "sec": 10 }] },
    { "cue": "dep:lights:B", "shots": [{ "move": "topOrbit", "who": "dep", "sec": 10 }] },
    { "cue": "dep:departing", "shots": [{ "move": "runwaySide", "who": "dep", "along": 900, "right": 90, "fov": 40, "fovTo": 22, "sec": 7 }] },
    { "cue": "dep:height:1500", "end": true, "shots": [{ "move": "chaseRise", "who": "dep", "sec": 10 }] }
  ]
}
```

### Cues

A cue holds once reached, so a beat is not missed when two things happen close together.

| Cue | When it holds |
|-----|---------------|
| `start` | At once. |
| `t:SECONDS` | That long after the scene began. |
| `ROLE:STATE` | The aircraft has reached that controller state, e.g. `dep:pushback`, `dep:taxiing`, `arr:rollout`. |
| `ROLE:airborne` | The departure has left the ground. |
| `ROLE:lights:X` | That light has come on: N nav, B beacon, S strobe, T taxi, L landing, O logo, W wing. |
| `ROLE:height:FEET` | The departure has climbed that high above the runway. |
| `ROLE:heard` | A call to or from the aircraft has been heard since the last beat. |

A beat's `listen` switches the radio to that aircraft's frequency from then on. `move` is one of the drone moves above (`revealRise`, `topOrbit`, `mainGear`, …), `crane` (the opening), or `runwaySide` / `underApproach` (fixed beside the runway, zooming from `fov` to `fovTo`).

```text
GET  /api/camera                     mode, subject, shot
POST /api/camera {mode, id}          off | follow (id: the card) | auto
GET  /api/camera/scenes              the scenes
POST /api/camera/scene?name=&icao=   play one
```
