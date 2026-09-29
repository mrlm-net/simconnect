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
| `parked` | on the ground, not moving |
| `taxiing` | on the ground, up to 40 kt |
| `runway` | on the ground, faster (take-off or landing roll) |
| `departing` / `arriving` | airborne within `AirportTerminalNM` of an airport below 10 000 ft, climbing / descending |
| `enroute` | anything else airborne |

An aircraft on the ground belongs to the airport within `AirportNearNM`. `Airports()` are the airports inside the radius with their distance from the centre. `Events()` reports aircraft and airports entering and leaving, and recentring (dropped when the channel is full; the picture itself stays current).

## Feeding the controllers

- **`Ground(icao)`** is the `GroundPicture` of an airport, shared by its controllers (`TaxiWithGroundPicture`, `ArrivalWithGroundPicture`). `Observe` reports the aircraft on the ground there that are not ours; ours report themselves.
- **`Allocate(icao, allocator)`** feeds a `StandAllocator` from the picture's scans, so it needs no scan of its own.

## On the airport map

The Layers tab has a **Traffic picture** section: centre (follow my aircraft, or this airport), radius, and the airports and aircraft in range by phase. The airports in range fill the ICAO fields (Load, flight plan) as a dropdown. `GET /api/world` returns the picture; `POST /api/world` sets the centre (`{"follow":true}`, `{"icao":"LKPR"}`, `{"lat":…,"lon":…}`) and the radius (`{"radiusNM":250}`).
