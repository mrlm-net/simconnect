---
title: "Jetway Helpers"
description: "pkg/jetway: dock the simulator's jetways to NonATC AI traffic with invisible ParkedATC helper aircraft: the helper package, the Service interface and its Helpers backend, door offsets."
order: 23
section: "packages"
---

# Jetway Helpers

MSFS 2024 docks a jetway only to an aircraft in its ATC parking table. Injected traffic is NonATC, and no SimConnect call, event or variable docks a jetway to it. What works is an invisible **ParkedATC helper aircraft** with the door of the right type, created on the gate under the parked aircraft: the simulator docks the gate's jetway to the helper, and so to the door of the aircraft standing there. `pkg/jetway` writes the helper models and runs them.

This page is the library level. How the traffic World uses it (placeholders at start-up, docking on arrival, undocking before the push, retries and gate rest) is on [Jetways at Gates](traffic-jetways.md).

## The helper package

`WritePackage` writes a Community package with one invisible helper SimObject per type (a model with no LODs, parked at gates only), copying the base model's `flight_model.cfg`, `engines.cfg`, `systems.cfg` and `ai.cfg` so the helper's door (`interactive_point.0`) is that type's. The simulator reads it at its next start.

```go
dir, err := jetway.WritePackage(community, []jetway.HelperModel{
    {Type: "A320", Dir: `...\SimObjects\Airplanes\A320`},
}, jetway.PackageOptions{SideSteps: 6, SideStepM: 0.1, AheadM: []float64{-9}})
```

| `PackageOptions` | |
|---|---|
| `Folder`, `Version`, `Creator` | the package's folder (`mycrew-jetway-helper`), version (`0.1.0`) and manifest creator |
| `Codes` | airline codes for each type's second helper (`… ALL`); none: only the helper without codes |
| `WingSpanFt` | replaces every helper's wing span (the simulator puts no ParkedATC next to an occupied gate) |
| `SideSteps`, `SideStepM` | door variants `SideStepM × k` to the right, `k` from `-SideSteps` to `SideSteps`, for gates where the simulator parks off the line (`HelperVariantTitle`) |
| `AheadM` | door variants moved along the fuselage, for stands where our aircraft stops off the simulator's place, such as the stand rules' offsets (`HelperAheadTitle`) |

Every helper's title starts with `HelperTitlePrefix` (`"MyCrew Jetway Helper "`); `IsHelper` and `IsHelperVariant` recognise them, so lists can leave them out and traffic never uses them as a model. The [jetway-helpers](https://github.com/mrlm-net/simconnect/tree/main/examples/jetway-helpers) example writes the package from the installed traffic models (`-types`, `-codes`, `-span`, `-side-steps`, `-side-step`, `-ahead`).

## Service and the Helpers backend

The engine talks to a `Service`; with none, or one not `Available`, traffic parks and pushes as without jetways. `Helpers` is today's backend; a direct API, once the simulator has one, would be another behind the same calls.

```go
svc := jetway.NewHelpers(client, jetway.HelperOptions{
    Types:       []string{"A320", "B738"}, // types in the installed package
    RetractPlan: retractPlanFor,           // a .pln out of icao for the placeholders
    RequestBase: 43200,                    // 1000 request IDs from here
    AircraftAt:  ourAircraftOn,            // our parked aircraft on a stand, 0 none
})
svc.Prepare("LKPR")            // fill the jetway gates with placeholders
svc.Dock("LKPR", 14, "A320")   // our A320 parked on stand index 14
svc.Undock("LKPR", 14)         // before the push
st := svc.Status("LKPR", 14)   // reserved, docking, docked, undocking, failed
// in the message loop and every frame or so:
svc.Handle(msg)
svc.Update(time.Now())
```

| Method | |
|---|---|
| `Available()` | whether the backend can work here, and why not |
| `Prepare(icao)` / `Release(icao)` | ask for the airport's jetways and fill its gates with placeholders, one at a time; release removes every placeholder and helper and sends the jetways home |
| `Dock(icao, stand, model)` / `DockAs(…, title)` | take the stand's placeholder away and create the helper of the model's type (or a given variant): the simulator gives it the one free gate and its jetway docks |
| `Undock(icao, stand)` | send the jetway home; a docked helper stays as the gate's placeholder (given the retract plan, released `RetractWait` later) so the simulator cannot hand the gate to anything else |
| `Status`, `Since` | a stand's jetway now, and when that last changed |
| `Dockable(icao)`, `Filling(icao)`, `HasJetway` | gates a helper can be given now (about every other jetway gate: no ParkedATC next to an occupied one), whether the gates are still being filled |
| `HelperFor(model)`, `Types()` | the helper title a model docks with, the types the package has |

Timings, measured live: `RetractWait` 25 s, `ClaimTimeout` 12 s for a placeholder no jetway takes, `EmptyFillTries` 25 while the simulator still loads the airport, `RefillEvery` 2 min, `DockSettle` 2 min between docks, `DockTimeout` 90 s, `HelperDelay` 3 s between a placeholder leaving and its helper arriving.

## Door offsets

Where a jetway meets the visible door differs by type: the jetway docks to the helper, not to our aircraft. `DoorOffset` (metres ahead and right) per type moves the helper so the jetway meets our door; `DoorOffsetFor(model)` finds the longest type a model or title contains. The table is `jetway.doorOffsets` in [Dictionaries](dictionaries.md), replaceable at runtime; none are shipped yet. `HelperOptions.Offset` gives the host's own computation, and `MoveHelpers` turns on actually moving the helper (off by default: live, a helper moved after the simulator placed it reported docked without the jetway moving).

`HelperOptions.OnPlaceholder` tells the host where the simulator parked each placeholder or helper on its gate, so the host can park its own aircraft the same way.
