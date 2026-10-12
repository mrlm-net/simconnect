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
| `CameraGetStatus`, `SubscribeToCameraStatusUpdate`, `UnsubscribeToCameraStatusUpdate`, `CameraEnableFlag`, `CameraDisableFlag`, `EnumerateCameraDefinitions`, `CameraSetUsingCameraDefinition`, `SubscribeToCameraWorldLockerStatusUpdate` | The rest of the API. |

SimConnect packs `SIMCONNECT_DATA_CAMERA` to one byte (84 bytes), so it crosses the API as bytes: `Bytes()` and `types.CameraDataFrom`.

### Conventions

These were measured live with SDK 1.7.3; the SDK documentation leaves them out.

- **World referential:** x is the latitude and y the longitude, both in degrees; z is the altitude in meters above sea level.
- **Relative to an aircraft:** x points to its **left**, y up, z forward, in meters.
- **Rotation:** what SimConnect returns does not follow the header's pitch, bank, heading order. `pkg/camera` aims with a point to look at (targeted) instead of angles.

`camera.On(object, right, up, forward)` takes an offset to the right and flips it for SimConnect (object 0 is the user aircraft). `camera.At(lat, lon, altM)` is a point in the world; a `Point` with `Frame: camera.Eyepoint` is an offset from the pilot's eyepoint.

## Poses and shots

```go
p := camera.Pose{
	Eye:    camera.On(obj, 40, 6, -60), // right, up, forward of the aircraft
	Target: camera.On(obj, 0, 2, 0),    // look at it
	FovDeg: 45,
}
```

A pose without `FovDeg` uses `camera.DefaultFovDeg` (55°). A `Shot` is the camera over time: `Length()`, `PoseAt(t)` and `Name()`. No shot runs longer than `camera.MaxShot` (10 s).

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

The director acquires the camera on the first `Tick` with a shot to play, and asks again every `AcquireRetry` (5 s) while it is not given (`ErrNotAcquired` meanwhile); a camera held by someone else or disabled by the user is not asked for again until `Release`. It sets the current shot's pose on every tick and moves to the next shot when one ends; with nothing queued it holds the last pose. After each cut the new shot holds its first pose for `CutSettle` (600 ms) while the picture settles. `OnShot` hears each shot as it starts; `Current` and `Remaining` tell what plays. After a reconnect call `Reset(api)` (nil keeps the client). `Release` gives the camera back and deletes the world locker. Always release it: a camera left acquired stays stuck for the user.

## Shots on a replay

For a shot designer over a recorded flight:

- **Moves listed:** `Moves()` gives the drone moves as `MoveInfo{Name, Doc, Kind, Func}`: name, what it shows, `Wide` or `Detail`, and the function making the shot (`Func(object, size, side, duration)`). `TrackZoom` is not among them, since it takes an eye point.
- **Shots in the world:** `Placed(point, poseOf)` places a shot's point in the world.
  - A world point stays as it is.
  - An object point is placed from that object's `ObjectPose` (lat, lon, altitude, heading, pitch, bank), rotated by its attitude.
  - An eyepoint point cannot be placed.

  `WorldPoses(shot, step, poseAt)` samples a whole shot that way, the object's pose taken at each time from the recording. Each sample has the eye, the target and the field of view, which is the camera path and view cone to draw on a map, matching what the simulator will show.
- **On the replay's clock:** a `Timeline` of `Cue{At, Shot}` places shots on a film. `Director.Follow(timeline, clock)` plays it on a clock the host supplies, such as the replay's time.
  - **Seeking and pausing:** a shot cued at 545 s plays then, after a seek or a pause, at any rate.
  - **Between shots:** the camera holds its last pose.
  - **At the end:** past the film's end, `Tick` returns `ErrFilmEnded`, and the host calls `Release`, which also stops following.
  - **Clock function:** it is called under the Director's lock, so it must not call the Director.
