---
title: "Failures"
description: "Fail and restore the user aircraft's systems by generic name with pkg/failures: engines, electrical, hydraulic, pitot and static, vacuum, brakes; a default profile on the simulator's failure events and per-model overrides."
order: 22
section: "packages"
---

# Failures

`pkg/failures` fails and restores systems of the user aircraft by generic names, the same way for every aircraft. It drives the simulator's own failure key events (MSFS 2024 "Aircraft Miscellaneous Events", each a toggle with no parameter): `TOGGLE_ENGINE1_FAILURE` to `TOGGLE_ENGINE4_FAILURE`, `TOGGLE_ELECTRICAL_FAILURE`, `TOGGLE_HYDRAULIC_FAILURE`, `TOGGLE_VACUUM_FAILURE`, `TOGGLE_PITOT_BLOCKAGE`, `TOGGLE_STATIC_PORT_BLOCKAGE` and the left, right and total brake failures.

```go
f := failures.New(client, failures.Default(), 0) // events from failures.DefaultEventBase

fmt.Println(f.Supported()) // the kinds this aircraft can fail

err := f.Apply(failures.Failure{Kind: failures.EngineFailure, Engine: 2})
if errors.Is(err, failures.ErrUnsupported) {
    // the aircraft's profile has no way to fail it
}
f.Apply(failures.Failure{Kind: failures.Pitot})

for _, s := range f.Active() { // oldest first
    fmt.Println(s.Kind, s.Engine, s.Since)
}

f.Clear(failures.Failure{Kind: failures.Pitot})
f.ClearAll()
```

## Kinds

| Kind | Name | Default event |
|---|---|---|
| `EngineFailure` | `engine-failure` | `TOGGLE_ENGINE<n>_FAILURE`, engine 1–4 in `Failure.Engine` |
| `Electrical` | `electrical` | `TOGGLE_ELECTRICAL_FAILURE` |
| `Hydraulic` | `hydraulic` | `TOGGLE_HYDRAULIC_FAILURE` |
| `Vacuum` | `vacuum` | `TOGGLE_VACUUM_FAILURE` |
| `Pitot` | `pitot` | `TOGGLE_PITOT_BLOCKAGE` |
| `Static` | `static` | `TOGGLE_STATIC_PORT_BLOCKAGE` |
| `Brakes` | `brakes` | `TOGGLE_TOTAL_BRAKE_FAILURE` |
| `BrakeLeft`, `BrakeRight` | `brake-left`, `brake-right` | `TOGGLE_LEFT_BRAKE_FAILURE`, `TOGGLE_RIGHT_BRAKE_FAILURE` |
| `EngineFire` | `engine-fire` | none: unsupported unless a model's override maps it |
| `FuelLeak` | `fuel-leak` | none |
| `Pressurisation` | `pressurisation` | none |

The simulator has no failure event for an engine fire, a fuel leak or a pressurisation failure, so these are unsupported by default.

## Profiles: default plus override

A `Profile` maps kinds to the events that toggle them: `Events` for one event per kind, `EngineEvents` for one per engine (index 0 is engine 1). `Default()` is the simulator's own failures. An aircraft with its own systems gets an override on top with `With`; an empty event name (or no engine events) takes a kind away, so a study-level aircraft that ignores the simulator's failures can map none, and `Supported` then says so. `LoadProfile` reads an override from a JSON file, for a local override beside the application's settings:

```json
{"events": {"electrical": "", "pitot": "MY_AIRCRAFT_PITOT_FAIL"},
 "engineEvents": {"engine-fire": ["MY_ENG1_FIRE", "MY_ENG2_FIRE"]}}
```

```go
over, err := failures.LoadProfile(path)
prof := failures.Default().With(over)
f := failures.New(client, prof, 0)
```

The application never asks which aircraft it is: it builds the profile for the loaded model and calls the same methods.

## Behaviour

- `Apply` does nothing when the failure is already active; `Clear` does nothing when it is not. Both send the toggle event, so only change a failure through `Failures`, or its idea of what is active drifts from the simulator's.
- Events are mapped to client event IDs on first use, from the base given to `New` (0: `DefaultEventBase`). See [Request and ID Management](manager-requests-ids.md#library-default-id-ranges) for the range.
- After a reconnect the new connection knows neither the mapping nor the failures: call `Reset`, then `Reapply` to toggle every active failure on again.
- `Failures` is safe for concurrent use.

The traffic engine's emergencies are about AI traffic; this package is for the user aircraft. See [Emergencies & Scenarios](traffic-emergencies.md) for the World side.
