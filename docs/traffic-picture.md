---
title: "Traffic Picture"
description: "All traffic around a centre of the world — fixed or following the user's aircraft — with phases, airports in range, and the ground and stand feeds for the controllers."
order: 8
section: "traffic"
---

# Traffic Picture

`traffic.TrafficPicture` is one picture of all traffic around a **centre of the world** (#366): our controlled aircraft, MSFS AI and the user, with where each one is and what it is doing, and the airports inside the radius. The centre is fixed — an airport or a position — or follows the user's aircraft; the library never assumes which.

```go
picture := traffic.NewTrafficPicture(traffic.PictureOptions{
    Centre:   traffic.Centre{FollowUser: true}, // or {ICAO: "LKPR"}, or {Position: ...}
    RadiusNM: 250,                              // default DefaultPictureRadiusNM
})
lister := traffic.NewAirportLister(client, 0)
lister.Request()

for msg := range client.Stream() {
    if airports, ok := lister.Handle(msg); ok {
        picture.SetAirports(airports)
    }
    // … your aircraft scan (RequestDataOnSimObjectType) → []traffic.Observation
    picture.Observe(time.Now(), scan)
}
```

## Feeding it

The picture is fed, not self-driving:

- **`Observe(now, scan)`** with each aircraft scan. SimConnect's `RequestDataOnSimObjectType` reaches at most `MaxScanRadiusMeters` (200 km): MSFS AI farther away is not seen. Aircraft outside the radius, or not seen for `PictureStaleAfter`, leave the picture.
- **`SetAirports`** with the airports around, e.g. from `AirportLister` (SimConnect's facilities list: the airports the simulator has loaded around the user, about 180 NM in MSFS 2024). `AddAirport` adds one it does not reach, such as a flight's destination.
- **`SetOwn(objectID, phase, icao)`** for the aircraft our controllers drive — they know their phase best; `ForgetOwn` when a controller lets go.

## Centre

A fixed centre stays put; `SetCentre` moves it, `SetRadius` changes the radius. A centre following the user moves only once the user is `RecentreNM` (25 NM) from it, so the picture does not churn with every scan.

## What it knows

`Aircraft()` — nearest to the centre first — gives each aircraft's scan data with its **phase** and **airport**:

| Phase | For aircraft not ours |
|---|---|
| `parked` | on a stand below `StandMovingKts` (3 kt: creeping into position is not taxiing), or still where the layout is unknown |
| `pushback` | moving tail first: the track more than `PushbackOffNoseDeg` (120°) off the nose, at 1 kt or more |
| `holding` | stopped off a stand |
| `taxiing` | moving on the ground off the runways |
| `runway` | on a runway, slow (lining up, vacating, waiting on it) |
| `takeoff` / `landing` | on a runway faster than `RollKts` (30 kt); a landing roll within `LandingRollFor` (90 s) of touching down |
| `departing` / `arriving` | climbing / descending within `AirportTerminalNM` of an airport below 10 000 ft |
| `approach` | descending below `ApproachBelowFt` (3000 ft) near an airport; it lasts until a climb (a go-around) or back above 4000 ft |
| `climbing` / `descending` | climbing / descending elsewhere |
| `enroute` | level in the air |

The simulator reports 0 kt for AI aircraft on the ground however they move (measured at LKPR: taxiing at 6–32 kt by their positions, 0.0 reported). Below `MovingKts` on the ground the picture works out the speed from the movement since the last scan (`SpeedDerived`, #622); such a speed counts as movement only above `DerivedMovingKts` (2 kt), as a metre of jitter between scans is about 1 kt. Climbing and descending follow the altitude over the last `ProfileWindow` (30 s), not one scan's vertical speed: a climb or descent starts past `ProfileEnterFpm` (400 ft/min) and ends inside `ProfileLeaveFpm` (150); with less than `ProfileMin` (10 s) of history the first scan's phase stands (#623).

On the ground, **where** an aircraft is comes from `airport.Locate` among the airports within `AirportNearNM` whose layout `PictureOptions.Layout` gives (`Where`: runway, parking, taxiway; `WhereName`: "06/24", "C22", "A"). Without layouts the phase is by speed alone and the airport the nearest within `AirportNearNM`. The phases follow the MyCrew app's observer, which measured the simulator live (mycrew-online/app `internal/agent/traffic_phase.go`).

An aircraft on the ground belongs to the airport within `AirportNearNM`. `Airports()` are the airports inside the radius with their distance from the centre. `Events()` reports aircraft and airports entering and leaving, and recentring (dropped when the channel is full; the picture itself stays current).

## Feeding the controllers

- **`Ground(icao)`** is the `GroundPicture` of an airport, shared by its controllers (`TaxiWithGroundPicture`, `ArrivalWithGroundPicture`; see [Ground traffic](traffic-taxi.md#ground-traffic)). `Observe` reports the aircraft on the ground there that are not ours; ours report themselves.
- **`Allocate(icao, allocator)`** feeds a `StandAllocator` ([Stand allocation](traffic-arrival.md#stand-allocation)) from the picture's scans, so it needs no scan of its own.
- **`ManagerOptions.Picture`** gives it to the [Traffic Manager](traffic-manager.md#other-traffic), which counts the aircraft that are not ours as other traffic and follows its own airborne flights in it.

## On the airport map

The Map section has the **Traffic picture**: the centre (follow my aircraft, this airport, or the map centre), the radius, the airports and aircraft in range by phase, and the load of the aircraft we drive. The 🌐 button (bottom left) is the **world view** (#371): the map zooms out to the picture's circle and shows the airports in range and every aircraft, coloured by phase and labelled with call sign, level, phase and destination. Other traffic appears there only when its layer is on. Press 🌐 again to go back to the airport. The airports in range fill the ICAO fields (Load, flight plan) as a dropdown. `GET /api/world` returns the picture; `POST /api/world` sets the centre (`{"follow":true}`, `{"icao":"LKPR"}`, `{"lat":…,"lon":…}`) and the radius (`{"radiusNM":250}`).
