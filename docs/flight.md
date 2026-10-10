---
title: "Flight Recording"
description: "Record how an aircraft flies, the user's or AI, every sim frame with pkg/flight: a versioned Track of position, attitude, speeds, gear, flaps, spoilers, control surfaces, lights, engines and autopilot, written as JSON lines and interpolated at any time."
order: 14
section: "packages"
---

# Flight Recording

`pkg/flight` records how an aircraft flies — the user aircraft or any AI object — every sim frame, as a `Track`. A Track holds what the aircraft looks and moves like, so a replay of it or a profile learned from it looks like the real flight.

## Recording

```go
rec := flight.NewRecorder(client, 0) // IDs from flight.DefaultRecorderBase
rec.Start(types.SIMCONNECT_OBJECT_ID_USER, flight.RecordOptions{Title: title, Model: "A320", Note: "LKPR 24"})
// in the message loop:
if rec.Handle(msg) {
	continue
}
// later:
track := rec.Stop(types.SIMCONNECT_OBJECT_ID_USER)
track.WriteFile("LKPR-LPPT.jsonl.gz")
```

- One data definition, one request per object, `SIMCONNECT_PERIOD_SIM_FRAME` (`EveryFrames` thins it). Up to 63 objects at once.
- `OnSample` hears each sample as it comes, for a live view or a stream; `Listen(f)` adds more listeners (the function it returns removes one).
- `Watch(objectID, everyFrames)` streams an object's samples to the listeners without keeping a Track (a pilot's every frame); `Unwatch` ends it. Watched and recorded at once, one request serves both at the finer interval, and the Track keeps only its own share of the frames.
- `Snapshot` copies a Track while it records; `Reset` asks again on a new connection.
- The recorder never reads the connection itself: feed it every message.

## A sample

