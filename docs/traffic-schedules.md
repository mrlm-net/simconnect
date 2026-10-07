---
title: "Traffic Schedules"
description: "Scheduled flights for the airports around the centre of the world: airlines, fleets, routes and time-of-day waves, and overflights crossing the area; deterministic for a seed."
order: 9
section: "traffic"
---

# Traffic Schedules

`traffic.Schedule` generates the flights that should exist at the **focus airports**, for example the airports in the [traffic picture](traffic-picture.md) (#367). It returns a timetable: who departs and arrives where and when. The [Traffic Manager](traffic-manager.md) turns the timetable into aircraft (#368).

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

- **How many:** in each hour, `PeakPerHour` (default 24 movements) × the airport size (hub 1, major ½, regional ⅕) × `Density` × the hour's **wave**. The waves are by local solar time, from the airport's longitude. The built-in day is quiet at night and has a morning and an evening peak. Departures and arrivals are about even.
- **Which airline:** airlines are weighted by `Weight`. **Home carriers** get 8× the weight: an airline is a home carrier when the airport is one of its `Bases` or when one of the airport's stands names it (`Parking.Airlines` in the layout). Other airlines fly only where their `Regions` (ICAO prefixes, `*` for anywhere) cover the airport.
- **Where to:** a visiting airline flies to one of its bases. A home carrier flies anywhere in its regions, and bigger airports are picked more often.
- **Which type:** a type from the airline's fleet whose range (`Types[t].MinNM`–`MaxNM`) covers the distance and whose runway need fits both airports. For example, an ATR flies short hops and a 777 flies to Dubai.
- **Times:** STD and STA fall on whole five minutes. Block time is 20 minutes plus the distance at cruise speed.
- **Call signs:** the airline ICAO code plus a flight number, each used once in a schedule. `ScheduleOptions.Used` lists call signs already flying or planned (the hours scheduled before), and none of them is given again.

A focus airport that is missing from the config still gets traffic when its layout is passed (as a regional airport). The runway lengths then come from the layout.

## Business and light aircraft

Two more generators add general aviation:

- `traffic.BusinessFlights(cfg, BusinessOptions{Focus, Layouts, PerHour, Density, Seed}, from, to)` gives business jets and turboprops flying IFR in to and out of the large focus airports (#619). An airport is large with a runway of `LargeRunwayMeters` (3000 m) or more and `LargeGates` (10) gates or more (`LargeAirport`). It gives on average `BusinessPerHour` (1.5) arrivals and as many departures an hour at the peak, following the waves. The types come from `BusinessTypes` by weight, within their range and runway. The call sign is a registration, and `Operator` is "business".
- `traffic.VFRFlights` gives light aircraft through the circuit at the smaller fields ([VFR traffic](traffic-vfr.md)).

## Overflights

`traffic.Overflights(cfg, OverflightOptions{…}, from, to)` generates the flights that only cross the area: between the config's airports outside it, whose route passes through it (#369).

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

## Editing the data

`DefaultScheduleConfig()` ships with:

- 17 airlines, including CSA and Smartwings at Prague, Lufthansa, KLM, Ryanair, Wizz Air and the Gulf carriers.
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
