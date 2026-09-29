---
title: "Traffic Manager"
description: "Turn a schedule into traffic: departures board and push on time, arrivals fly their STAR, aircraft turn around, and a situation checker adjusts to what is happening — including traffic that is not ours."
order: 10
section: "traffic"
---

# Traffic Manager

`traffic.TrafficManager` runs the schedule of the managed airports (#368). It decides **what happens when**. A **Spawner** does it in the simulator: it knows the stands, models, runways and controllers, and it reports back.

```go
cfg := traffic.DefaultScheduleConfig()
mgr := traffic.NewTrafficManager(spawner, traffic.ManagerOptions{
    Source: func(from, to time.Time, airports []string) []traffic.Flight {
        return traffic.Schedule(cfg, traffic.ScheduleOptions{Focus: airports, Seed: seed ^ uint64(from.Unix()/3600)}, from, to)
    },
    Picture:     picture, // the TrafficPicture: other traffic is respected
    MaxAircraft: 12,
    OnEvent:     func(e traffic.ManagerEvent) { log.Println(e.Kind, e.Flight.Callsign, e.Reason) },
}, "LKPR")

for now := range time.Tick(time.Second) {
    mgr.Tick(now)
}
```

## What it does

| When | What |
|---|---|
| STD − `DepartureLead` (10 min) | A departure is **spawned on a stand** and boards; it pushes at its STD (`TaxiRequest.PushbackAt`). |
| STA − `ArrivalLead` (25 min) | An arrival is **spawned at its STAR entry** and flies the STAR and approach, lands and taxis to a stand. |
| Arrival parked | If the arrival pairs with a later departure of the same airline and type from that airport (`MinTurn` 40 min – `MaxTurn` 3 h after the STA), that departure **adopts the aircraft on its stand** (turnaround). Otherwise the aircraft is removed after `RemoveParkedAfter`. |
| Departure airborne | Removed `RemoveDepartedAfter` (5 min) after it leaves the controllers. |
| Too late | A departure is cancelled 15 min after its STD without an aircraft. An arrival is cancelled 10 min after it should have appeared. A departure waits as long as its inbound aircraft is still on its way. |

The limits are `MaxAircraft` in total and `MaxPerAirport`. Spawns are spaced: arrivals `ArrivalSpacing` (3 min) apart, departures `DepartureSpacing` (1 min). The Source is asked `Horizon` (2 h) ahead, an hour at a time. Flights already too late when they are added are left out, so a schedule started mid-day does not show the morning as cancelled.

## The Spawner

```go
type Spawner interface {
    Spawn(f ManagedFlight)  // must not block: report with Update, Describe, Failed
    Remove(f ManagedFlight)
}
```

- `Update(callsign, status, now)` reports progress: boarding, taxiing, departing, departed, approaching, landed or parked.
- `Describe(callsign, model, stand, runway)` records what the Spawner chose, for the boards.
- `Failed(callsign, err, now)` handles a failed spawn. It is tried again after `RetryAfter`, up to `MaxAttempts`, and the Spawner takes another model or stand on each attempt (`f.Attempts`). An aircraft that was already flying is cancelled and removed.
- `ErrSpawnBlocked` means the place was taken: another aircraft is at the STAR entry, or no stand is free. The spawn waits and is tried again without counting an attempt.
- A Spawner that is also a `Holder` (`Hold(f, on)`) can keep a boarding departure on its stand. The map uses `TaxiController.HoldPushback`.

`ModelsFor(models, airline, name, type, max)` ranks the simulator's aircraft titles for a flight:
1. The type in the airline's livery (`FSLTL_FAIB_B738_TVS-Smartwings`, `FSLTL A20N DLH Lufthansa`).
2. Another type of the same size in the airline's livery, same maker first.
3. The type in any livery.

Stubs, VIP, business-jet, freighter and military versions are left out.

## Situation checks

Each Tick the manager looks at each airport the way the people there would. It predicts what is coming and adjusts. The controllers already do this up close (give way, queue behind, hold for traffic behind the stand); the checks cover the wider picture. A check is a function `func(Situation) []Advice`. `ManagerOptions.Checks` defaults to `DefaultChecks()`:

| Check | Sees | Advises |
|---|---|---|
| `CheckLandingFlow(gap, queue)` | The predicted landing of every arrival: ours, and other traffic arriving (ETA from distance and speed). | **Delay** a scheduled arrival that would land less than `gap` (3 min) behind the one before. The gap doubles while `queue` (2) departures wait. **Estimate** its ETA. |
| `CheckGroundCongestion(max)` | Aircraft taxiing: our departures and arrivals, and other traffic. | **Hold** boarding departures on their stands while `max` (4) or more taxi: a ground stop. |
| `CheckTurnaround(minGround)` | A turnaround's inbound aircraft: parked, or predicted to land. | **Estimate** the departure late when fewer than `minGround` (25 min) remain on the stand. |
| `CheckStuck(after)` | How long a flight stays in one status. | **Remove** an aircraft that stopped making progress (e.g. taxiing 30 min). |

The advice actions are:
- `AdviceDelay` delays a spawn.
- `AdviceHold` holds a departure on its stand while it is advised; the hold is released on the first Tick that no check advises it.
- `AdviceEstimate` sets `Estimated`, shown on the boards with `Note`.
- `AdviceRemove` removes the aircraft.

Write your own checks and add them with `append(traffic.DefaultChecks(), myCheck)`.

## Other traffic

`ManagerOptions.Picture` gives the manager the traffic picture. Aircraft that are not ours count as **other traffic**: MSFS AI, other add-ons and the user. The manager can treat it two ways:

- **`OtherRespect`** (the default): other traffic fills the taxiways, takes landing slots and, through the Spawner, blocks spawn points. `Situation.Others` lists it for your own checks, and `Others(icao)` lists it for the app.
- **`OtherIgnore`**: the manager plans as if it were not there.

Switch between them with `SetOthers`.

## Lifecycle events

Every step is an event, for the app's own state machine: logs, boards, sounds, or rules of its own.
- `ManagerOptions.OnEvent` is called with each event, in order, outside the manager's lock, so it may call the manager.
- `Events()` is a channel of 256 events; events are dropped when it is full.

| Kind | When |
|---|---|
| `added`, `turnaround` | A flight entered the schedule, or an arrival and a departure became one aircraft. |
| `status` | Any status change (`Previous` → `Flight.Status`), including spawning, cancelled and done. |
| `retry`, `blocked` | A spawn failed and will be tried again, or found its place taken. |
| `delayed`, `estimated` | A check moved a spawn, or changed an estimate. |
| `held`, `released` | A ground-stop hold started or ended. |
| `removed` | The aircraft left the simulator. |
| `enabled`, `disabled` | Spawning was switched on or off. |

## Boards

`Board(icao)` returns an airport's departures and arrivals by time. Each `ManagedFlight` carries its status, `Estimated`, `Note` (held, landing flow, late inbound), `TurnFrom`/`TurnTo`, the model, stand and runway, and `Err` when it was cancelled.

## On the airport map

The Traffic tab has a **Scheduled traffic** section: ▶ Start / ■ Stop for the loaded airport, density and max aircraft, and the **Departures** and **Arrivals** boards.

The spawner for scheduled flights:
- picks a model in the airline's livery (`ModelsFor`);
- plans the flight to or from the other end (SID, airways, level), or falls back to the runway's SID or STAR;
- uses the runway in use, a free stand, a tug and automatic de-icing.

Other traffic appears in the Layers tab under **Other traffic**, drawn in blue. It is off by default and listed apart from ours. There you can set whether the schedule respects or ignores it, and ✕ removes one of those aircraft from the simulator.

The API:
- `GET /api/schedule` returns the settings and every flight.
- `POST /api/schedule` takes `{"enabled", "icao", "density", "maxAircraft", "seed", "others": "respect"|"ignore"}`.
- `GET /api/boards?icao=` returns an airport's departures and arrivals.
- `POST /api/world/remove {"objectId"}` removes an aircraft that is not ours.
