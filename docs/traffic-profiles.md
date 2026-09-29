---
title: "Aircraft Profiles and Telemetry"
description: "One profile per aircraft type for injected traffic: airframe, taxi, take-off, approach, rollout, flaps and stand stop; refined from SimVars and tuned from recorded movements."
order: 7
section: "traffic"
---

# Aircraft Profiles and Telemetry

Speeds, distances and attitudes differ per aircraft type. `pkg/traffic` bundles every per-type figure injected traffic uses into an `AircraftProfile`:

| Field | Used for |
|-------|----------|
| `Type`, `Category` | ICAO type designator (`A20N`, `B77W`) and jet / turboprop / piston |
| `WingspanM`, `LengthM`, `WheelbaseM`, `ICAOCode` | Airframe; the aerodrome code letter from the span (A < 15 m, B < 24, C < 36, D < 52, E < 65, F < 80) |
| `CGHeightM` | Reference point height on the wheels (spawn altitude) |
| `TakeoffDistanceM` | Published take-off distance at MTOW, a reference figure |
| `Motion` | Ground motion: wheelbase, span, tail, taxi speed, accelerations |
| `Takeoff` | Take-off roll, Vr, rotation, climb, tail-strike pitch |
| `Approach` | Glide path speeds, approach and flare pitch, flare height, touchdown rate |
| `Rollout` | Braking, exit speeds |
| `NoseOffsetM` | Reference point to nose: the stand stop |
| `Flaps` | Take-off, approach and landing settings, retract and full-flap heights |
| `PushbackKts`, `Lights` | Pushback speed; light differences |

## Resolving a profile

```go
p := traffic.ProfileFor("FSLTL B77W Emirates") // p.Type == "B77W", p.ICAOCode == 'E'
p = traffic.ProfileFor("ATCCOM.AC_MODEL A319.0.text") // ATC MODEL works too
```

`ProfileFor` matches the container title, the `ATC MODEL` or a type designator against the known types, the most specific first: A20N, A320, A321, A21N, A319, B738, B38M, B739, B39M, B737, B77W, B77L, B772, B789, B788, B78X, A332, A333, A359, A35K, A388, B744, B748, E175, E190, E195, CRJ9, AT76 and DH8D (`KnownTypes()`). The figures are published values (span, length, wheelbase, final approach speed, Vr, take-off distance) and typical ones for what the simulator does not expose (pitch attitudes, flare, taxi speeds).

Unknown titles get `DefaultAircraftProfile()` — the A320 family figures every default in the package stands for (`DefaultMotionProfile`, `DefaultTakeoffProfile`, `DefaultApproachProfile`, `DefaultRolloutProfile`, `DefaultNoseOffsetMeters`, the flap tunables). `GenericProfile(span, category)` picks a representative type by size and scales its airframe.

`WakeFor(type)` takes the same titles and designators and gives the type's wake categories for spacing in the air ([Airborne Separation](traffic-separation.md#wake-categories)).

## Requests

`TaxiRequest` and `ArrivalRequest` have an `Aircraft *AircraftProfile`. `nil` resolves it from `Model`. The profile fills `Profile`, `Takeoff`, `Approach`, `Rollout` and `NoseOffset` where those are zero, so figures set explicitly still win and existing callers keep working. The controllers take the flaps, pushback speed, logo light, spawn height and the turn-around loop size from it.

```go
ac := traffic.ProfileFor(model)
ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: "24", Parking: stand, Model: model,
    InjectApproach: true, Aircraft: &ac})
```

`TakeoffProfileFor(model)` and `MotionProfileFor(model)` remain as shortcuts for `ProfileFor(model).Takeoff` and `.Motion`.

## Refining from SimVars

`ProfileReader` reads what the simulator reports about a spawned aircraft, through the application's message loop like `nav.WeatherReader`:

```go
pr := traffic.NewProfileReader(client, 8400, 8401)
pr.Request(objectID)
for msg := range client.Stream() {
    if v, ok := pr.Handle(msg); ok {
        ac := traffic.Refine(traffic.ProfileFor(v.ATCModel), v)
        _ = ac
    }
}
```

It reads `WING SPAN`, `DESIGN SPEED VS0`, `DESIGN SPEED VS1`, `DESIGN TAKEOFF SPEED`, `DESIGN SPEED CLIMB`, `DESIGN SPEED VC`, `NUMBER OF ENGINES`, `ENGINE TYPE`, `MAX GROSS WEIGHT`, `TOTAL WEIGHT`, `FLAPS NUM HANDLE POSITIONS`, `STATIC CG TO GROUND`, `ATC MODEL`, `ATC TYPE` and `CATEGORY`, and writes nothing. For object 0 (the user aircraft), the answer carries the user's own object ID.

`Refine` then:

- gives a generic profile (`Type ""`) the reported span and engine type, the final approach at 1.3 VS0 + 5 kt, rotation at `DESIGN TAKEOFF SPEED`, the climb at `DESIGN SPEED CLIMB`, and snaps its flaps to the handle detents;
- scales the speeds of any profile by weight, √(total / reference weight) within −12 % and +8 %, and the take-off acceleration inversely;
- takes a reported `STATIC CG TO GROUND`.

Known types keep their table airframe, speeds and flaps. A live read in MSFS 2024 at LKPR shows why:

| SimVar | Fenix A319 (user) | FSLTL A320 (AI) | Real A320 |
|--------|-------------------|-----------------|-----------|
| `WING SPAN` | 33.8 m | 31.7 m | 35.8 m |
| `DESIGN SPEED VS0` / `VS1` | 119 / 148 kt | 120 / 165 kt | about 105 / 140 kt |
| `DESIGN TAKEOFF SPEED` | 150 kt | 135 kt | 130–145 kt |
| `MAX GROSS` / `TOTAL WEIGHT` | 75.5 t / 44.9 t | 68.0 t / 87.3 t | 78 t MTOW |
| `FLAPS NUM HANDLE POSITIONS` | 5 | 4 | 5 |
| `STATIC CG TO GROUND` | 3.18 m | 3.73 m | — |
| `ATC MODEL` | `ATCCOM.AC_MODEL A319.0.text` | `A320` | — |

AI models report every variable, but the airframe and weights are those of a simplified flight model: the FSLTL A320's total weight exceeds its maximum, so `Refine` ignores weights above `MAX GROSS WEIGHT`. `ATC MODEL`, engine count and type, and `STATIC CG TO GROUND` are reliable.

## Telemetry

A `Recorder` logs every controlled movement to JSON lines, one per arrival or departure when it ends (#308). Pass it the events the application already reads from a controller:

```go
rec := traffic.NewRecorder(logFile)
stop := ctl.Plan().Stop
for ev := range ctl.Events() {
    rec.Arrival(traffic.MovementInfo{Model: model, Stop: &stop}, ev)
}
for _, s := range rec.Summary() {
    fmt.Printf("%s: %d arrivals, touchdown %.0f m at %.0f fpm\n", s.Type, s.Arrivals, s.TouchdownM, s.TouchdownFpm)
}
```

Each `Movement` has the model and type, the phase it ended in and any error, and — where they apply — the touchdown distance and vertical speed, the speed leaving the runway, the stand stop error, the lift-off distance from the start of the roll, and the mean and maximum taxi speed. `Summary()` (or `Summarize` over lines read back) gives per-type counts and medians to compare with the profile and tune it.
