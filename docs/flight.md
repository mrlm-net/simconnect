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
- `OnSample` hears each sample as it comes, for a live view or a stream.
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