- **Designed shots:** a `WorldPath` is a shot keyed in the world on the replay's clock.
  - **Keys:** each `WorldKey` has a replay time `T` (from the recording's first sample), the eye (`Lat`, `Lon`, `AltM` above sea level), a heading and pitch, and a field of view. The eye moves on a Catmull-Rom curve through the keys, eased over the whole shot (`Ease`, nil: `Smooth`).
  - **Look at:** `LookAircraft` follows the replayed aircraft, either its live object (`Object`, placed by the simulator in step with what it shows) or, without one, its recorded track (`Track(t)`). `LookPoint` keeps a fixed point in view, and `LookFree` aims by each key's heading and pitch.
  - **Checks:** `Validate` checks the structure (two keys or more, increasing times, a complete look mode). `Long` returns `ErrShotTooLong` past `MaxShot` (10 s) as a warning only: the shot still plays at its full length.
  - **Playing:** `p.Cue()` places it at its first key's time; `NewTimeline(cues...)` and `Director.Follow(timeline, replayTime)` play it, released at `ErrFilmEnded`.
  - **Presets:** `KeysOf(shot, at, step, poseAt)` turns any shot, such as `SideDolly`, into world keys at replay time `at`, each aimed where the shot looks, so a designer starts from a preset and edits it (played back with `LookFree`).
  - **Lens:** `FovFromFocalMM` and `FocalMMFromFov` convert a full-frame focal length to the horizontal field of view and back (50 mm is 39.6°).

## On the airport map

The camera button in the status strip opens the director:

- **🎥 Camera**
  - *Auto director* cuts to the aircraft on the radio as you hear the call. It plays a short sequence for what that aircraft is doing: wide, a detail, wide again; on the runway and on final it opens with a camera beside the runway.
  - *Follow selected* stays on the selected card's aircraft.
  - *Tower* looks from the airport's tower. With an aircraft selected it turns with that aircraft. With none, it stands at the tower aimed at the middle of the airfield, and the *Tower look* buttons turn it: ⟲ ⟳ left and right, ▲ ▼ up and down, + − zoom; hold one to keep turning. The aim eases after each turn (`towerEase`, 0.35 s). *Swing* makes it look round by itself, one slow swing side to side (110°, 70 s there and back). `POST /api/camera {"mode":"look","yaw":4,"tilt":0,"fov":0}` turns it; `{"mode":"look","swing":true}` swings. Keys turn it too (arrows, + and −, Shift faster), and so does the middle mouse button held and dragged (with the wheel while held for zoom); a double click of the middle button locks the look to the mouse with no button held, until another middle click or Esc. The *Angle* switch (off by default) draws on the map a cone of where it looks, labelled with its bearing, tilt and field of view; `GET /api/camera` gives them as `look`. The tower is where you placed it (Airport tab: *Tower*, saved per airport in the user cache folder, `towers.json`). Otherwise it is the airport's known tower (`airport.Limits.Tower`: LKPR's facility position is not its tower), then the facility's `TOWER_LATITUDE`/`TOWER_LONGITUDE` (`TOWER_ALTITUDE` is the ground there, not the cab), then over the airport reference point. The cab is 100 m above the airfield unless you set another height (10–250 m). `GET`/`POST /api/tower`.
  - When our camera is released, the simulator's camera is set back to what it was before (live, it otherwise drops into chase).
- **Simulator camera** gives the camera back to the simulator and sets its own: `CAMERA STATE` cockpit (2), chase (3), drone (4), fixed on plane (5), free (6, environment), written as INT32 like `examples/set-variables`. Measured live in MSFS 2024: the numbering on the 2024 SDK page (drone 8, showcase 7, follow traffic 23) was refused. ◀ ▶ step through the camera's views (`CAMERA VIEW TYPE AND INDEX:1`) from the one it is on. API: `POST /api/camera {"mode":"sim","sim":"drone"}` or `{"mode":"sim","step":1}`.
- **🎬 Scene** plays a scripted film. A scene spawns its cast, listens to one of them on the radio, and plays **beats**: each waits for its cue, then cuts to its shots. If a beat's shots end before the next cue, more shots of the same aircraft fill the gap.

Scenes are JSON files in `-scenes` (default `scenes/`), read on every play, so a scene can be edited and played again without a restart. The built-in scenes come after them; a file with a built-in scene's name wins.

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