| Field | From |
|---|---|
| `T` | `SIMULATION TIME` (runs with the sim rate, stops while paused) |
| `Lat`, `Lon`, `AltFt`, `GroundFt`, `CGFt` | position, ground under it, static CG height |
| `Pitch`, `Bank`, `Heading` | nose up and right wing down positive (SimConnect's own are the other way); heading true |
| `IAS`, `GS`, `VS`, `OnGround` | knots, feet per minute |
| `GearHandle`, `GearPct`, `FlapsIndex`, `FlapsPct`, `Spoilers`, `SpoilersArmed` | levers and surfaces |
| `Elevator`, `Aileron`, `Rudder`, `Brakes`, `ParkingBrake` | −100…100, brakes 0…100 |
| `Lights` | bits: nav, beacon, landing, taxi, strobe, logo, wing (`LightNav`…) |
| `EngineCount`, `Throttle[4]`, `N1[4]`, `Reverser[4]` | per engine |
| `AP` | master, FD, A/THR armed, the holds engaged (HDG, ALT, VS, SPD, NAV, APPR, G/S) and the selected HDG, ALT, VS, SPD |

The standard SimVars give these. An add-on that keeps its own state in L:vars (the Fenix's autopilot, say) shows the sim's values; its profile in `pkg/systems` reads the real ones.

## The file

JSON lines: a header (`version`, title, model, user, started, note, and `fields`, the sample's field names in order), then one sample per line as an array of numbers. A reader takes the fields by name: one it does not know is skipped and one missing stays zero, so fields can be added without breaking older files. A path ending in `.gz` is gzipped (an hour at 60 frames a second is about 216,000 lines).

```go
t, err := flight.ReadTrackFile("LKPR-LPPT.jsonl.gz")
s, ok := t.At(t.Samples[0].T + 42.5) // the aircraft 42.5 s in
```

`At` interpolates between the samples around a time: numbers linearly, headings and longitudes the short way round, and what is on or off (gear handle, lights, flap detent, autopilot modes) as the earlier sample has it. `Lerp` does the same for two samples.

## Replay

`Player` is a Track's playback clock: `Play`, `Pause`, `Seek(t)`, `SetRate(r)`, `Time` and `Sample(now)`, with times in seconds from the first sample. It applies nothing itself. Each frame, give the sample to an applier.

```go
p := flight.NewPlayer(track)
p.Play(time.Now())
// every sim frame (SIM_FRAME event or the frame's data):
s, done := p.Sample(time.Now())
replay.Apply(s)
```

**On the user aircraft** (`UserReplay`): `Start` freezes it (`FREEZE_LATITUDE_LONGITUDE_SET`, `FREEZE_ALTITUDE_SET`, `FREEZE_ATTITUDE_SET`). `Apply` writes its position and attitude every frame (`PLANE LATITUDE` … `PLANE HEADING DEGREES TRUE`) and sends the rest only as it changes: the gear handle (`GEAR_SET`), the flap lever (`FLAPS_SET`), the spoilers and their arming, the lights one by one (`*_LIGHTS_SET`), the throttles per engine and the control surfaces (axis events). `Stop` frees it where the replay left it.

**As a ghost** (`Ghost`): an AI object you created (`AICreateNonATCAircraft`) and the traffic `Injector` took over (`Takeover`). `Apply` places it as flown (`Injector.PlaceFlown`), with its own CG height on the ground, so another model rolls on its own wheels. Its gear, flaps, spoilers, lights, engines (running from N1 15 %) and throttle follow.

A frame's sample is interpolated (`At`), so the replay is as smooth as the sim's frame rate. Not measured live yet: the elevator axis sign, and the control surfaces moving on a frozen user aircraft.

## Learning how a type is flown

`Learn(model, tracks...)` reads the player's recorded flights of a type and gives a `Learned` profile, each value the median of the flights that show it:

- the take-off: rotation speed (the nose 1° above the roll's pitch), lift-off speed and pitch, the initial climb pitch (lift-off to 1000 ft), the gear-up height;
- the climb-out: the height of the first flap retraction, and the speed each detent was left at (`FlapsUpKts`);
- the approach: the speed and height each detent was reached at (`FlapsDownKts`, `FlapsDownAGLFt`), the gear-down height, the final approach speed (1000 to 200 ft);
- the landing: the flare height (where the nose started up over the final's pitch) and the sink rate at touchdown.

`pilot.Config.WithLearned` takes them into the pilot flying's profile where the config leaves a value to its default.

**A ghost on its own** (`GhostReplay`): `NewGhostReplay(client, injector, track, title, tail, reqID)` replays a Track as an AI aircraft without wiring the creation yourself. `Start` creates the object where its `Player` stands. `Handle(msg)` takes the object ID the simulator assigns, has the Injector take it over, and then flies it as the Player says, at most every frame. `Stop` removes it. Use the host's own Injector, with IDs clear of any other's.

## Judging a flight

`Assess(track, AssessOptions)` judges a recorded flight against common airline practice and returns an `Assessment`. It holds the flight's key figures (`Profile`), the findings in time order, each with what happened and what to do, and a `Score` to rank flights by: 100, less 5 for each minor finding and 15 for each major one.

The profile has the fastest taxi speed, the rotation and lift-off speeds, the lift-off pitch and the fastest pitch rate, the highest altitude and steepest bank, the speed and sink rate at 1000 ft and 500 ft above the ground, the approach speed, and the touchdown (sink rate, speed, pitch, bank, bounces). With `AssessOptions.Runway` (the threshold and true heading) it also has where the aircraft touched down past the threshold and how far off the centreline.

| Phase | Finding | Severity |
|---|---|---|
| taxi | faster than 30 kt; engines running without the beacon | minor |
| take-off | no landing lights; rotation faster than 4°/s | minor |
| take-off | lift-off pitch past the tail limit (`TailstrikePitch`, 11°) | major |
| airborne | bank over 30° (major over 35°); over 260 kt below 10,000 ft | minor |
| airborne | bank over 10° below 100 ft | major |
| approach | gear still up at 1000 ft | minor |
| approach | not stable at 500 ft and landed (gear up, not landing flaps, speed outside Vapp −5/+10, sinking over 1000 fpm) | major |
| landing | over 600 fpm (hard), nose gear first, pitch past the tail limit, touched down short (under 150 m) | major |
| landing | over 360 fpm (firm), banked over 3°, bounced, long (past 900 m), more than 5 m off the centreline | minor |
| landing | under 60 fpm (floated: soft, but it eats runway) | info |

Vapp is `AssessOptions.ApproachKts`, else the median speed between 1000 ft and 200 ft on the final. The phases come from the track alone: the take-off roll starts at 40 kt, and the landing is the last touchdown.

```go
a := flight.Assess(track, flight.AssessOptions{Runway: &flight.AssessRunway{Lat: thr.Lat, Lon: thr.Lon, Heading: 243}})
fmt.Println(a.Score, a.Profile.TouchdownFpm)
for _, f := range a.Findings {
    fmt.Println(f.Phase, f.Severity, f.Text)
}
```

## Scenes: the traffic around a flight

A `Scene` is the aircraft around a flight, recorded alongside it so a replay can show the scene as it was (#1011). Each `SceneAircraft` has a `Key` (stable within the scene), its identity (`Callsign`, `Title`, `Livery`, `Type`, and `Source`: `ours`, `sim-ai` or `player`), and its own `Track`. Aircraft come and go, so each has its own `First()` and `Last()` time. `Scene.At(t)` gives the aircraft there at simulation time `t`, interpolated. `Span()` gives the scene's first and last time, and `Find(key)` finds one aircraft. `Write` and `ReadScene` (or `WriteFile` and `ReadSceneFile`, gzipped for `.gz`) use versioned JSON lines in one file. The file has a header, then each aircraft's identity line followed by its sample rows in the Track's field order.

`SceneRecorder` records a scene on the player's `Recorder` (#1012), so the scene runs on the player's Track clock (SIMULATION TIME). About once a second, give `Update` the aircraft near the player, nearest first, each a `SceneObject` (object ID and identity). Each new aircraft is recorded every `EveryFrames` frames (default 60, about 1 Hz), up to `Max` at once (default 40, never past the Recorder's 63). An aircraft that has gone is stopped and its stretch kept. One that comes back later gets a fresh key (`CSA1#2`). `Snapshot()` gives the scene so far, and `Stop()` gives the finished scene.

```go
scene := flight.NewSceneRecorder(rec, flight.SceneOptions{Note: "LKPR"})
// every second:
scene.Update(nearPlayer) // []flight.SceneObject, nearest first
// at the end:
scene.Stop().WriteFile(id + ".traffic.jsonl.gz")
```

`GhostFleet` replays a scene in sync with a `Player`, the player replay's or the ghost's clock (#1013). It maps the Player's time onto the scene's: the track's first sample plus `Player.Time`. Pause, seek and rate therefore follow the Player with nothing else to do.

- **Spawning.** Each aircraft is created as a NonATC AI aircraft (its title, its callsign as the tail) when its time in the scene comes, taken over by the Injector and flown as its Track was (`Ghost`). If no object comes within 5 s (its title was refused), it is created once more as `FleetOptions.Fallback`'s model.
- **Removal.** An aircraft is removed when its track ends, when it falls out of `Budget` (default 20, nearest the player first, or nearest `Centre`), or on `Stop`.
- **Seek.** A seek is just another moment: the aircraft due then are created and the rest removed. An object that arrives after its aircraft stopped being due is removed at once.
- **IDs.** Request IDs start at `FleetOptions.IDBase` (`DefaultFleetBase` 0x7E00, 256 of them).
- **Running it.** Feed it every message (`Handle`); it brings the fleet to the present at most every frame. Call `Tick` after a seek for an immediate change.

```go
fleet := flight.NewGhostFleet(client, injector, scene, replay.Player(), flight.FleetOptions{Budget: 25})
// in the message loop:
fleet.Handle(msg)
// at the end:
fleet.Stop()
```
