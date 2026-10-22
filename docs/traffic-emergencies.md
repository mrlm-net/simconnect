---
title: "Emergencies & Scenarios"
description: "Emergencies in the traffic: MAYDAY and PAN PAN calls, priority landings, departures held, fire and rescue vehicles, held runways; and scripted scenarios and camera scenes played around the traffic."
order: 10
section: "traffic"
---

# Emergencies & Scenarios

The World can play an emergency in its traffic the way a tower and approach unit run one, and can script whole situations around the traffic: an emergency, a closed runway, a go-around, filmed or not.

## Emergencies

An aircraft in an emergency gets **priority**:

- it lands number one at its own time, and the other arrivals are fitted around it in the landing sequence;
- while it is within `priorityHoldNM` (15 NM) of the airport on its way in, no departure takes off: each waits with "priority traffic (call sign)" as its reason;
- the airport's fire and rescue vehicles meet it ([Rescue vehicles](#rescue-vehicles)).

`World.Priority(tail, on)` gives an aircraft priority or takes it away; `World.PriorityFor(tail, kind)` gives it for a kind of emergency, so the right rescue is sent. With `tail` "" it is the user aircraft: the host's own ATC handles its call and the words, and the World sequences the player at the nearest airport of ours within `playerPriorityNM` (60 NM) and keeps its traffic out of the way.

### One of ours declares one

