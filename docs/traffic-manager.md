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
| STA − `VFRLead` (8 min) | A VFR arrival (`Flight.Rules`) appears near the airport to join the circuit ([VFR traffic](traffic-vfr.md)). |
| Arrival parked | If the arrival pairs with a later departure of the same airline and type from that airport (`MinTurn` 40 min – `MaxTurn` 3 h after the STA), that departure **adopts the aircraft on its stand** (turnaround). Otherwise the aircraft is removed after `RemoveParkedAfter`. |
| Departure airborne | Flies on along its plan and is removed once it leaves the area (see [Leaving](#enroute-traffic-and-overflights)), at the latest `RemoveDepartedAfter` (30 min) after it leaves the controllers. |
| Too late | A departure is cancelled 15 min (`DepartureLate`) after its STD without an aircraft. An arrival is cancelled 10 min (`ArrivalLate`) after it should have appeared. A departure waits as long as its inbound aircraft is still on its way. |

The limits are `MaxAircraft` in total (default 24) and `MaxPerAirport` (default 16). Spawns are spaced: arrivals `ArrivalSpacing` (3 min) apart, departures `DepartureSpacing` (1 min). The Source is asked `Horizon` (2 h) ahead, an hour at a time. Flights already too late when they are added are left out, so a schedule started mid-day does not show the morning as cancelled. A failed spawn is tried again after `RetryAfter` (30 s), up to `MaxAttempts` (3); one unanswered for `SpawnTimeout` (2 min) counts as failed. Done and cancelled flights stay on the boards for `Keep` (1 h).

`Add(flights)` adds flights beside the Source's (flights at chosen times); a flight already known by kind and call sign keeps its state. `SetAirports`, `SetEnabled` and `SetLimits` change the managed airports, spawning and the limits while it runs; `Replan` asks the Source again for the hours ahead.

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

Within each of the first two, titles with the airline's ICAO code come before those that only carry its name, which may be a sister airline's ("TVP-Smartwings Poland" for Smartwings, TVS). Stubs, VIP, business-jet, freighter and military versions are left out.

`ModelsForFlight(models, airline, name, type, callsign, max)` ranks the same way, but takes the equally good best titles in turn by call sign. Each flight always gets the same one, and a fleet with several liveries or versions installed shows them all instead of one aircraft. The map spawns with it.

## Situation checks

Each Tick the manager looks at each airport the way the people there would. It predicts what is coming and adjusts. The controllers already do this up close (give way, queue behind, hold for traffic behind the stand); the checks cover the wider picture. A check is a function `func(Situation) []Advice`. `ManagerOptions.Checks` defaults to `DefaultChecks()`:

| Check | Sees | Advises |
|---|---|---|
| `CheckLandingFlow(gap, queue)` | The predicted landing of every arrival: ours, and other traffic arriving (ETA from distance and speed). | **Delay** a scheduled arrival that would land less than `gap` (3 min) behind the one before. One gap is doubled while `queue` (2) or more departures wait, and the weather on final (`ManagerOptions.Conditions`) stretches every gap: twice in low visibility, a third more on a contaminated runway ([Weather on final](traffic-separation.md#weather-on-final)). **Estimate** its ETA. |
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

## Enroute traffic and overflights

Traffic flies between airports, not only at one (#369).

- **Enroute arrivals:** an arrival appears `EnrouteLead` (20 min) before it would appear at its STAR entry. It shows up **en route** on its flight plan, at the point the plan puts it now, at the planned level. MSFS AI flies it to the entry, where the Spawner **hands it over** to the arrival controller (status `enroute`, then `approaching`). If the enroute spawn fails, the arrival appears at its STAR entry instead, with no attempt lost. A negative `EnrouteLead` turns enroute arrivals off.
- **Overflights:** `ManagerOptions.Overflights` gives flights between airports outside the area whose route crosses it. `traffic.Overflights(cfg, OverflightOptions{Centre, RadiusNM, PerHour, Density, Seed, Exclude}, from, to)` generates them: airports outside the area, an airline serving both ends, a type that fits, and `Enter`/`Exit` when the flight crosses into and out of the area. Each appears at `Enter`, where its flight is by then, up to `MaxOverflights` (4) at once.
- **Departures fly on** after their SID along their plan, and stay ours.
- **Leaving:** an airborne aircraft of ours (departed, en route, overflying) is removed once the picture has not seen it for `LeftAfter` (1 min), i.e. it left the area. `RemoveDepartedAfter` (30 min) and an overflight's `Exit` are the fallbacks.

`Attach(callsign, objectID)` tells the manager which aircraft flies a flight, so that it can follow it in the picture.

On the map an enroute arrival keeps its distance to the one ahead of it in the landing sequence (`pkg/traffic/world/enroute_pace.go`). Every 10 s, once it is within 10 NM of its leader and faster, its route ahead is flown no faster than the leader's ground speed (at least 210 kt). It gets its planned speeds back once the gap has opened past 14 NM. A change under 10 kt is not made.

### Appearing airborne

`traffic.EnrouteStart(route []RoutePoint)` turns the rest of a flight (points with altitude and speed, `EnrouteSpeedKts`) into two things:
- where the aircraft appears: at the first point, heading for the second, at its altitude and speed;
- the waypoint chain it flies from there.

Create the aircraft with `Fleet.RequestNonATC` at that position, then `ReleaseControl` and `SetWaypoints` once it exists. `nav.FlightPlan.PositionAt(distNM)` gives the point, planned altitude and track along a plan.

A flight plan cannot start an aircraft mid-route in MSFS 2024. `AICreateEnrouteATCAircraft` puts it on the ground at the plan's departure airport whatever the phase. It also refuses a plan whose departure airport the simulator has not loaded.

## Real-world flights

A flight with `Observed` set is a real aircraft a feed sees (#841, `manager_observed.go`; the World drives it, see [Traffic World](traffic-world.md#real-world-traffic-v022)). It is added with `Add` like any flight and handled differently:

- it is not spaced from other spawns, never cancelled as too late, and a real arrival is never delayed by a check (it is in the air already);
- it turns around only as the feed says: `Turn(arrival, departure)` makes the departure adopt the arrival's aircraft once it has parked;
- `Observe(kind, callsign, sighting, origin, destination)` takes a later sighting: all of it before the spawn, only the registration, origin and destination after;
- `Retime(callsign, std)` moves a departure's STD while it is still on its stand;
- `Drop(kind, callsign, now)` ends it: not spawned, boarding or parked, it goes now; in progress it plays out (an arrival lands and parks, then goes; a departure leaves).

`Flight(kind, callsign)` returns one managed flight. `AddOverflight(flight)` adds a real overflight (with `Enter` and `Exit`), flown from its sighting across the area.

## Replaying a recorded day

A scheduled flight can carry its recorded way, `Flight.Path` (#845): points with `lat`, `lon`, `altFt` and the time `at` it was there. The flights are scheduled as usual (`Add`, with their STD, STA, or `Enter`/`Exit`), and the World flies each along its path:

- an arrival or an overflight appears, when its en-route stage starts, where its path has it at that moment on the simulator's clock (between the timed points either side, at the speed of that leg), and flies the rest. An arrival flies on to where the path meets a STAR of the runway in use; an overflight flies across the area;
- a departure flies its SID, then its path;
- an untimed path starts at its first point, flown at 420 kt.

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

The Schedule section has **Scheduled traffic**: ▶ Start / ■ Stop, density and max aircraft, the **airports** whose timetables run (the loaded one by default; others are loaded on start), and the **Departures**, **Arrivals** and **Overflights** boards, with a board per managed airport (#371).

The spawner for scheduled flights (enroute arrivals and overflights appear airborne, see above; their labels show call sign, flight level and destination, and **Overflights** is a third board):
- picks a model in the airline's livery (`ModelsFor`);
- plans the flight to or from the other end (SID, airways, level), or falls back to the runway's SID or STAR;
- uses the runway in use, a free stand, the airport's service vehicles (tug, fuel truck, stairs, GPU: [Ground services](traffic-world.md#ground-services-v020)) and automatic de-icing.

Other traffic is a layer in the Map section (**Other traffic**), drawn in blue. It is off by default and listed apart from ours. There you can set whether the schedule respects or ignores it, and ✕ removes one of those aircraft from the simulator.

The API:
- `GET /api/schedule` returns the settings and every flight.
- `POST /api/schedule` takes `{"enabled", "airports": ["LKPR", "LKTB"] (or "icao"), "density", "maxAircraft", "seed", "ifr", "vfr", "generator", "offsetMin", "others": "respect"|"ignore"}` (`world.ScheduleSettings`).
- `GET /api/flights` lists the manager's flights; `POST /api/flights` adds flights ([Traffic World](traffic-world.md#flights-at-a-chosen-time)).
- `GET /api/boards?icao=` returns an airport's departures and arrivals.
- `POST /api/world/remove {"objectId"}` removes an aircraft that is not ours.
