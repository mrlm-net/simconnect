---
title: "Camera"
description: "The MSFS 2024 add-on camera: bindings, poses, drone moves, a director that plays shots, and scripted scenes on the airport map."
order: 23
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

## Long shots

`MaxShot` (10 s) caps every shot unless it opts out. `camera.Stretch(shot, d)` plays any shot over `d`, up to `MaxStretch` (5 minutes), its move slowed to fit, for a camera that stays with one aircraft through an event. In a scene or a `ShotRequest`, `"long": true` makes `sec` exact (no pacing, no cap).

## Moments

The instants worth a cut, on any aircraft the World sees: `flare`, `touchdown`, `reversers`, `vacated`, `rotate`, `liftoff`, `gearup`. `World.OnMoment(f)` hands each one over as it happens; `GET /api/camera/moments?since=SEQ` gives the last 256 after that number (`seq`, `name`, `tail`, `objectId`, `at`, `user`, `ours`). Scene cues use them as `ROLE:MOMENT`.

Ours say all of them, at the frame they happen. The player's and other traffic's are read off the scan, at its rate, and are only what the simulator shows: `touchdown` and `liftoff` (from `SIM ON GROUND`, 20 ft up before it counts as flown, so a bounce is not a lift-off), `gearup`, and `vacated` (50 m off the line it touched down on). They have no `flare`, `rotate` or `reversers`.

## Spotting director

`World.StartSpotting(SpottingRequest{ICAO: "LKPR"})` or `POST /api/camera/spotting`: the camera watches the airport by itself. It picks an aircraft landing, taking off or taxiing and the nearest fitting spot, stays with it until that is over, and moves on. Landings and take-offs come first and interrupt a taxi, never each other. `GET /api/camera` gives `subject`, `event` and `shot`; mode `off` ends it.

It reads the scan, so it sees ours, other traffic and the player alike. `Kinds` (`ours`, `other`, `player`), `Types` (ICAO designators) and `Events` limit what it may pick. It cuts to a closer spot at most every 10 s, and only one 30% closer. What an aircraft is doing is worked out from where it is and how it moves, not from our controllers: an unusual approach off the runway's line counts as nothing to watch until it lines up.

## Spots

A `camera.Spot` is a camera standing in the world: fixed, or on a straight rail (`RailEnd`, at most `RailSpeed`, 4 m/s by default) going abeam its subject. Its lens (`FovMin`, `FovMax`) is chosen to keep the aircraft at `Share` of the frame's width, the change smoothed. `Spot.Track(object, size, d, feed, clock)` plays it for up to `MaxStretch`.

The aim is a world point from a filter on the subject's samples, which may come only once a second: it runs on with the velocity found between them and takes corrections out slowly, so a long lens does not show a step and a subject flying straight is not lagged. `LockAim` aims at the object itself instead, the simulator keeping it dead centre.

`World.Spots(icao)` (`GET /api/camera/spots?icao=`) lists an airport's spots, generated from its data: `threshold:<end>`, `touchdown:<end>` (on a rail), `approach:<end>`, `holding:<end>` and `apron:<lowest stand>`. A low one keeps clear of the runways, taxiways and stands, and a spot with no clear place is left out. `World.PlaySpot` (`POST /api/camera/spot`) plays one on any aircraft: `id` (-1 the player) or `tail`, with `sec`, `share` and `lock`. In a scene, `"spot": "touchdown:24"` on a shot, or `"touchdown"` for the aircraft's own runway.

`spots.json` in `DataDir` changes them by airport, local winning per value; a name not generated adds a spot:

```json
{"LKPR": {"touchdown:24": {"up": 12}, "threshold:06": {"off": true},
          "mine": {"lat": 50.1, "lon": 14.26, "up": 4}}}
```

Heights are above the runway's elevation, or the airport's off a runway. SimConnect has no call for the ground at a point, and no building data, so where the ground falls or rises under a spot (an approach, most of all) the table sets `up` or `altM`.

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

