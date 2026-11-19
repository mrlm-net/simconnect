---
title: "Traffic Schedules"
description: "Scheduled flights for the airports around the centre of the world: airlines, fleets, routes and time-of-day waves, and overflights crossing the area; deterministic for a seed."
order: 4
section: "traffic"
---

# Traffic Schedules

`traffic.Schedule` generates the flights that should exist at the **focus airports**, for example the airports in the [traffic picture](traffic-picture.md). It returns a timetable: who departs and arrives where and when. The [Traffic Manager](traffic-manager.md) turns the timetable into aircraft.

```go
cfg := traffic.DefaultScheduleConfig() // or traffic.LoadScheduleConfig("schedule.json")
flights := traffic.Schedule(cfg, traffic.ScheduleOptions{
    Focus:   []string{"LKPR", "LKTB"},
    Density: 1,   // scales everything
    Seed:    42,  // same seed, same timetable
    Layouts: map[string]*airport.Layout{"LKPR": layout}, // optional
}, from, to)

for _, f := range flights {
    fmt.Println(f.Callsign, f.Type, f.Origin, "→", f.Destination, f.STD, f.STA)
}
// CSA809   A320 LFPG→LKPR 05:15 06:35
// SWR643   A20N LKPR→LSGG 06:40 07:55
```

Each `Flight` either departs from a focus airport or arrives at one between `from` and `to`. Flights are sorted by their time at the focus airport.

## How flights are drawn

- **How many:** in each hour, `PeakPerHour` (default 24 movements) × the airport size (hub 1, major ½, regional ⅕) × `Density` × the hour's **wave**. The waves are by the airport's clock, its civil time with summer time (`traffic.LocalHour`, `AirportZone`: the zone by ICAO prefix, the US by longitude), as airline timetables are; solar time from the longitude where the region is not known. Overflights follow the first focus airport's clock. The built-in day is quiet at night and has a morning and an evening peak. Departures and arrivals are about even.
- **Which airline:** airlines are weighted by `Weight`. **Home carriers** get 8× the weight: an airline is a home carrier when the airport is one of its `Bases` or when one of the airport's stands names it (`Parking.Airlines` in the layout). Other airlines fly only where their `Regions` (ICAO prefixes, `*` for anywhere) cover the airport.
- **Where to:** a visiting airline flies to one of its bases. A home carrier flies anywhere in its regions, and bigger airports are picked more often.
- **Which type:** a type from the airline's fleet whose range (`Types[t].MinNM`–`MaxNM`) covers the distance and whose runway need fits both airports. For example, an ATR flies short hops and a 777 flies to Dubai.
- **Times:** STD and STA fall on whole five minutes. Block time is 20 minutes plus the distance at cruise speed.
- **Call signs:** the airline ICAO code plus a flight number, each used once in a schedule. `ScheduleOptions.Used` lists call signs already flying or planned (the hours scheduled before), and none of them is given again.

A focus airport that is missing from the config still gets traffic when its layout is passed (as a regional airport). The runway lengths then come from the layout.

## Business and light aircraft

Two more generators add general aviation:

- `traffic.BusinessFlights(cfg, BusinessOptions{Focus, Layouts, PerHour, Density, Seed}, from, to)` gives business jets and turboprops flying IFR in to and out of the large focus airports. An airport is large with a runway of `LargeRunwayMeters` (3000 m) or more and `LargeGates` (10) gates or more (`LargeAirport`). It gives on average `BusinessPerHour` (1.5) arrivals and as many departures an hour at the peak, following the waves. The types come from `BusinessTypes` by weight, within their range and runway. The call sign is a registration, and `Operator` is "business".
- `traffic.VFRFlights` gives light aircraft through the circuit at the smaller fields ([VFR traffic](traffic-vfr.md)).

## Overflights

`traffic.Overflights(cfg, OverflightOptions{…}, from, to)` generates the flights that only cross the area: between the config's airports outside it, whose route passes through it.

```go
over := traffic.Overflights(cfg, traffic.OverflightOptions{
    Centre:   centre,           // e.g. the traffic picture's
    RadiusNM: 250,
    PerHour:  6,                // in the peak hour (default 6), scaled by the waves and Density
    Seed:     42,
    Exclude:  []string{"LKPR"}, // the focus airports: overflights neither start nor end there
}, from, to)
```

