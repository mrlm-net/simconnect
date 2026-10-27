---
title: "Airspaces"
description: "pkg/airspace: the simulator's airspace outlines from a simconnect-data dump. Raw records, a Store that loads 10° tiles by box, shapes with arcs and circles flattened, the class at a point, a route check, and the SERA VFR minima in pkg/traffic."
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
- `StorePreferNewerSet()` leaves out a record when a newer data set has one of the same type and nearly the same box (all edges within 0.02°, or within 0.1° when the names match too). Over Czechia every airspace comes twice, and the newer set follows the AIP: PRAGUE CTA 2 (E) runs from 1,000 ft MSL in the older set and from 1,000 ft AGL in the newer, with boxes 0.04° apart. Elsewhere the two sets were not compared. Worldwide, this option keeps 34,964 of the 56,419 records.

`Area` finds the records through a 1° grid of the index, built on first use; boxes wider than 400 cells are checked on every query.

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

`s.Shape(a)` is the same, built once per record and kept by the store. Outlines run to 10,000 vectors (Riga CTA, Vilnius CTA/UTA).

## Class at a point

```go
s, _ := airspace.Open(fsys, airspace.StorePreferNewerSet(), airspace.StoreSkipBroken())
in, _ := s.Containing(lat, lon, altM, groundM) // every record holding the point, floor to ceiling
cl, _ := s.ClassAt(lat, lon, altM, groundM)    // the class record (B–E), nil where none
```

`ClassAt` returns the strictest class where records overlap. Over LKPR at 3,000 ft that is RUZYNE CTR (D) over the Prague CTA's E. MOA, restricted, prohibited, warning and alert areas are not classes, so `ClassAt` ignores them; the route check below lists them.

- **No class A or G records.** Where `ClassAt` returns nil, the class is the caller's to decide. G is right for most of Europe at low levels. It is wrong above FL180 in the US (class A) and above FL195 under SERA.6001(b) (class C), wherever no record covers that airspace.
- **AGL limits** need the ground elevation under the point. Pass 0 when it is not known, and AGL limits are then taken over sea level.
- **Records with both limits 0** (2,020, of which 1,890 are `Broken()`) are skipped (`NoLimits`), not treated as surface to unlimited.
- **Conditional activation is not in the data** (Prague TMA VIII, for example). The class is wrong there whenever the real airspace is not active.

### In the traffic World

`world.Options.Airspaces` takes a store. With it, the World takes an aircraft's class from the records (G where none covers it) for who is separated from whom (#570), and `Options.Airspace` is not used. For the ground under an aircraft the World uses its height above ground; without one in the air, it uses the nearest managed airport's field elevation. Without the store, the World keeps the old stand-in: a managed airport's zone of `Options.Airspace` out to 10 NM and up to 5,000 ft above the field, and E everywhere else.

## Route check

```go
path := []airspace.PathPoint{{Lat: 49.85, Lon: 14.26, AltM: 610, GroundM: 365}, {Lat: 50.10, Lon: 14.26, AltM: 610, GroundM: 365}}
entries, _ := s.Route(path)          // RUZYNE CTR, D, clearance, 15.4 km along
look := airspace.Ahead(here, trk, gs, 5*time.Minute) // no plan: a straight line ahead
```

`Route` samples the path every 250 m (`RouteStep`) and lists each airspace the path enters, in order. An entry is made each time a sample is inside an airspace, floor to ceiling, and the sample before it was not. An airspace the path starts in is listed with `Start` set. Each `Entry` carries the record's id, name, type and designation, the first point inside and the distance along the path to it, the floor and ceiling over that point's ground, and what the airspace needs from a VFR flight:

| Type | `Need` |
|---|---|
| B, C, D | `NeedClearance` (SERA.6001(a)(2)–(4)) |
| E | `NeedNone` (SERA.6001(a)(5)) |
| restricted, prohibited | `NeedAvoid` |
| MOA, warning, alert | `NeedInform` |
| anything else | `NeedUnknown` |

`RouteNeeds` replaces these per type, for countries where the meaning differs: in the US, class C needs only two-way radio contact. The data has no activation times, so `NeedAvoid` counts every restricted area as always active, which is over-cautious. `Frequency` is filled when a record has one, and no record dumped so far does. The entries are data only, with no wording; any callout for the player belongs to the app.

## VFR minima

`pkg/traffic` has the SERA VFR weather minima (Regulation (EU) No 923/2012, consolidated text of 26.01.2023):

- `VFRMinima(class, altFt, aglFt, ...)` gives the minima from SERA.5001, Table S5-1. At and above 10,000 ft AMSL every class needs 8 km of flight visibility and 1,500 m horizontal / 1,000 ft vertical distance from cloud. Below 10,000 ft it is 5 km with the same distances. Classes F and G at or below 3,000 ft AMSL, or 1,000 ft above terrain if that is higher, need 5 km, clear of cloud and with the surface in sight. Two options cover the table's footnotes: `VFRFlightLevel` makes FL100 the band limit where the transition altitude is below 10,000 ft (footnote 1), and `VFRReducedVisibility` sets the authority's lower visibility for F and G, never under 1,500 m (footnote 3).
- `CheckControlZone(groundVisM, ceilingFt, helicopter)` checks an aerodrome in a control zone. Ordinary VFR needs 5 km ground visibility and a 1,500 ft ceiling (SERA.5005(b)). Below that, special VFR can be cleared down to 1,500 m (800 m for helicopters) and a 600 ft ceiling (SERA.5010(c)). `SpecialVFRMinima` gives the special VFR flight's own minima from SERA.5010(b).
- SimConnect (2024) gives no cloud base and no ceiling. `VMCMinima.Check` checks visibility only and always sets `CloudUnknown`. `CheckControlZone` sets `CeilingUnknown` when it has no ceiling. Neither ever makes a ceiling up.
