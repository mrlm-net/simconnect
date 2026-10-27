---
title: "Airspaces"
description: "pkg/airspace: the simulator's airspace outlines from a simconnect-data dump. Raw records, a Store that loads 10° tiles by box, and shapes with arcs and circles flattened for point-in-airspace tests."
order: 24
section: "packages"
---

# Airspaces

SimConnect has no airspace records. The simulator has them on its JavaScript side, behind a nearest search for boundaries. simconnect-data dumps them worldwide under `airspaces/`: 56,419 records in 10° tiles, plus an index. `pkg/airspace` reads that dump without a simulator, so it works offline and in WASM.

## Records

`Airspace` matches a tile's JSON field for field, as the simulator gave it:

| Field | Meaning |
|---|---|
| `ID` | Unique; `DataSet()` is `ID / 2^32` |
| `Name` | "RUZYNE CTR", "PRAGUE TMA III"; CTR, TMA and ATZ appear only in the name |
| `Type` | `BoundaryType`: `ClassB` 3, `ClassC` 4, `ClassD` 5, `ClassE` 6, `MOA` 14, `Restricted` 15, `Prohibited` 16, `Warning` 17, `Alert` 18; other values are kept as they come |
| `MinAlt`, `MaxAlt` | Metres, measured from `MinAltType` / `MaxAltType` (`AltMSL` 1, `AltAGL` 2, `AltUnknown` 0). Flight levels come as feet MSL |
| `TopLeft`, `BottomRight` | Bounding box (`Box()`) |
| `Designation` | Area number of restricted, prohibited and military areas |
| `Vectors` | The outline: `VectorStart`, `VectorLine`, `VectorOrigin` (a centre), `VectorArcCW` / `VectorArcCCW` (round the last origin to the point), `VectorCircle` (`Radius` metres round the origin before it) |

`Floor(groundM)` and `Ceiling(groundM)` give the limits in metres MSL; AGL limits are added to the ground elevation you pass. `Broken()` flags a record with no start or circle vector, or an unknown altitude type. There are 1,890 of them, in the newer data sets, mostly over Russia and the CIS.

## Store

```go
s, err := airspace.Open(os.DirFS(filepath.Join(dataDir, "airspaces")))
list, err := s.Area(airspace.BoxAround(50.1008, 14.26, 30000)) // within 30 km of LKPR
here, err := s.At(50.1008, 14.26)                              // outlines holding the point
```

`Open` reads `index.json`. `Area` picks the records whose box overlaps the given box, and decodes each tile once, the first time it is needed. A record box wider than 90° of longitude reaches the antimeridian (Magadan CTA, Bay CTA C, Nadi OCA/CTA at -180 to 180, Sankt-Peterburg CTA to 180), so it matches every longitude at its latitudes. A query box with `West > East` crosses 180.

The records are raw by default. Two options clean them up:

- `StoreSkipBroken()` leaves out the `Broken()` records.
- `StorePreferNewerSet()` leaves out a record when a newer data set has one of the same type and nearly the same box (all edges within 0.02°). Over Czechia every airspace comes twice, and the newer set follows the AIP. Elsewhere the two sets were not compared. Worldwide, this option keeps 35,063 of the 56,419 records.

## Shapes

`a.Shape()` flattens the outline into rings:

- lines stay as they are;
- arcs are flattened every 2°, with the radius taken from the origin's distance to the point before;
- circles become 72-sided polygons.

Where an outline crosses ±180, the longitudes run on past it. `Shape.Inside(lat, lon)` is the even-odd test over the rings. `Shape.Contains(lat, lon, altM, groundM)` also checks the floor and ceiling.

```go
sh, _ := ctr.Shape()
sh.Contains(50.1008, 14.26, 400, 380) // true: Ruzyne CTR runs from GND to 1,066.8 m MSL
```

The simulator's box does not always hold an arc's full sweep. POLICKA ATZ is a 345° clockwise arc, and its box stops at the end point's longitude. Use the shape, not the box, for anything but picking tiles.

What the traffic World will use airspaces for (sector boundaries, keeping traffic out of restricted areas) is not decided yet.
