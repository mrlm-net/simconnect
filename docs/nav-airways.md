---
title: "Airways, Waypoints & Navaids"
description: "Crawl the enroute airway network from the facility API with pkg/nav, cache it as JSON and route between fixes."
order: 1
section: "nav"
---

# Airways, Waypoints & Navaids

`pkg/nav` reads enroute navigation data — waypoints, VORs, NDBs and the airways joining them — from the SimConnect facility API, builds a routable airway graph and caches it as JSON so later runs (and tests) need no simulator.

```go
import "github.com/mrlm-net/simconnect/pkg/nav"
```

| Type | What it is |
|------|-----------|
| `FixKey` | A fix's identity: ident + ICAO region + kind (`W` waypoint, `V` VOR, `N` NDB) |
| `Fix` | A fix: position, waypoint type, and for navaids frequency and name |
| `NavLoader` | Requests fixes and assembles them (with their airway links) from your message loop |
| `AirwayCrawler` | Follows airways breadth first from seed fixes within a radius |
| `AirwayGraph` | Fixes as nodes, airway segments as edges; `Route`, `DirectTo`, JSON cache |

## Facility data

A fix is requested with `RequestFacilityDataEX1(def, req, ident, region, kind)`: identifiers repeat between waypoints, VORs and NDBs, and the kind byte (`'W'`, `'V'`, `'N'`) picks one. The loader registers three definitions, using only fields the SDK lists for each record:

| Record | Fields |
|--------|--------|
| `WAYPOINT` (40 bytes + 4 for `IS_TERMINAL_WPT`) | `LATITUDE`, `LONGITUDE` (FLOAT64), `TYPE`, `ICAO`, `REGION` (STRING8), `N_ROUTES`, `IS_TERMINAL_WPT` |
| `ROUTE` child (116 bytes) | `NAME` (STRING32), `TYPE`, then `NEXT_` and `PREV_` `ICAO`, `REGION`, `TYPE`, `LATITUDE`, `LONGITUDE`, `ALTITUDE` (FLOAT32) |
| `VOR` | `VOR_LATITUDE`, `VOR_LONGITUDE`, `FREQUENCY` (UINT32 Hz), `NAME` (STRING64) |
| `NDB` | `LATITUDE`, `LONGITUDE`, `FREQUENCY` (UINT32 Hz), `NAME` (STRING64) |

A VOR or NDB is requested twice: as a `WAYPOINT` (for its airways) and as a navaid (for frequency and name). Findings from MSFS 2024:

- Each `ROUTE` record is one airway through the fix with its previous and next fix; at an airway's end one side is empty. The data has no one-way flag, so every segment is routable both ways.
- An airway identifier can be several disjoint pieces: M725 runs HDO – RAVKU – GOLOP and, separately, VOZ – TABEM – OKF, broken around the Prague TMA.
- A navaid off the airway network (OKL, the Prague VOR) has no `WAYPOINT` record: the waypoint request raises exception 1 (`ERROR`) and sends no `FACILITY_DATA_END`. The loader matches the exception to its request through `GetLastSentPacketID`; `Expire` ends anything else left unanswered.
- `NEXT_ALTITUDE`/`PREV_ALTITUDE` are minimum altitudes in meters of round feet (1524 = 5000 ft); Czech segments mostly give 0.
- Airway `TYPE` is 1 victor, 2 jet, 3 both — in the LKPR area all three occur.
- A wrong region is tolerated: `LAGAR` requested in `ED` returns the `LK` fix. The loaded fix keeps the region the simulator returned.

## Loading fixes

Like `airport.Loader`, `NavLoader` never reads the stream itself: call `Request`, hand it every message, and it returns each fix when complete. Both `engine.Client` and `manager.Manager` satisfy `nav.FacilityClient`.

```go
loader := nav.NewNavLoader(client) // 16 fixes in flight, IDs 8700+/8800+
loader.Request(nav.Key("VOZ", "LK", nav.KindVOR))
for msg := range client.Stream() {
    if res, ok := loader.Handle(msg); ok {
        fmt.Println(res.Fix.Name, res.Fix.Freq) // VOZICE 116.95
        for _, r := range res.Routes {
            fmt.Println(r.Airway, r.Prev, r.Next)
        }
        break
    }
}
```

## Crawling the airway network

`AirwayCrawler` starts from seed fixes, and every fix it loads names its neighbours on each airway; those within `RadiusNM` of `Center` are loaded next, until nothing is left or `MaxRequests` is reached. Neighbours beyond the radius stay in the graph as airway ends, known by key and position.

```go
c := nav.NewAirwayCrawler(nav.NewNavLoader(client), nav.CrawlOptions{
    Center:      airport.LatLon{Lat: 50.1008, Lon: 14.26}, // LKPR
    RadiusNM:    250,
    MaxRequests: 3000,
})
c.Start(nav.Key("VOZ", "LK", nav.KindVOR), nav.Key("GOLOP", "LK", nav.KindWaypoint))
tick := time.NewTicker(time.Second)
for done := false; !done; {
    select {
    case msg := <-client.Stream():
        done, _ = c.Handle(msg)
    case now := <-tick.C:
        done, _ = c.Tick(now) // expires unanswered requests
    }
}
g := c.Graph()
g.SaveJSON("LKPR-airways.json")
```

Around LKPR (250 NM, seeded with the enroute ends of the SIDs and STARs) the crawl takes 1080 requests and under two seconds: 1221 fixes (41 VORs, 7 NDBs), 441 airways, 2010 segments. `examples/spike-airways` runs it (`-radius`, `-max`, `-seeds`, `-out`); with `-raw 1..4` it dumps the raw records instead.

## Routing

```go
g, _ := nav.LoadAirwayGraph("testdata/LKPR-airways.json")
steps, err := g.Route(nav.Key("VOZ", "LK", nav.KindVOR), nav.Key("OKF", "LK", nav.KindVOR))
fmt.Println(nav.FormatRoute(steps)) // VOZ M725 OKF
```

`Route` is A* over (fix, airway) states by great-circle distance, adding `DefaultAirwayChangePenaltyNM` (10 NM) at every change of airway; `RouteWithPenalty` takes another value. It returns `ErrNoRoute` when the fixes are not connected. Each `RouteStep` names the airway flown to reach its fix (empty for the first step).

The network can make long detours between nearby fixes — VOZ to GOLOP is 65 NM direct but 331 NM by airway, around the Prague TMA. `RouteOrDirect(from, to, maxStretch)` falls back to a single `DCT` step when there is no route or the route is longer than `maxStretch` times the direct distance; `DirectTo` gives the direct leg on its own. `Nearest(pos)` finds the closest fix on an airway, to join the network from an airport.