`POST /api/emergency {"tail": "CSA123", "kind": "engine-failure", "level": "", "on": true}` (or a scenario's `emergency` action) has one of our arrivals on its way in declare an emergency. For now only arrivals approaching can. The kinds are `traffic.EmergencyKind`:

| Kind | Call (`traffic.EmergencyLevel`) | Rescue |
|---|---|---|
| `engine-failure` (the default) | MAYDAY | fire trucks |
| `fire` | MAYDAY | fire trucks |
| `pressurisation` | MAYDAY | fire trucks |
| `hydraulic` | PAN PAN | fire trucks |
| `fuel` | PAN PAN | fire trucks |
| `medical` | PAN PAN | an ambulance |

`level` (`mayday` or `panpan`) overrides the call. On the radio, as ICAO phraseology has it (Doc 4444 15.1, CAP 413 chapter 8):

1. the crew squawks 7700 (`traffic.EmergencySquawk`) and calls: "MAYDAY MAYDAY MAYDAY, Ruzyne Approach, CSA123, engine failure, request priority landing runway 24" (`traffic.DeclareEmergency`; PAN PAN three times for an urgency);
2. the controller answers: "CSA123, roger the MAYDAY, squawk 7700, you are number one, runway 24" (`traffic.AcknowledgeEmergency`);
3. the controller asks for what the message should carry, "report persons on board, fuel endurance and intentions" (`traffic.EmergencyQuestions`), and the crew answers with plausible figures for its size: "156 persons on board, endurance 2 hours 10 minutes, request immediate landing runway 24" (`traffic.EmergencyDetails`).

`{"on": false}` cancels it: "cancel MAYDAY", "roger, MAYDAY cancelled, squawk 4521" with its own code back (`traffic.CancelEmergency`, `traffic.AcknowledgeCancelEmergency`), and priority taken away. The wording of the questions and of a cancellation is the project's: the ICAO Phraseology Reference Guide lists the items, not the words.

### Rescue vehicles

As an aircraft with priority comes within `rescueDispatchNM` (20 NM) of an airport of ours, the airport's fire and rescue drive out (`traffic.SimObjectRescue`, placed every frame like the fuel trucks):

- **Who:** `rescueFireTrucks` (2) fire trucks, GSX's (`FSDT_Firetruck_*`) by preference, else MSFS's own; for a medical emergency, an ambulance instead.
- **Standby:** from the nearest vehicle depot (the fire station's stand-in) along the vehicle roads to standby points beside the landing runway, where they wait.
- **Following:** once the aircraft is down they follow it, `RescueGapMeters` (80 m) behind and the second truck 40 m further, at up to `RescueFollowKts` (50 kt), down the runway and to where it stops. An ambulance follows it in to its stand.
- **After:** they stay `rescueStay` (1 min) once it has stopped, `rescueStayMedical` (5 min) for a medical, then drive home and are removed.

A runway the vehicles stand on can be held while they work: `World.HoldRunway(icao, runway, true)` stops all clearances on it without a runway change ([The runway in use](traffic-world.md#the-runway-in-use)).

### The user aircraft's failures

`pkg/failures` fails and restores systems of the user aircraft by generic names (engines, electrical, hydraulics, pitot and static, brakes), through the simulator's `TOGGLE_*_FAILURE` key events, with a default profile and per-model overrides ([Failures](failures.md)). The World does not fail anything itself: a host fails the player's systems and gives the player priority with `Priority("", true)`.

## Scenarios

A scenario is a scripted situation at an airport, played around the schedule's traffic: `World.StartScenario(sc, hooks)` returns a `*ScenarioRun` (`Status()`, `Stop()`). It runs until a step marked `End`, its `LimitSec` (default 900 s), or `Stop`.

```go
on := true
run, err := w.StartScenario(world.Scenario{
    Name: "engine failure on approach", ICAO: "LKPR",
    Cast: []world.ScenarioCast{
        {Role: "arr", Kind: "arrival", Type: "A320", Callsign: "CSA123", Procedure: true},
        {Role: "dep", Kind: "departure", Type: "B738", Callsign: "RYR1527"},
    },
    Steps: []world.ScenarioStep{
        {Cue: "arr:distance:25", Actions: []world.ScenarioAction{{Do: "emergency", Who: "arr", Kind: "engine-failure"}}},
        {Cue: "arr:rollout", Delay: "20s", Actions: []world.ScenarioAction{{Do: "runway", Runway: "24", Kind: "hold", On: &on}}},
    },
}, world.ScenarioHooks{Event: func(e world.ScenarioEvent) { log.Println(e.Kind, e.Role, e.Tail, e.Err) }})
```

- **Cast** (`ScenarioCast`): aircraft of ours pinned to a role, flown by the World like any other. Each has a `Kind` (departure or arrival), a type, call sign and optional model fragments (`Models`), a stand, when it appears (`AtSec`; negative: only on a `spawn` action), a departure's push time (`PushAfterSec`), whether it flies a procedure (a SID, or an arrival from a STAR's first fix; else on the final, cleared at once) and whether it gets a tug.
- **Steps** (`ScenarioStep`) wait side by side, each for its cue, then its `Delay` (a Go duration), then run their actions once.
- **Cues:** `start`; `t:SEC` since the scenario began; `after:ID` once step ID has run; `ROLE:STATE` a controller state reached (pushback, taxiing, holding short, lining up, departing, approaching, landing, parked, …); `ROLE:airborne`; `ROLE:heard` (a call to or from it); `ROLE:lights:X` (N B S T L O W); `ROLE:height:FEET` above the runway after its take-off; `ROLE:distance:NM` within NM of the airport (`>NM` beyond it). Anything else goes to `ScenarioHooks.Cue`. Time is the World's clock: a pause stops it.
- **Actions** (`ScenarioAction.Do`):

| Action | Does |
|---|---|
| `spawn` | `Who` appears now |
| `push` | `Who` is cleared to push back |
| `goAround` | `Who` is sent around |
| `hold` | `Who` holds at its STAR's hold (`On` false: leaves it) |
| `runway` | `Runway` closed (`On` true) or open again; with `Kind` "hold" held a while instead, the runway in use kept; with `Use` made the runway in use ("" back to the wind) |
| `emergency` | `Who` declares an emergency of `Kind`, at `Level`; `On` false cancels it |
| `say` | to `Who`: `cancelTakeoff`, `holdPosition`, `goAround`, `expectDelay` (`Args["minutes"]`) or `custom` (`Args["text"]` said as written, `Args["station"]` the unit) |
| `camera` | camera shots on the cast |

Anything else goes to `ScenarioHooks.Action`. `ScenarioHooks.Event` reports each cast member appearing, each step and action (with its error), and the end; `ScenarioHooks.Fill` fills a custom call's placeholders as it is said.

## Camera scenes

A camera scene is a scenario that films: it spawns its cast, listens to one of them on the radio, and plays beats, each cutting to its shots ([Camera](camera.md)). Scenes are JSON files in the airport map's `-scenes` folder, read on every run, with built-in ones as the fallback: `GET /api/camera/scenes` lists them and `POST /api/camera/scene?name=&icao=` plays one. A scene can set the airport up first (`Jetways`, `Schedule` with a density), have a cast member declare an emergency (`SceneCast.Emergency`), run the scenario actions at a beat (`SceneBeat.Actions`: a runway closed or held, an "expect delay"), and run `EndActions` however it ends (the runway opened again). Besides the scenario cues, a scene waits on `ROLE:rescue` (rescue vehicles sent for it) and `ROLE:rescue:with` (down, a vehicle within 80 m of it).