A host can play a scene it holds: `World.PlayScene(icao, scene)` (`POST /api/camera/scene?icao=` with the scene in the body), validated first, and `World.StopScene()`. `World.OnSceneEvent` hears what it reaches, in order and off the camera's goroutine: `start`, `cue`, `beat`, `shot`, `end`. `GET /api/camera` has the same as `scene`: name, beat, beats, cue and elapsed seconds.

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
| `ROLE:below:FEET` | The arrival has come down that low above the ground. |
| `ROLE:MOMENT` | That [moment](#moments) has happened since the scene began. ROLE can also be `player` or the call sign of any aircraft outside the cast. |
| `ROLE:heard` | A call to or from the aircraft has been heard since the last beat. |
| `ROLE:rescue` | Rescue vehicles have been sent for it (it has priority: an emergency). |
| `ROLE:rescue:with` | It is down and one of its rescue vehicles is within 80 m. |

### Radio

Scene events of kind `radio` give each transmission: `speaker` (a crew's call sign or the station), `callsign` (who a station talks to), `frequency`, `text`, `atSec` and `durSec`. `atSec` is when it starts, later than the event when it waits for the frequency. `durSec` is the speaking time worked out from the words, not what a voice took. Events of kind `shot` and `beat` give the `subject` on screen.

`lines` is the scene's own radio, to a timetable: `{"atSec": 12, "who": "dep", "text": "..."}`. `who` is a cast role (its crew) or a station's name, given as the controller so that all its lines are one voice; a station needs `frequency` and can name `to`. A line is said word for word at its time and never moves. Traffic on the frequency is kept off it for 15 s before and until it ends, and lines overlapping on one frequency are refused. These times are the scene's clock, as `t:` cues.

`mute` keeps the rest of the radio from the host: traffic goes on talking and acting, and its transmissions still come as `radio` events with `muted`, but `OnTransmission` hears only the lines. A host's voice still chooses what it plays, for example by `SceneFrequency`.

### The airport, emergencies and actions

A scene can do more than film. These fields make one a whole demonstration, as the built-in `4-showcase` does:

- **`jetways`**: true switches the jetways on as the scene starts.
- **`schedule`**: `{"density": 2, "maxAircraft": 30}` switches the airport's scheduled traffic on around the cast if it is off.
- **Cast:**
  - `atSec` is when it appears, in seconds from the start of the scene. With `after` set to a cue, `atSec` counts from that cue instead. For example, `"after": "dep:pushback"` keeps an arrival in step with the departure however long its fuelling takes.
  - `pushAfterSec` is the earliest push. A fuel truck or tug still at work comes first.
  - `stand` puts a departure on that stand; if it is taken, the departure goes to any free one.
  - `fuel` has it refuelled before the push.
  - `emergency` gives an arrival an emergency of that kind (`engine-failure`, `medical`, `fuel`, `pressurisation`, `fire`, `hydraulic`). It is declared once the aircraft flies, with its MAYDAY or PAN PAN, priority in the sequence, and the airport's rescue vehicles.
- **Beat `actions`**: run as the beat starts, with `who` a role. They are the scenario actions (`World.Act`):
  - `{"do": "say", "kind": "expectDelay", "who": "dep", "args": {"minutes": "5"}}`;
  - `{"do": "say", "kind": "custom", "who": "kal", "args": {"text": "…"}}` is a line said as written. It comes from the controller unless `"from": "pilot"`. `"reply"` is the other side's answer as written (`"none"` for no answer); by default a controller's line gets "Roger, …" and a crew's line gets nothing. `"frequency": "132.89"` puts it on a frequency no station of ours has, such as Praha Control en route;
  - `{"do": "emergency", "kind": "medical", "who": "kal", "args": {"say": "none"}}` gives priority, squawk 7700 and the rescue without the stock radio calls, so the scene's own lines tell it;
  - `{"do": "runway", "runway": "06/24", "kind": "hold", "on": true}` holds the runway: no clearances on it, but it stays the runway in use. Without `kind`, the runway is closed, and the runway in use changes.
- **`delaySec`**: starts a beat that long after its cue.
- **`endActions`**: run however the scene ends, for example to release a held runway.

A beat's `listen` switches the radio to that aircraft's frequency from then on, or to a frequency given (`"listen": "132.89"`). `move` is one of the drone moves above (`revealRise`, `topOrbit`, `mainGear`, …), `crane` (the opening), or `runwaySide` / `underApproach` (fixed beside the runway, zooming from `fov` to `fovTo`).

### Compositions and glides

Shots are composed first: a shot is a slow glide between two frames picked from stills, not a move judged only in motion. A frame is a move held still at one moment and distance. Its name is `<move>_dist<back>_t<moment>`; for example `sideDolly_dist1.6_t50` is the middle of `sideDolly`, 1.6 times farther from what it looks at.

The picked frames come from a sweep at LKPR: every move at three distances and three moments, reviewed as stills. The close details (cockpit, lights, main gear, hero push-in, head-on) were all too tight and are not picked (`sceneCompositions`). Two rules apply to every glide:

- **Never from behind.** A frame with the camera behind the tail is left out.
- **Clear of buildings.** At a stand the camera never goes ahead of the nose (the terminal) or to the left (the jetway). Elsewhere it stays low only over the taxiways and aprons (within 35 m of the taxi graph); 30 m up, it is above the roofs.

A shot is one of:

- **`{"from": "revealRise_dist2.4_t50", "to": "revealRise_dist1.6_t50"}`**: a glide from one frame to the next, at most 1.2 m/s. A glide longer than a shot allows is not played.
- **`{"from": "sideDolly_dist1_t50", "to": "dolly"}`**: a dolly, the camera and its aim sliding together along the aircraft, about 0.5 m/s. It is used only from a side frame. When one end is hidden, the slide starts there and comes out, showing the aircraft, never covering it.
- **`{"from": "…"}`** alone holds the frame.

The scene filler and the auto camera both play the glides in turn, never the same one twice until all have played.

### Composing your own shots

`cmd/scene-cli` drives the map while you write a scene:

```text
scene-cli spawn -tail DEMO009 [-model …] [-stand N]   a departure parked, its push 2 h away
scene-cli car -near DEMO009 -right 30 -forward 5        a car to frame against (-at-camera, -lat/-lon); Ctrl+C removes it
scene-cli capture NAME [-tail DEMO009] [-dir DIR]       the simulator's camera as NAME.json and NAME.jpg
scene-cli shot -tail DEMO009 -from sideDolly_dist1_t50 -to dolly   one shot (-glide N, -move M -at 0.5 -back 1.6)
scene-cli scene NAME | off | glides | moves | vehicle-radio on|off
```

A capture reads the simulator's own camera (`CameraGet` answers without taking it). It saves the camera's position, heading, pitch and field of view, and its offset from the aircraft filmed: right, up and ahead, with the point aimed at. It also saves where the camera is on the airport: the nearest stand and taxiway, and its distance along and beside the runway. The picture is the simulator's window, which must be on screen; a covered window gives a stale picture.

```text
GET  /api/camera                     mode, subject, shot
POST /api/camera {mode, id}          off | follow (id: the card) | auto
GET  /api/camera/scenes              the scenes
POST /api/camera/scene?name=&icao=   play one
GET  /api/camera/moves               the moves a shot takes
GET  /api/camera/glides              the glides between picked frames
POST /api/camera/shot {icao, tail, move | from, to | glide, side, sec, back, fov, at}   one shot; the camera goes back after it
POST /api/camera/capture {icao, name, tail}   the simulator's camera, placed
```
