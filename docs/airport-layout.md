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
| Frequencies | `Layout.Frequencies`: the airport's radio frequencies (kind, MHz, name) and `FrequencyFor(kind)` with ATC's fallbacks (#416). |
| IDs | 7 facility definition IDs from `DefaultLoaderDefinitionBase` (7100) and 112 request IDs from `DefaultLoaderRequestBase` (7200). Move them with `LoaderWithIDs(defBase, reqBase)` if they clash with your own. |
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

- **`Parking.Size()`** classes a spot as `StandSmall`, `StandMedium` or `StandHeavy`: by `TYPE` where the scenery names a size (`GATE_SMALL`/`MEDIUM`/`HEAVY`/`EXTRA`, `RAMP_GA_SMALL`/`MEDIUM`/`LARGE`/`EXTRA`; `EXTRA` is heavy), otherwise by `RADIUS` (below 15 m small, below 25 m medium). Fuel and vehicle spots are `StandNone`.
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
| `RouteToParkingFrom(from, prev, parking, opts)` | From a node reached via `prev` (-1: any heading) → stand: a taxi-in re-planned on its way |
| `RouteFromNodes(nodes)` | A route along given adjacent nodes, e.g. a pushback joined to its taxi-out |

`ExitFor(runwayEnd, rollout)` picks the first exit at least `rollout` meters past the landing threshold (the last exit when none is that far). `Fits(edge, opts)` checks one edge against the aircraft size as a search would, and `SpokenTaxiways(upto)` gives a route's taxiways as a controller says them, without stubs shorter than `SpokenMinMeters` (150 m) that only lead onto the next one.

`Route.Cost` is what the search minimised (length plus turn, crossing and apron penalties), to compare alternatives. `RouteOptions.OwnApronMeters` (default 250 m) waives the apron penalty around the start: an aircraft leaving its own apron uses its taxilanes (LKPR C17 leaves by JB, the nearest), while through traffic still keeps off them.

**Aircraft size.** With `RouteOptions.HalfSpan` (half the wing span, meters) a route keeps to taxiway edges the aircraft fits:

- `Edge.Clearance` is the free half-width beside each taxiway edge: the distance from its centreline to the nearest stand circle (`Parking.Radius`, the space of the largest aircraft the stand takes). An edge fits with `WingtipMargin` (default 3 m) to spare. `OwnStands`, the stands the aircraft leaves or enters, are not obstacles; the stand-based routing functions add theirs.
- `TaxiwayMaxSpan` limits taxiways by name to a largest span, for published restrictions the scenery does not carry. It defaults to the airport's entry in `KnownTaxiwayMaxSpan` (a first seed of the airport limits, #335): LKPR's apron taxilanes JO and JB are code C (36 m), so a 777 leaves B14 by J while an A320 from C17 takes JB.
- When no route fits, the route is found without the size check and marked `Route.Tight`. A [custom route](#custom-routes-via-points-and-taxiways) returns `ErrTooNarrow` instead.

`pkg/traffic` controllers set `HalfSpan` from the aircraft's `MotionProfile`.

**Aircraft in the way.** `RouteOptions.Occupied` lists places other aircraft take (`Occupied{Points, HalfSpan}`: a standing aircraft, or the path it pushes back along). The route keeps off taxiway edges that come within both half spans plus `WingtipMargin` of one. When no route does, it is found without them and marked `Route.Occupied`. `PassesOccupied(route, from, opts)` tells whether a route, from edge `from` on, comes too near any of them, e.g. to decide whether to re-plan.

**Fewer stands.** Between routes of about the same length, the one past fewer stands wins: a second search prices apron taxilanes at `FewerStandsApronPenalty` (4), and its route is taken when it passes fewer stands (`StandsPassed`) and is at most 15 % (`FewerStandsTolerance`, at least 150 m) longer. Not for custom routes.

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
| Entering a stand through a lead-in ahead of it (turning round on the apron), so arrivals park nose-in | 3000 m | `StandTurnAroundPenalty` |
| Leaving a stand through a lead-in behind it (a pushback), so departures leave forward where the stand allows | 200 m | `PushbackPenalty` |
| Named taxiway off a `Taxiways` list, until its last one is joined | ×10 its length | `OffTaxiwaysFactor` (constant) |
| Turning back (≥ 150°) | 2000 m | — |

Zero selects the default and a negative value disables a cost. `Route.Length` is always the real length.

### Custom routes: via points and taxiways

Two `RouteOptions` fields shape the route itself (#340). Every routing function honours them (`Route`, `RouteToRunway`, `RouteToRunwayEntry`, `RouteToRunwayFrom`, `RouteToParking`, `RouteToParkingFrom`, `RouteFromRunway`):

- `Via []NodeID`: the route passes these nodes in order. Every leg uses the same search and costs, and at a via point the route goes on the way it arrived. It never turns back there, so a via point at a dead end cannot be passed.
- `Taxiways []string` ("via F, L"): the names must appear in `Route.Taxiways` in this order, matched case-insensitively. Until the last one is joined, other named taxiways cost `OffTaxiwaysFactor` (×10) their length, so they serve only as connectors. After the last one the route goes on freely to its destination.

```go
route, err := g.RouteToRunway(c22, "30", airport.RouteOptions{Taxiways: []string{"F", "L"}})
// 2503 m via [F L] (the default is [H1 K L])

var re *airport.RouteError
_, err = g.RouteToRunway(b14, "24", airport.RouteOptions{Taxiways: []string{"JO"}, HalfSpan: 32.4})
if errors.As(err, &re) && errors.Is(err, airport.ErrTooNarrow) {
    fmt.Println("aircraft too big for", re.Taxiway) // JO
}
```

A custom route never skips the size check. Where only a route the aircraft does not fit would follow the request, the error is `ErrTooNarrow` and the route is not returned `Tight`. Failures come as a `*RouteError` naming the via point (`Via` index and `Node`) or the taxiway (`Taxiway`), so a UI can say "does not connect" or "too big for JO":

| Error | Meaning |
|---|---|
| `ErrViaUnreachable` | Via point `Via` cannot be reached in order without turning back at the previous one (or the node does not exist) |
| `ErrNoRoute` with `Via == len(opts.Via)` | The destination cannot be reached from the last via point |
| `ErrTaxiwaysNotFollowed` | No route follows the taxiways in order; `Taxiway` is the first one it cannot reach |
| `ErrTooNarrow` | Only a route the aircraft does not fit would work; `Taxiway` is where it does not fit |
| `ErrUnknownTaxiway` | The airport has no taxiway of that name (`TaxiwayNames()` lists them) |

All except `ErrUnknownTaxiway` wrap `ErrNoRoute`. These errors appear only when the request is at fault: a destination that cannot be reached at all returns the plain error. `ValidateRouteOptions(opts)` checks via nodes and taxiway names before routing. `RemainingOptions(opts, walked)` drops the via points and taxiways that a walk (a pushback, a runway exit) already passed, so a search can continue from the walk's end.

### Runway entries and exits

`RunwayEntries("24")` lists the taxiways onto a runway end for departures, nearest the threshold first (entries as far from it in taxiway name order, so the list is the same every time), with the runway remaining ahead of each (`Remaining`) and the turn onto the runway (`Angle`). `RunwayExits("24")` lists the exits for landings on it. An entry onto 24 is an exit for landings on 06 driven the other way. Exits turning more than `MaxExitAngle` (90°) point back along the runway and are left out. A departure at taxi speed turns sharper: entries may turn up to `MaxEntryAngle` (135°), because threshold entries often meet the runway square or slightly back (LKPR 12 at L, 120°).

Scenery data sometimes draws a taxiway ending on another node without sharing it: at LKPR the F lead-in ends on the 06 centreline beside the runway node. `BuildGraph` joins a dead end to another node within 3 m, so the lead-in reaches the runway.

```go
entries, _ := g.RunwayEntries("24") // A and Z (3510 m ahead), B (2406 m), L (1549 m) at LKPR
route, err := g.RouteToRunwayEntry(idx, "24", "B", airport.RouteOptions{})
// 965 m via [H1 H JO J D B]; route.Entry == "B"
```

An unknown entry name returns `ErrUnknownEntry`. When several entries share a name, the reachable one with the most runway ahead wins.

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

### The flown route

`FlownRouteFor(FlownRequest)` lays out a whole flight as one line, for a map, a flight page or a copilot's navigation. You give it both layouts, the departure and arrival runway ends, both airports' `Procedures`, the filed enroute fixes (`RouteFix`: ident, position, optional altitude) and, optionally, the SID, STAR and approach (with transitions) ATC cleared. It returns `FlownRoute`:

- `Points`: one line from the runway to the threshold. Each fix carries its ident, `LegConstraint` (feet and knots), fly-over flag and phase (`sid`, `enroute`, `star`, `approach`). Points between fixes are turns and course legs drawn as `ProcedurePath` draws them.
- `Missed`: the missed approach, as its own line from where the final ends.
- The procedures taken: `SID`, `STAR`, `Approach` (`"synthetic"` for a made-up final), and their transitions.

How each part is chosen:

1. **Departure.** The SID is the one cleared, else the one ending at a route fix (the latest along the route). It is flown as its runway transition, common legs and enroute transition, and joins the route at its last fix. Without a SID the aircraft flies the runway heading for 5 NM (`FlownClimbOutNM`), then turns toward the first fix, so there is no hairpin back over the runway.
2. **Arrival.** The STAR is the one cleared, else the one entered at a route fix (the latest along the route). The route is cut where the STAR is entered.
3. **Approach.** The approach is the one cleared, else `BestApproach` (ILS, then RNAV, then LOC, VOR, NDB). Its transition is the one cleared, else the one starting at the STAR's last fix, else the final from that fix when the final passes it, else the transition whose first fix is nearest. The STAR's closing vectors (heading legs after its last fix) are left out when an approach takes over there.
4. **No approach.** A synthetic final joins the extended centreline 10 NM out (`FlownFinalNM`) on a 3° path. A base leg comes first when the intercept would be more than 90°.

The points are not smoothed. `FlownRoute.Smoothed(radius)` rounds the fly-by corners (`SmoothPath`) but keeps fly-over fixes and reversals of 150° or more, so a turn never cuts a fly-over fix or loops.

Not in the simulator's data: STAR altitude constraints at LKPR are empty (the AIP chart has them), and the `HOLDING_PATTERN` fields are rejected by MSFS 2024 — do not request them. `pkg/traffic` builds its own holds on STAR fixes instead ([Holding](traffic-separation.md#holding)).

## Airport limits

`LimitsFor(layout, procedures)` returns the values that belong to the airport rather than to the aircraft (#335), taken from the facility data where it has them and from `KnownLimits` (published values, keyed by ICAO code) otherwise:

| Field | Source | Default |
|-------|--------|---------|
| `TransitionAltitudeFt` | `KnownLimits` (LKPR 5000, EDDF/EDDM 5000, LOWW 10000, EGLL 6000, LFPG 5000, EHAM 3000, EPWA 6500, LSZH 7000), else the facility's `TRANSITION_ALTITUDE` | 5000; 18000 for K… and C… codes |
| `ClimbHandoverFt` (above the field) | the SIDs' initial climb: the highest CA/VA/FA leg starting a runway transition, minus the elevation, at least `MinClimbHandoverFt` (1500) | `DefaultClimbHandoverFt` (1500) without procedures |
| `InitialClimbFt`, `InitialClimbs` (per SID) | `KnownLimits` (EDDM 7000, LOWW 5000, EGLL 6000); `InitialClimbFor(sid)` picks | `DefaultInitialClimbFt` (FL100) |
| `TaxiMaxKts` / `ApronMaxKts` | `KnownLimits` | 30 / 15 |
| `PreferredRunways` | `KnownLimits` (LKPR 24, then 06; EGLL 27R, 27L) | none |
| `MSAFt`, `NoReverseThrust` | `KnownLimits` (no reverse: EDDM, LOWW) | unknown / false |
| `Tower` | `KnownLimits` (LKPR) | nil: the facility's tower position |
| `DeicingPads`, `Tugs`, `FuelTrucks`, `Stairs`, `GPUs`, `Buses`, `FollowMe` | `KnownLimits` | none / 0: sized by the stands |

At LKPR the SIDs climb to 1700 ft on the runway heading first; the field is at about 1200 ft, so the hand-over stays at the 1500 ft floor. Pass the limits to `traffic.TaxiRequest.Airport` / `ArrivalRequest.Airport`, and `nav.RunwayLimitsFrom(lim)` gives the preferential runways to `nav.ActiveRunways` (or to a `nav.RunwaySelector`, which keeps the runway in use, see [Keeping the runway in use](nav-weather.md#keeping-the-runway-in-use)). `Graph.Apron(node)` reports a stand's junction with the taxilane, where `ApronMaxKts` applies.

```go
lim := airport.LimitsFor(layout, &procs) // LKPR: TA 5000, hand-over 1500 ft, 24 then 06
use := nav.ActiveRunways(layout, weather, nav.RunwayLimitsFrom(lim))
```

## Which airport an aircraft is at

`Locate(LocateQuery, layouts)` names the airport a position is at among the airports around it (load those within about 15 km). It reports the airport, what of it the position is on (`OnRunway`, `OnTaxiway`, `AtParking`, `OnApproach`, `OnDeparture`, `NearAirport`), that runway, runway end, stand or taxiway, and the distance.

- **On the ground** the airport whose surface is nearest wins: its runway rectangles, taxiway segments with their width, and parking spots with their radius. An airport without any geometry is its reference point. Most of the simulator's idents are like that: 603 of 693 around the test airports, mostly heliports and private strips. Where two airports share a surface, the larger wins, then a four-letter ICAO code, and the other is in `Alternatives`. Two cases do this: aliases (196 four-letter airports have another ident at the very same spot, such as EBBR/EBMB or PAAW/KEB) and a strip inside a larger field.
- **In the air** it picks the runway end whose approach corridor (10 NM, 150 m either side widening at 10°) or departure corridor (5 NM, widening at 15°) the position is in. The height fits a 3° glide path or a 7° climb. `Track` and `VerticalFpm` tell an approach from a departure on the same line. `RunwayMeters` (the runway the aircraft needs) leaves out the corridors of shorter runways. Off every corridor it is near the nearest surface, within `LocateNearMeters` (5 km). Beyond that it is at no airport.

`Tracker` follows one aircraft through whole flights. It locates on the ground and remembers where the aircraft took off, then keeps to that airport's departure while it climbs out. On an approach it keeps to its destination (`SetDestination`) or to the approach it was already flying. Where two fields' corridors overlap, one position alone cannot tell them apart; the flight can.

`tools/locate-eval` measures both on facility dumps. The sample: the ten test airports and 60 crowded spots from the simulator's world list, each with every airport within 15 km. It samples every stand and taxi node, runway centrelines and edges, the same 25 m off, approaches and departures, and whole flights through a `Tracker`. The plain nearest-reference-point method is shown next to it:

| | Locate / Tracker | Nearest reference point |
|---|---|---|
| Stands, taxi nodes, runways (79,000) | 99.99–100% | 91–92% |
| Approaches, with track and vertical speed | 99.7% | 33% |
| Departures, with track and vertical speed | 99.1% | 42% |
| Flights: departures, approaches to the destination | 100% | 31–42% |
| Flights: approaches with no destination | 99.65% | 31% |

The remaining misses are distinct strips on one line or crossing each other's corridors, such as El Vergel and Los Gavanes, 1.8 km apart on the same runway line.

## Seeing it on a map

[`cmd/airport-map`](../cmd/airport-map) serves the layout on a Leaflet map with every feature's raw values and overlapping-stand highlighting. Setting up a new flight previews its route: a departure from stand to runway (full length or at an entry), an arrival from the runway exit to the stand with the vacate stop, and via points picked on the map. Run it with `-dump` to save an airport's raw records, and with `-file` to view them without the simulator. The Procedures panel draws the SIDs, STARs and approaches of a runway as charts do: pick one from the list to see its fixes (VOR, NDB, waypoint symbols), constraints, tracks and distances, direction arrows, and where a STAR ends in radar vectors.

The sidebar has one tab per task: **Traffic** (new flights, aircraft cards with their clearances, the [ATC game](atc-game.md)), **Sequence**, **Schedule**, **Radio**, **Airport** (ATIS, tower, weather, airport info, procedures, VFR circuits and reporting points, de-icing pads) and **Map** (base map, panel position, layers such as live traffic, safe zones, overlapping and occupied stands). A Quick reference button opens a help dialog. Map buttons show the whole airport, your aircraft or the world view, open the layers, go full screen with the panel, and hide or show the panel.

To drive an AI aircraft along a route, see [Departure Taxi](traffic-taxi.md). The map's scheduled traffic, world view and landing sequence are described in [Traffic Manager](traffic-manager.md#on-the-airport-map), [Traffic Picture](traffic-picture.md#on-the-airport-map) and [Airborne Separation](traffic-separation.md#the-landing-sequence).
