---
title: "Airport Layout & Taxi Routing"
description: "Load an airport's ground layout with pkg/airport and compute taxi routes between stands and runways."
order: 1
section: "airport"
---

# Airport Layout & Taxi Routing

`pkg/airport` turns SimConnect facility data into a typed model of an airport's ground layout (runways, parking stands, taxi points, taxi paths and taxiway names) and builds a routable taxi graph from it.

```go
import "github.com/mrlm-net/simconnect/pkg/airport"
```

| Type | What it is |
|------|-----------|
| `Loader` | Requests an airport's facility data and assembles it from your message loop |
| `Layout` | The decoded ground layout of one airport |
| `Graph` | The routable taxi network of a `Layout` |
| `Route` | A taxi route: points, length, taxiway names, runway crossings |
| `Cache` | Layouts by ICAO code, each with a lazily built `Graph` |

## Loading a layout

`Loader` never reads `engine.Client.Stream()` itself. You call `Request`, then hand it every message your loop receives; `Handle` reports the airport when all of its data has arrived. This lets it run alongside any other code that consumes the same stream, including `pkg/manager`.

```go
cache := airport.NewCache()
loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
if err := loader.Request("LKPR"); err != nil {
    return err
}

tick := time.NewTicker(time.Second)
defer tick.Stop()
for {
    select {
    case msg := <-client.Stream():
        if res, done := loader.Handle(msg); done {
            if res.Err != nil {
                return res.Err
            }
            fmt.Println(res.Layout.Name, len(res.Layout.TaxiPaths), "taxi paths")
        }
        // ... the rest of your message handling
    case now := <-tick.C:
        for _, res := range loader.Expire(now) {
            fmt.Println("timed out:", res.ICAO) // res.Err wraps airport.ErrTimeout
        }
    }
}
```

With `pkg/manager`, feed the loader from a message handler. The manager satisfies `airport.FacilityClient`:

```go
loader := airport.NewLoader(mgr, airport.LoaderWithCache(cache))
mgr.OnMessage(func(msg engine.Message) {
    if res, done := loader.Handle(msg); done {
        // res.Layout or res.Err
    }
})
```

### Loader details

| | |
|---|---|
| IDs | 6 facility definition IDs from `DefaultLoaderDefinitionBase` (7100) and 96 request IDs from `DefaultLoaderRequestBase` (7200). Move them with `LoaderWithIDs(defBase, reqBase)` if they clash with your own. |
| Concurrency | Up to 16 airports in flight at once. `Pending()` lists them. |
| Timeout | `LoaderWithTimeout` (default 30 s). SimConnect sends **nothing** for an unknown ICAO code, so an unknown airport ends in `ErrTimeout` via `Expire`. |
| Reconnect | Call `Reset(newClient)` so the definitions are registered again on the new connection. |
| Raw data | `Result.Raw` holds the decoded facility records (`RawAirport`); save it as JSON to replay later with `BuildLayout`. |

## The Layout model

```go
l := res.Layout

rwy, end, ok := l.RunwayEnd("24")         // also "6", "06", "RW27R"
fmt.Println(rwy.Name(), end.Heading, end.Threshold)

idx, err := l.ParkingIndex("C22")         // ErrUnknownParking / ErrAmbiguousParking
stand := l.Parking[idx]
fmt.Println(stand.Label(), stand.Type, stand.Radius, stand.IsGate())

for _, hs := range l.HoldShortPoints() {
    fmt.Println(hs.Index, hs.Position, hs.IsILSHoldShort())
}
```

Every slice is indexed by SimConnect list index (`TaxiPoints[i].Index == i`), and positions carry both the raw `BiasX`/`BiasZ` offsets and the resolved `LatLon`. Enumerations use the existing `pkg/types` facility enums (`SIMCONNECT_FACILITY_TAXI_PATH_TYPE`, `…_TAXI_POINT_TYPE`, `…_TAXI_PARKING_TYPE`, `…_TAXI_PARKING_NAME`, `…_RUNWAY_DESIGNATOR`).

Runways expose both ends (`Primary`, `Secondary`) with name, heading and threshold. Thresholds are the ends of the runway surface; displaced thresholds are not applied.

Parking labels combine `NAME`, `NUMBER` and `SUFFIX`: `GATE_C` + 22 → `C22`, `S_PARKING` + 22 + suffix `GATE_A` → `S22A`. Labels are usually but not guaranteed unique, which is why `ParkingByLabel` returns every match.