Each has `Enter` and `Exit`, when it crosses into and out of the area at its cruise speed; they are zero for other flights. Like `Schedule`, it is deterministic for a seed. The [Traffic Manager](traffic-manager.md#enroute-traffic-and-overflights) spawns both: `ManagerOptions.Source` for the schedule and `ManagerOptions.Overflights` for the overflights.

### Streams and volume per area

Real traffic over an area runs in strong, place-specific streams at very different volumes by region: Western Europe to and from the Gulf and Asia over Central Europe, the North Atlantic's westbound day and eastbound night waves near London, leisure flows to the Mediterranean. With `OverflightOptions.Flows` (`POST /api/schedule` with `"overflightFlows": true`) the overflights follow the area's streams:

- The **flows table** `traffic.overflightFlows` (replaceable at runtime like the other [tables](dictionaries.md), local values win) has an entry per area: a latitude/longitude box (a FIR's, roughly), `peakPerHour` (the overflights an hour when every stream is at its peak, at density 1) and its `streams`: `from` and `to` regions as ICAO prefixes (`"EG"`, `"K"`, `"*"` any), a `weight` against the others, `hoursUTC` (24 values, the stream's volume by UTC hour against its peak; none: flat all day) and `minNM` (the shortest pair, 200 NM by default).
- Each hour, the mean is `peakPerHour × Density × Σ weight × hoursUTC[h] / Σ weight` (`traffic.OverflightRate`). Each overflight draws a stream by its weight that hour, then a pair of the stream's airports outside the area whose great circle crosses it (bigger airports more often), then an airline serving both ends; the type comes from its fleet by range, so long-haul streams fly long-haul types.
- **Generic fallback:** an area with no entry gets its volume from the airport list: the pairs whose great circle crosses it, by their sizes (`traffic.FallbackOverflightsPerHour`, between 1 and 40 an hour, none where no pair crosses), the pairs drawn by size and the hours by the first focus airport's waves, as without the flows. About 14 an hour over Prague with the built-in airports, 5 over Košice, 1 over the Faroes.
- `overflightsPerHour`, when set, is the area's peak volume instead of the table's; `Density` scales it as everything else. `GET /api/schedule` shows `overflightArea` and `overflightPeakPerHour`.
- Overflights never start or end at a focus airport, and the minimum pair distance is kept, as before.

Shipped entries (the project's estimates, not from a source): `LKAA` (Praha FIR, 36 an hour at the peak: Western Europe ↔ the Gulf and Asia, Germany and Benelux ↔ south-east Europe, the north ↔ the Mediterranean, Poland and the Baltics ↔ the west) and `EGTT-LON` (London, 24 an hour: the North Atlantic both ways, Ireland and the north ↔ the continent and the Mediterranean). Everywhere else the fallback.

```json
{"area": "LKAA", "name": "Praha FIR", "north": 51.2, "south": 48.4, "west": 11.9, "east": 19.0, "peakPerHour": 36,
 "streams": [{"name": "Western Europe to the Gulf and Asia", "from": ["EG", "EH", "LF"], "to": ["OM", "OT"], "weight": 3,
              "hoursUTC": [0.1, 0.05, "… 24 values …"]}]}
```

Without the setting the overflights are drawn as before: `PerHour` on the waves, the pairs by size.

### Route structure

With `"overflightRoutes": true` each overflight crosses by the airspace's route structure, from the table `nav.routeStructure` (replaceable like the other [tables](dictionaries.md)). An entry is a box, `freeRouteFt` (free route at and above it), `airwaysOnly`, and optional named entry and exit `points`.

- **Free route** at its level: direct on the great circle. When the entry lists points, it goes via the point nearest where it enters and the one nearest where it leaves (within 40 NM, picked per call sign).
- **Elsewhere:** along its plan's airways, as an arrival's (direct where the World knows no airways).
- **Lateral spread:** each overflight flies 0, 1 or 2 NM right of its route, picked per call sign (a strategic lateral offset), so flights on one route are not on one line. Levels still follow the semicircular rule.
- Shipped: `LKAA` (free route from FL245) and `EGTT-LON` (airways only), the project's estimates; an area with no entry is airways only.

### Transits

With `"transits": true` the departures and arrivals of the airports in and near the area (within its radius plus 100 NM, not the focus airports) cross it on their climb or descent (`traffic.Transits`).

- **Volume:** for each such airport, per hour, about 24 × its size share × its local wave × density. The other end is drawn by size, at least 100 NM away, on a great circle passing within 60 % of the radius of the centre.
- **Flown on its profile:** each is flown on its flight plan's profile (`nav.Plan`: climb, the cruise level for its distance and type, descent from its top of descent). It appears where it is above 10,000 ft and inside the area, out of view (past the materialise range, else 40 NM, plus 5), and goes 60 NM past the area or at 10,000 ft on its descent, out of view.
- **Level planning:** its level is planned up front against everyone's planned ways: its top as planned, else a lower level of its direction (a climb levels off sooner, a descent starts lower). Never a hold or an orbit.
- **To the World:** each is an overflight (its Enter and Exit, the tiers, the caps); `GET /api/schedule` lists their call signs in `transitFlights`.

### Keeping clear of the terminal airspace

With `"crossingKeepClear": true` overflights and transits keep out of the terminal airspace.

- **What it is:** the focus airports' terminal airspace, and with the real airspaces (`world.Options.Airspaces`) the other airports' within the area's radius plus 20 NM, the flight's own ends excepted. It is the control zone and the terminal areas stacked on it (`airspace.Store.TerminalStack`, a generic rule on the records: from the class B–D record holding the airport from the surface, each next layer is the controlled record holding the airport whose floor is the last one's ceiling, up to FL245). LKPR: Ruzyne CTR to 3,500 ft, Prague TMA II to 9,500 ft, TMA IX to 16,500 ft. Without the airspaces it is a generic cylinder round each focus airport: 30 NM, to 12,500 ft above the field.
- **How:** no level is given that comes into a layer (with 1,000 ft to spare) inside its outline. A transit whose every top would comes round it: its points within the area's reach move square off its track, 3 NM outside. Separation from the focus airports' own traffic is the up-front plan's: departures' climb clearances and arrivals' levels are planned against everyone's planned ways, crossing traffic's among them.

### Distance tiers

Every overflight in the simulator is an object that costs the simulator's main thread, seen or not. With the tiers on (`world.Options.Overflights`, or `POST /api/schedule` with `overflightMaterialiseNM`), an overflight far from the viewer is a **virtual track**: the World flies it along its planned waypoints (`traffic.VirtualFlight`, the flying model of the offline simulator), with no simulator object and no request. The viewer is the user aircraft and the aircraft our camera is on.

- **Materialise:** within `overflightMaterialiseNM` of a viewer, the nearest virtual tracks are created as NonATC aircraft at exactly the track's pose (position, level, heading, speed) and given the waypoints they have left.
- **Dematerialise:** farther than `overflightDematerialiseNM` from every viewer (default 10 NM past the materialise range), the aircraft is removed and flown on virtually from where it was. The gap keeps an aircraft from flipping at the edge.
- **Caps:** `maxOverflights` then counts the overflights in the simulator, nearest first; `maxVirtualOverflights` (default 200) counts all of them at once, virtual or not.
- **Still traffic:** a virtual track is on the map (`GET /api/traffic`) and in `Snapshot().Virtual`, marked `"virtual": true`, with an ID from `0xFF000000` up. The levels and releases of the other traffic are planned clear of it, and the user aircraft's traffic information tells it. `GET /api/schedule` shows the ranges and `virtualOverflights`, how many are virtual now.

Off by default: with no materialise range, every overflight is in the simulator for its whole crossing, as before. A negative `overflightMaterialiseNM` turns the tiers off again. About the visibility from the flight deck (40 NM) is enough. Offline at LKPR, 60 overflights an hour with a 40 NM range kept at most 7 in the simulator (20, the cap, with the tiers off) and cut the overflights' data requests from about 1050 to 160 a minute.

## Editing the data

`DefaultScheduleConfig()` ships with:

- 17 airlines, including CSA and Smartwings at Prague, Lufthansa, KLM, Ryanair, Wizz Air and the Gulf carriers.
- 6 cargo airlines (`"cargo": true`, #1300) from their hubs: European Air Transport (DHL, Leipzig), AeroLogic, Lufthansa Cargo, UPS, FedEx and ASL Airlines Ireland. A few flights a day at Prague; they fly freighters and park on cargo stands first.
- About 80 airports with their position, size and longest runway.
- The waves.
- Range and runway limits for the known types.

`SaveScheduleConfig(path, cfg)` writes the config as JSON. Edit the file and read it back with `LoadScheduleConfig(path)`:

```json
{
  "airlines": [{"icao": "CSA", "telephony": "CSA", "fleet": {"A320": 1, "AT76": 1},
                "bases": ["LKPR"], "regions": ["LK", "ED", "EG"], "weight": 1}],
  "airports": [{"icao": "LKPR", "position": {"lat": 50.1008, "lon": 14.26}, "size": 2, "runwayM": 3715}],
  "waves": [0.05, 0.02, "… 24 values …"],
  "types": {"A320": {"MinNM": 150, "MaxNM": 3000, "RunwayM": 2000}}
}
```

A type that is not in `types` flies 100–2500 NM and needs a 2000 m runway.
