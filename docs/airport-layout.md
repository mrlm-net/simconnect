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

`RouteToRunway` prefers runway holding points over ILS holds, and among the hold-shorts within `RouteOptions.IntersectionTolerance` (default 300 m) of the one nearest the threshold, picks the shortest route, so aircraft depart from (or near) the full runway length.

A route never passes *through* a parking stand, and crossing a runway on a taxiway is allowed but reported in `RunwayCrossings`. Errors: `ErrNoTaxiNetwork`, `ErrUnknownParking`, `ErrAmbiguousParking`, `ErrUnknownRunway`, `ErrNoHoldShort`, `ErrNoRoute` (e.g. vehicle-only stands).

On LKPR, `BuildGraph` takes about 0.2 ms and `RouteToRunway` about 0.2 ms.

## GeoJSON

```go
b, err := l.GeoJSON()            // FeatureCollection: runway Polygons, taxiPath LineStrings,
                                 // parking and taxiPoint Points, raw fields in properties
f := route.Feature()             // a route as a LineString Feature
```

Coordinates are `[longitude, latitude]` per RFC 7946. Every feature has a `kind` property (`runway`, `taxiPath`, `parking`, `taxiPoint`, `route`).

## Seeing it on a map

[`examples/airport-map`](../examples/airport-map) serves the layout on a Leaflet map with every feature's raw values, a departure route viewer and overlapping-stand highlighting. Run it with `-dump` to save an airport's raw records, and with `-file` to view them without the simulator.

To drive an AI aircraft along a route, see [Departure Taxi](traffic-taxi.md).
