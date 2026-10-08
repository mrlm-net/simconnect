---
title: "Geodesy and Unit Conversion"
description: "pkg/calc for distances, bearings, tracks, wind and Dubins paths; pkg/convert for units, local offsets, angles and ICAO codes."
order: 16
section: "packages"
---

# Geodesy and Unit Conversion

Two small packages of plain functions, standard library only and on every platform. Latitudes and longitudes are decimal degrees, headings degrees true unless named magnetic.

## pkg/calc

| Function | Gives |
|---|---|
| `HaversineMeters`, `HaversineNM`, `HaversineKM(lat1, lon1, lat2, lon2)` | Great-circle distance (mean Earth radius 6,371,000 m) |
| `BearingDegrees(lat1, lon1, lat2, lon2)` | Initial great-circle bearing, [0, 360) |
| `BearingFromOffsets(xEast, zNorth)` | Bearing to a point in a local frame (X east, Z north, as SimConnect's airport frame); 0 at the origin |
| `CrossTrackMeters(latA, lonA, latB, lonB, latD, lonD)` | Distance of D from the great circle A → B: positive right of track, negative left |
| `AlongTrackMeters(latA, lonA, latB, lonB, latD, lonD)` | Distance from A to the point of A → B nearest D: negative behind A |
| `DisplaceByHeading(lat, lon, hdgDeg, distanceMeters)` | The point `distanceMeters` along `hdgDeg` (negative: backwards); sub-metre to about 50 km |
| `IntermediatePoint(lat1, lon1, lat2, lon2, f)` | The point a fraction `f` (0–1) along the great circle |
| `Dubins(lat1, lon1, heading1, lat2, lon2, heading2, r, step, first)` | Shortest path with turn radius `r` m between two positions and headings, as `[lat, lon]` points every `step` m; `first` +1 right, -1 left, 0 either; `nil` when none |
| `TrueToMagnetic(trueHeading, magVar)`, `MagneticToTrue(magneticHeading, magVar)` | Heading conversion, [0, 360); `magVar` positive east |
| `WindCorrectionAngle(windDir, windSpeed, tas, course)` | WCA in degrees: fly course + WCA; 0 when `tas` is about 0 |
| `HeadwindCrosswind(windDir, windSpeed, runwayHeading)` | Headwind (negative: tailwind) and crosswind (positive: from the right); also `HeadwindComponent`, `CrosswindComponent` |

```go
d := calc.HaversineNM(50.1008, 14.26, 48.1103, 16.5697) // LKPR → LOWW
brg := calc.BearingDegrees(50.1008, 14.26, 48.1103, 16.5697)
head, cross := calc.HeadwindCrosswind(240, 15, 243)    // wind 240/15, runway 24
```

## pkg/convert

| Group | Functions |
|---|---|
| Altitude, vertical speed | `FeetToMeters`, `MetersToFeet`, `FeetPerMinuteToFeetPerSecond`, `FeetPerSecondToFeetPerMinute` |
| Distance | `NMToMeters`, `MetersToNM`, `NMToKilometers`, `KilometersToNM`, `KilometersToMeters`, `MetersToKilometers`, `NMToStatuteMiles`, `StatuteMilesToNM`, `KilometersToStatuteMiles`, `StatuteMilesToKilometers`, `StatuteMilesToMeters`, `MetersToStatuteMiles` |
| Speed | `KnotsToKilometersPerHour`, `KilometersPerHourToKnots`, `KnotsToMetersPerSecond`, `MetersPerSecondToKnots`, `KnotsToFeetPerSecond`, `FeetPerSecondToKnots`, `FeetPerMinuteToMetersPerSecond`, `MetersPerSecondToFeetPerMinute`; Mach at sea-level ISA (661.4788 kt): `KnotsToMach`, `MachToKnots`, `KilometersPerHourToMach`, `MachToKilometersPerHour` |
| Pressure | `InHgToMillibar`, `MillibarToInHg`, `InHgToHectopascal`, `HectopascalToInHg`, `InHgToPascal`, `PascalToInHg` |
| Temperature | `CelsiusToFahrenheit`, `FahrenheitToCelsius`, `CelsiusToKelvin`, `KelvinToCelsius`, `FahrenheitToKelvin`, `KelvinToFahrenheit` |
| Weight, volume | `PoundsToKilograms`, `KilogramsToPounds`, `USGallonsToLiters`, `LitersToUSGallons` |
| Angles | `DegreesToRadians`, `RadiansToDegrees`, `NormalizeHeading` ([0, 360)), `NormalizeAngle` ((-180, 180]), `AngleDifference(from, to)` (signed, positive clockwise) |
| Local offsets | `OffsetToLatLon(latRef, lonRef, xEast, zNorth)` and `LatLonToOffset(latRef, lonRef, lat, lon)`: metres east/north of a reference point (WGS84), round-trip |
| ICAO codes | `IsICAOCode(code)` (a known prefix, not a registry check), `ICAORegion(code)` (by the first letter), `ICAOCountry(code)` (by the first two letters) |

```go
x, z := convert.LatLonToOffset(refLat, refLon, lat, lon) // metres east, north of the reference
lat2, lon2 := convert.OffsetToLatLon(refLat, refLon, x+50, z)
qnh := convert.InHgToHectopascal(29.92)
country := convert.ICAOCountry("LKPR")
```
