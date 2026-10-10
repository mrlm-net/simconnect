---
title: "Checklists"
description: "The aircraft's normal checklists as data with pkg/checklist: a default per family and per-model sets, a local override file, a runner that steps through a list with the roles of the moment and checks each item against systems.State, and checklist findings for flight.Assess."
order: 15
section: "packages"
---

# Checklists

`pkg/checklist` holds the aircraft's normal checklists as data and runs them (#1002). The simulator's own checklists are encrypted and SimConnect has no checklist API, so the lists are ours. The copilot uses them for challenge and response; `flight.Assess` uses them to judge a flight.

## The model

A `Set` holds an aircraft's lists. Each `List` has:

- `Name`: its key, such as `before-start`, `before-takeoff`, `approach`, `landing` or `parking`.
- `Title`: the name as shown.
- When it is due: `Phases` (pkg/pilot's phase names: `ground`, `takeoff`, `climb`, `approach`, `landing`…) or `Stages` the caller names (`before-start`, `after-start`, `before-takeoff`, `after-landing`, `parking`). `Set.Due(phaseOrStage)` lists the ones due, so the lists work with or without the copilot flying.
- `CalledBy` and `ReadBy`: the PF calls for a list and the PM reads it, unless the list says otherwise.

Each `Item` has:

- `Challenge` and `Response` (`"SEAT BELTS"`, `"ON"`).
- `Role`: who responds, `PF`, `PM`, `both`, or empty for either.
- `Check`, optional: a test against `systems.State` by generic value name (`gearDown`, `flapsIndex`, `spoilersArmed`, `seatbelts`, `lightBeacon`, `xpdrState`, `v1Kt`…), plus the derived `enginesRunning` and `doorsClosed`. The value must be `is` true or false, or within `min` and `max`.
- `Action`, optional: a systems action the PM may take to make the item so (signs, lights, arming the spoilers), with `on` (true by default). Without one the item is verify only. Gear, flaps, the autopilot, thrust and the flight controls are never a checklist's actions; the pilot flying owns them, and `Validate` refuses them.

## Sets and overrides

The shipped sets follow the systems profiles' pattern. `A320 family` is the family's default, the Airbus FCOM normal checklist. `Fenix A320` extends it; its systems profile maps the checked values to the Fenix's own variables, so the family's items check as they are. `For(aircraft, local...)` gives an aircraft its family's set, the model's on top, then each local set (a file beside the settings, read with `ReadSet`) on top of that, the GSX way: local wins per value. An override list replaces the fields it gives; its items match by challenge (any case) and are replaced field by field, removed (`"remove": true`) or added at the end.

```json
{"name": "mine", "lists": [{"name": "landing", "items": [
  {"challenge": "AUTOBRAKE", "response": "MED"},
  {"challenge": "CABIN CREW", "remove": true}]}]}
```

## Running a list

`Run(list)` steps through a list. Roles swap mid-flight, so every call takes who is PF now (`copilotPF`): `CalledBy`, `ReadBy` and `Responder` resolve a role to `Player`, `Copilot` or both. `Next(state)` checks the current item and moves on. Each result is `Done`, `NotDone`, or `CantCheck` (no check, or a value this aircraft does not read). `Open()` lists the items found not done, with the action that would fix them where there is one. `List.Verify(state)` checks a whole list at once, and `List.Complete(state)` says whether every checkable item is done.

```go
set, _ := checklist.For(systems.Aircraft{Package: pkg, Title: title, ATCType: typ}, local...)
for _, l := range set.Due("approach") {
    r := checklist.Run(l)
    for !r.Done() {
        it, _ := r.Current()
        who := r.Responder(copilotPF) // says it.Response
        res, _ := r.Next(state)        // done, not done, can't check
        _ = who
        _ = res
    }
}
```

## On a recorded flight

`StateFromSample` turns a `flight.Sample` into `systems.State`: the gear, flaps, spoilers armed, lights, parking brake, engines and autopilot. A recording has no signs or doors, so those items can't be checked. `CompletedAt(list, samples, from, to)` is when a list was first complete. `ForAssess(set, track)` gives `flight.AssessOptions.Checklists`, when the before-take-off and landing checklists were complete. `Assess` then flags a take-off before the before-take-off checklist was done, and a landing checklist not done by 1000 ft, in place of its gear check.

## Importing an aircraft's MSFS checklist

The simulator's own checklists are encrypted, but some community aircraft ship theirs as readable XML (FlyByWire's A32NX: `SimObjects/<aircraft>/Checklist/*.xml`, a checklist and its checkpoint library). `ImportMSFS(name, checklist, libraries...)` reads one into a `Set`:

- Each `Page` becomes a `List` (named from its subject: "Landing Checklist" → `landing-checklist`).
- It is due at its `Step`, by the step's own id (`landing_approach`) and our stage or phase for it (`LANDING_APPROACH` → `approach`, `PREFLIGHT_GATE` → `before-start`, `LANDING_GATE` → `parking`…).
- `Block`s are flattened. Each `Checkpoint` becomes an `Item` with its subject and expectation as said (localisation keys such as `GAME.CHECKLIST_LANDING_GEAR` as their words), and its reference id as the note.
- A checkpoint's test becomes a `Check` only when it is one simple test (a SimVar on, `NOT` it, or `EQUAL` a value) of a SimVar `pkg/systems` names generically: the gear handle, flaps index, lights, parking brake, spoilers armed, seat belts, autopilot master, transponder. The rest is confirmed by the crew.
- The checkpoints' copilot actions are raw key events and are not imported: imported items are verify only.

FlyByWire's A320 imports as 27 lists and 405 items. Only a few items get checks, because most of its tests read its own L:vars or chain several tests.