### Stands: size, conflicts, airlines

```go
for _, i := range l.SuitableStands(18) {   // RADIUS ≥ 18 m: an A320 (half span 17.9 m)
    p := l.Parking[i]
    fmt.Println(p.Label(), p.Size(), p.Airlines, l.ParkingConflicts(i))
}
heavy := l.SuitableStands(30, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY)
ok := l.Parking[heavy[0]].ServesAirline("DLH")
```

- **`Parking.Size()`** classes a spot as `StandSmall`, `StandMedium` or `StandHeavy`: by `TYPE` where the scenery names a size (`GATE_SMALL`/`MEDIUM`/`HEAVY`, `RAMP_GA_SMALL`/`MEDIUM`/`LARGE`), otherwise by `RADIUS` (below 15 m small, below 25 m medium). Fuel and vehicle spots are `StandNone`.
- **`Layout.SuitableStands(minRadius, types...)`** lists the spots with at least that `RADIUS` and, if given, one of the `TYPE`s; never fuel or vehicle spots. `RADIUS` is half the space the spot offers, so pass half the aircraft span plus a margin.
- **`Layout.ParkingConflicts(i)`** lists the spots whose `RADIUS` circles overlap spot `i` by more than half a meter: split and alternate stands (LKPR `S22`/`S22A`) and tightly packed gates. At LKPR 20 pairs overlap (two more only touch), at EDDM 4, at LROP 38. Whether two aircraft actually clash depends on their spans; the stand allocator (#292) decides that.
- **`Parking.Airlines`** are the airline codes the scenery assigns to the stand (`TAXI_PARKING_AIRLINE` child records; EDDM assigns 10–23 airlines to 119 of its 175 stands, LKPR none). `ServesAirline(code)` is true for a listed code (case-insensitive) and for stands without airlines.

### Facility data semantics

These were verified against MSFS 2024 data for LKPR (the fixture in `pkg/airport/testdata`) and differ from what older code in this repository assumed:

- **`TAXI_PATH.START` / `END` index the taxi point list, except for `PARKING` paths, whose `END` indexes the parking list.** At LKPR all 133 parking paths resolve that way (median 43 m); read as taxi points they would be 0.5–3.3 km phantom segments. `Layout.PathEndpoints` and `TaxiPath.EndsAtParking` apply this.
- **Hold-short points are identified by `TAXI_POINT.TYPE`**: `HOLD_SHORT` (2), `ILS_HOLD_SHORT` (4) and their `_NO_DRAW` variants (5, 6). LKPR uses only the `NO_DRAW` types. They sit on taxiways, never on runway paths.
- **Taxiways are `TAXI` (1) and `PATH` (4) paths**, and an airport may use only one of them: LKPR has no `TAXI` paths at all.
- **Parking `SUFFIX`** tells apart stands that share `NAME` and `NUMBER` (LKPR: `S22` and `S22A`).

## Taxi graph and routes

```go
g, err := airport.BuildGraph(l) // or cache.Graph("LKPR")

route, err := g.RouteToRunway(idx, "24", airport.RouteOptions{})
fmt.Printf("%.0f m via %v, crossing %v\n", route.Length, route.Taxiways, route.RunwayCrossings)
// 1595 m via [H1 H A], crossing []
```

The graph has one node per taxi point (node `i` = taxi point `i`) and one per parking spot (node `len(TaxiPoints)+k`).

| Path type | In the graph |
|---|---|
| `TAXI`, `PATH` | Taxiway edges |
| `PARKING` | Edge from its `START` taxi point to the parking spot at `END` |
| `RUNWAY` | Runway edges, used only with `RouteOptions.UseRunwayPaths` |
| `CLOSED`, `VEHICLE`, `ROAD`, `PAINTEDLINE` | Excluded |

Hold-short nodes are associated with the runway whose centreline is nearest (within 300 m), with `HoldShort.ILS`, the distance along the runway from the primary threshold and the offset from the centreline.

### Routing API

| Method | Route |
|---|---|
| `Route(from, to, opts)` | Shortest route between any two nodes |
| `RouteToRunway(parking, runwayEnd, opts)` | Stand → hold-short of a runway end (departure) |
| `RouteToParking(from, parking, opts)` | Any node → stand (taxi-in) |
| `RouteToRunwayEntry(parking, runwayEnd, entry, opts)` | Stand → hold-short of a runway end at a named entry: "24 at B" (empty entry = `RouteToRunway`) |
| `RouteFromRunway(exit, parking, opts)` | Runway exit → stand, continuing in the exit's direction |
| `RouteToRunwayFrom(from, prev, runwayEnd, entry, opts)` | From a node reached via `prev` (no turning back into it) → hold-short: a taxi-out after a pushback onto `prev` |
| `RouteFromNodes(nodes)` | A route along given adjacent nodes, e.g. a pushback joined to its taxi-out |

`Route.Cost` is what the search minimised (length plus turn, crossing and apron penalties), to compare alternatives. `RouteOptions.OwnApronMeters` (default 250 m) waives the apron penalty around the start: an aircraft leaving its own apron uses its taxilanes (LKPR C17 leaves by JB, the nearest), while through traffic still keeps off them.

**Aircraft size.** With `RouteOptions.HalfSpan` (half the wing span, meters) a route keeps to taxiway edges the aircraft fits:

- `Edge.Clearance` is the free half-width beside each taxiway edge: the distance from its centreline to the nearest stand circle (`Parking.Radius`, the space of the largest aircraft the stand takes). An edge fits with `WingtipMargin` (default 3 m) to spare. `OwnStands`, the stands the aircraft leaves or enters, are not obstacles; the stand-based routing functions add theirs.
- `TaxiwayMaxSpan` limits taxiways by name to a largest span, for published restrictions the scenery does not carry. It defaults to the airport's entry in `KnownTaxiwayMaxSpan` (a first seed of the airport limits, #335): LKPR's apron taxilanes JO and JB are code C (36 m), so a 777 leaves B14 by J while an A320 from C17 takes JB.
- When no route fits, the route is found without the size check and marked `Route.Tight`.

`pkg/traffic` controllers set `HalfSpan` from the aircraft's `MotionProfile`.

`RouteToRunway` prefers runway holding points over ILS holds, and among the hold-shorts within `RouteOptions.IntersectionTolerance` (default 300 m) of the one nearest the threshold, picks the shortest route, so aircraft depart from (or near) the full runway length.

A route never passes *through* a parking stand, and crossing a runway on a taxiway is allowed but reported in `RunwayCrossings`. Errors: `ErrNoTaxiNetwork`, `ErrUnknownParking`, `ErrAmbiguousParking`, `ErrUnknownRunway`, `ErrNoHoldShort`, `ErrNoRoute` (e.g. vehicle-only stands).

### Route cost: fewer turns, no crossings

Routes are not simply the shortest. Pilots and ATC prefer fewer and gentler turns even when a route is a little longer, so the search tracks which way the aircraft arrives at every node and adds a cost to the length:

| Cost | Default | `RouteOptions` field |
|---|---|---|
| Turn at a taxiway junction, per 90° above `TurnFreeAngle` (15°) | 60 m | `TurnPenalty` |
| Turning onto a differently named taxiway (unnamed connectors inherit the name; going straight on where the name changes is free) | 40 m | `TaxiwayChangePenalty` |
| Each runway crossing | 1000 m | `RunwayCrossingPenalty` |
| Taxiway edge running along a runway surface (e.g. crossing at a runway end, LROP) | ×20 its length | `UseRunwayPaths` removes it |
| Edge at a taxi point where a stand connects (apron taxilane), so through traffic keeps to taxiways without stands | +50 % of its length | `ApronPenalty` |
| Turning back (≥ 150°) | 2000 m | — |

Zero selects the default and a negative value disables a cost. `Route.Length` is always the real length.

### Runway entries and exits

`RunwayEntries("24")` lists the taxiways onto a runway end for departures, nearest the threshold first, with the runway remaining ahead of each (`Remaining`) and the turn onto the runway (`Angle`). `RunwayExits("24")` lists the exits for landings on it. Both leave out taxiways that meet the runway at more than `MaxExitAngle` (90°): they point back along the runway. An entry onto 24 is an exit for landings on 06 driven the other way.

```go
entries, _ := g.RunwayEntries("24") // A (3510 m ahead), B (2406 m), L (1549 m) at LKPR
route, err := g.RouteToRunwayEntry(idx, "24", "B", airport.RouteOptions{})
// 954 m via [H1 H JO G B]; route.Entry == "B"
```

An unknown entry name returns `ErrUnknownEntry`.

On LKPR, `BuildGraph` takes about 0.2 ms and a route a few milliseconds.

## GeoJSON

```go
b, err := l.GeoJSON()            // FeatureCollection: runway Polygons, taxiPath LineStrings,
                                 // parking and taxiPoint Points, raw fields in properties
f := route.Feature()             // a route as a LineString Feature
```

Coordinates are `[longitude, latitude]` per RFC 7946. Every feature has a `kind` property (`runway`, `taxiPath`, `parking`, `taxiPoint`, `route`).

## Procedures: SIDs, STARs, approaches

`ProcedureLoader` reads an airport's departures, arrivals and approaches with their runway, enroute and approach transitions and every leg (ARINC 424 leg type, fix and position, turn direction, course, distance, altitude and speed constraints, IAF/FAF/MAP). Feed it messages like the layout `Loader`:

```go
pl := airport.NewProcedureLoader(client)
pl.Request("LKPR")
for msg := range client.Stream() {
    if p, ok := pl.Handle(msg); ok {
        fmt.Println(len(p.Departures), "SIDs", len(p.Arrivals), "STARs", len(p.Approaches), "approaches")
    }
}
```

A SID is flown runway transition → `Legs` → enroute transition; a STAR enroute transition → `Legs` → runway transition. Courses are magnetic; `Procedures.MagVar` is the facility's MAGVAR (356 = 4° east, true = magnetic + 4).

`ProcedurePath(legs, start, startAlt, magVar, turnRadius)` turns legs into points for a map: straight between fixes, turns (Dubins, in the charted direction) where the aircraft turns by heading (a charted turn, a course intercept, a course reversal), open legs (to an altitude, DME distance, manual termination) approximated from the climb gradient. `Leg.Constraint()` prints a leg's constraint as charts do (`≥4000`, `FL070`, `≤210KT`).

To fly a procedure rather than draw it, resolve it into `NavPoint`s: one per fix (ident, kind, position, IAF/FAF/MAP, fly-over) plus a computed point where a leg without a fix ends (a climb to an altitude at 5%, a DME distance, an intercept of the next course, or a heading to radar vectors, `Vectors`). Each carries its altitude window in meters (`AltMin`/`AltMax`, 0 = none: AT sets both, at-or-above the minimum, at-or-below the maximum, between `Alt2`–`Alt1`), `SpeedMax` and the true `Course` flown to it; the same fix twice in a row (a STAR ending at the approach's IAF) is merged. `ResolveSID(name, runway, enroute, start, startAlt)`, `ResolveSTAR(name, enroute, runway)`, `ResolveApproach(name, transition)` and `MissedApproach(name)` return `ErrNoProcedure` or `ErrNoTransition` when a name is unknown. For ATC-style assignment, `SIDsFor`, `STARsFor` and `ApproachesFor` list a runway's procedures, `SIDToward(runway, exitFix)` and `STARFrom(runway, entryFix)` pick one by the flight plan's first or last fix, `BestApproach(runway)` prefers ILS, then RNAV, LOC, VOR, NDB, and `Arrival(runway, entryFix)` chains the STAR and the best approach through the transition where the STAR ends:

```go
sid, enroute, _ := p.SIDToward("24", "VOZ")
dep, _ := p.ResolveSID(sid.Name, "24", enroute, der, elevation) // VOZ4A: PR402 PR403 PR404 VOZ
arr, _ := p.Arrival("06", "GOLOP")
// GOLO4T + ILS 06 via KUVIX: GOLOP PR711 PR712 PR513 KUVIX PR741 PR742 CI06 FF06 RW06,
// at or above 1219 m (4000 ft) from PR741 to FF06, the threshold RW06 the MAP.
```

Not in the simulator's data: STAR altitude constraints at LKPR are empty (the AIP chart has them), and the `HOLDING_PATTERN` fields are rejected by MSFS 2024 — do not request them.

## Seeing it on a map

[`examples/airport-map`](../examples/airport-map) serves the layout on a Leaflet map with every feature's raw values, a route viewer and overlapping-stand highlighting. The route viewer has a departure mode (stand → runway, full length or at an entry) and an arrival mode (runway exit → stand, with the vacate stop and the stop point on the stand). Pick the entry or exit in the panel or click its marker on the map. Run it with `-dump` to save an airport's raw records, and with `-file` to view them without the simulator. The Procedures panel draws the SIDs, STARs and approaches of a runway as charts do: pick one from the list to see its fixes (VOR, NDB, waypoint symbols), constraints, tracks and distances, direction arrows, and where a STAR ends in radar vectors.

To drive an AI aircraft along a route, see [Departure Taxi](traffic-taxi.md).
