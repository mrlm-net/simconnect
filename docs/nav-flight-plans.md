---
title: "Flight Plans"
description: "Plan an IFR flight with pkg/nav: runways in use, SID, airways, STAR and approach, cruise level, time and fuel, and write it as an MSFS .pln file."
order: 3
section: "nav"
---

# Flight Plans

`nav.Plan` builds an IFR flight plan from what the rest of the library already knows: the airports' layouts and procedures (`pkg/airport`), the weather and runway in use, and the airway graph. The plan can be printed, saved as JSON, or written as an MSFS `.pln` file.

```go
import "github.com/mrlm-net/simconnect/pkg/nav"
```

| Type / function | What it is |
|-----------------|-----------|
| `FlightPlanRequest` | Departure and arrival (`AirportInfo`), aircraft type, cruise level, runways, weather, runway limits |
| `AirportInfo` | An airport: `Layout` and `Procedures`, or only `Position` and `ElevationM` |
| `Plan(req, graph)` | Plans the flight; `graph` may be nil (all direct); `ErrNoAirport` when an airport has no position |
| `FlightPlan` | Runways, SID/STAR/approach, ICAO route, waypoints, cruise level, distance, TOC/TOD, ETE, fuel |
| `Waypoint` | One point: ident, region, kind, position, airway, phase, constraints, planned altitude, distance |
| `FlightPlan.PLN()` | The MSFS `.pln` (AceXML) file |
| `CruiseLevel` | Semicircular-rule cruise level, capped by the type and the distance |
| `PerformanceFor` | Planning data of an ICAO type (speeds, rates, fuel flow, taxi fuel) |
| `FormatLLA` | A position in the `.pln` coordinate format |

## Planning a flight

```go
wx := nav.StaticWeather(240, 10, 9999, 15, 5, 1013) // or from nav.WeatherReader
fp, err := nav.Plan(nav.FlightPlanRequest{
    Departure:  nav.AirportInfo{ICAO: "LKPR", Layout: lkpr, Procedures: lkprProcs},
    Arrival:    nav.AirportInfo{ICAO: "LOWW", Layout: loww, Procedures: lowwProcs},
    Type:       "A20N",
    DepWeather: &wx,
    DepLimits:  nav.RunwayLimits{Preferred: []string{"24", "06"}},
}, graph)
if err != nil {
    return err
}
fmt.Print(fp)
pln, _ := fp.PLN()
os.WriteFile("LKPRLOWW.pln", pln, 0o644)
```

Layouts come from `airport.Loader`, procedures from `airport.ProcedureLoader`, the graph from `nav.LoadAirwayGraph` (a crawl saved by `examples/spike-airways`). The `examples/flight-plan` command loads everything from the simulator:

```bash
go run ./examples/flight-plan -from LKPR -to LOWW -type A20N -out LKPRLOWW.pln
```

```
LKPR/24 → LOWW/16  A20N  FL270  160 NM  ETE 0:30  track 138°M
route: VOZ5M VOZ M725 LANUX LANU7W
approach: ILS 16
fuel kg: taxi 150  trip 1330  contingency 67  alternate 0  reserve 1100  block 2647
  RW24     SID                   0.0 NM   1175 ft
  *        SID       VOZ5M       3.7 NM   2813 ft  ≥1700
  PR411    SID       VOZ5M      11.2 NM   6100 ft
  PR412    SID       VOZ5M      34.9 NM  16511 ft
  VOZ      SID       VOZ5M      47.3 NM  21992 ft
  TOC      ENROUTE              58.7 NM  27000 ft
  TABEM    ENROUTE   M725       69.3 NM  27000 ft
  TOD      ENROUTE              88.8 NM  27000 ft
  OKF      ENROUTE   M725       90.1 NM  26491 ft
  LANUX    STAR      M725       95.7 NM  15000 ft  6000–15000
  NERDU    STAR      LANU7W    126.8 NM  12847 ft  ≥6000
  WW671    APPROACH  ILS 16    143.0 NM   6798 ft  ≥5000
  FI16     APPROACH  ILS 16    145.1 NM   6039 ft  ≥5000
  RW16     APPROACH  ILS 16    159.7 NM    647 ft  600
```

## How the plan is made

1. **Runways.** `DepartureRunway` / `ArrivalRunway` when set, else `ActiveRunways` for the airport's weather (`DepWeather`, `ArrWeather`) and limits (`DepLimits`, `ArrLimits`). Without weather the wind is calm, so the longest runway (or the first preferred one) wins.
2. **Procedures.** Every SID from the departure runway (with each enroute transition) is resolved from the departure end of the runway, and every STAR to the arrival runway is joined to the best approach (`airport.Procedures.BestApproach`: ILS, then RNAV…) through the approach transition that starts where the STAR ends. Plan takes the pair with the shortest path: SID + great circle from the SID's last fix to the STAR's first + STAR and approach. Without STARs, the approach is entered through the transition that fits best, or at its final.
3. **Enroute.** Between the SID and the STAR the route follows the airway graph (`RouteOrDirect`); when the airways are more than `EnrouteMaxStretch` (1.5) times the direct distance — common around a TMA — it flies direct. An airport without procedures joins the airways at the nearest fix within 50 NM toward the other airport, direct from its runway threshold (or its position without a layout).
4. **Cruise level.** `CruiseLevel` applies the semicircular rule (RVSM) to the magnetic track between the airports: odd levels (FL350, FL370) on 000–179°, even (FL340, FL360) on 180–359°. It takes the highest such level up to the type's `MaxFL` at which the climb and descent (from the rates and speeds in `Performance`) leave at least 15% of the distance level, and at least 2000 ft above the higher airport. Set `CruiseFL` to force a level.
5. **Profile.** Every waypoint gets its distance from the start and a planned altitude: a straight climb to the top of climb, level, a straight descent from the top of descent, kept within the waypoint's constraints. `TOC` and `TOD` are inserted as waypoints of kind `PointProfile`.
6. **Time and fuel.** ETE is the climb, cruise and descent at the type's speeds, without wind. Trip fuel is the cruise fuel flow over that time plus half as much again during the climb; contingency is 5% of it, the final reserve 30 minutes at the cruise flow, the alternate `AlternateFuelKg`, and the block adds taxi fuel.

The magnetic variation comes from the departure's procedures (`Procedures.MagVar`), else the arrival's, else it is 0.

## Waypoints

`FlightPlan.Waypoints` runs from the departure runway threshold (or airport) to the arrival threshold (or airport). Each point has a `Phase` (`SID`, `ENROUTE`, `STAR`, `APPROACH`) and the `Airway` it is reached by: an airway name, `DCT`, the SID or STAR name, or the approach name. Constraints are in feet (`AltMinFt`, `AltMaxFt`, both set and equal for an "at") and knots (`SpeedMaxKts`); `Constraint()` formats them as charts do. Computed points of the procedures (the end of a climb, a heading to radar vectors) have no ident.

`PositionAt(distNM)` gives the position, planned altitude and track at a distance along the plan. The [Traffic Manager](traffic-manager.md#appearing-airborne) uses it to make an aircraft appear en route, where its flight would be by then.

`Route` is the ICAO route (item 15): the SID and its last fix, then airway and fix at each change of airway, the STAR's first fix and the STAR — `VOZ5M VOZ M725 LANUX LANU7W`, or `DCT FOLWU L984 SULUS Z650 TONSU Z35 LOMKI LOMK8T` from an airport without procedures.

## The .pln file

`PLN()` writes the MSFS flight plan format (`SimBase.Document`, AceXML version 1,1):

- header: `Title`, `FPType` IFR, `RouteType` (HighAlt from FL180, else LowAlt), `CruisingAlt`, departure and destination ID, position (`DepartureLLA`, `DestinationLLA`) and name, `DeparturePosition` (the runway), `AppVersion`;
- one `ATCWaypoint` for the departure airport, each charted fix, and the destination airport. SID fixes carry `DepartureFP` and `RunwayNumberFP` (with `RunwayDesignatorFP` for L/R/C), STAR fixes `ArrivalFP`, approach fixes `ApproachTypeFP` (ILS, RNAV, VORDME…) and the runway, enroute fixes their `ATCAirway`. Every fix has its `ICAORegion` and `ICAOIdent`;
- positions in the `.pln` format with the planned altitude: `N50° 6' 3.13",E14° 15' 36.23",+001247.00` (`FormatLLA`).

Runway thresholds, computed points and TOC/TOD are left out: the simulator rebuilds the procedures from their names.

### Reading a .pln

`ReadPLN(r)` and `ReadPLNFile(path)` read a `.pln` as MSFS 2020/2024 and `PLN()` write it, into a `PLNPlan`. They take what the file says and work nothing out:

- **Header:** title, rules (`FPType`), route type, cruise altitude in feet, departure and destination ID, and `DeparturePosition`.
- **Procedures:** the SID with its runway (`DepartureFP`, `RunwayNumberFP` and `RunwayDesignatorFP`: "6" becomes "06", "LEFT" becomes "L"), the STAR, and the approach with its runway, read from the waypoints. The MSFS 2024 layout (`AppVersionMajor` 12, e.g. SimBrief's "M24" export) names them in `DepartureDetails`, `ArrivalDetails` and `ApproachDetails` instead; those are read as well, and the waypoints win where both say. The approach's runway is the arrival runway.
- **Waypoints:** id, type, ident and region, position and altitude (`ParseLLA`), and the airway they are reached by. The MSFS 2024 layout has no `WorldPosition` on its waypoints: their position stays 0,0, and `PLNResolver` looks them up (#679).

`NewPLNResolver(nav.NewNavLoader(client), plan, airportAt)` fills them in: intersections, VORs and NDBs by ident, region and kind through the facility API (a fix in the plan twice asked once), airports with `airportAt` (an `AirportLister.RequestAll` list, loaded layouts; nil leaves them). Call `Start`, feed every message to `Handle`, `Expire` now and then, until `Done`; `Missing` lists what stayed without a position (unknown fixes, user points, airports the lookup did not know). Live, SimBrief's M24 export LKPR–LKPD: BEKVI.LK at 50.0734, 14.7224.

A `.pln` has no alternate airport field, so none is read.

**The active plan in MSFS 2024.** The `FlightPlan` system state is the active plan's path, and the `FlightLoaded` state is the flight's `.FLT`. Live, with no plan loaded, MSFS 2024 gave an empty `FlightPlan` and `…\MISSIONS\Custom\CustomFlight\CustomFlight.FLT`, with `[ATC_Aircraft.0] ActiveFlightPlan=False`. What 2024 reports for a plan made in the EFB or on the world map has not been checked yet.

## Performance

`PerformanceFor` knows A20N, A320, A321, B738, B38M, B77W, B789, E190, CRJ9, AT76 and DH8D; other types plan as a generic medium jet (`Performance.Type` is then empty). The figures are typical round numbers, not a particular airframe's.

### Top of descent

`TopOfDescent(PerformanceFor(type), fromFt, toFt, gsKts, extraNM)` is how far before the point where it should be at `toFt` an aircraft starts down from `fromFt` (#693): at its descent rate (`DescentFPM`) and ground speed above FL100, at the 250 kt limit below it (SERA.6001; 14 CFR 91.117), plus about a mile per 10 kt to slow to 250 kt (the pilots' rule of thumb) and `extraNM` (the slow-down to approach speed, a level segment). From FL360 to 4000 ft at 420 kt: about 137 NM with a 5 NM margin (132 without). `Performance.DescentMach` and `DescentIASKts` are the descent speeds, rounded and checked against the EUROCONTROL Aircraft Performance Database (indicative figures); a turboprop has no Mach.

## Limits

- No winds aloft: ETE and fuel are for still air.
- The profile is straight lines between TOC and TOD; constraints only clamp the planned altitude at their own waypoint.
- The airway graph covers the crawl radius around its centre (250 NM around LKPR in the test data); further out the route is direct.
- The weather from `WeatherReader` is at the user aircraft, so it is right for the departure only; give the arrival its runway or weather yourself.

## The player's choices

A plan the player edits before loading it gives its choices in the request. Each is optional ("" or 0 lets `Plan` choose as before):

- **Procedures:** `SID` and `SIDTransition`, `STAR` and `STARTransition`, `Approach` (by name, "ILS 24", any case) and `ApproachTransition`. Each is checked against its runway. A SID that is not one from the departure runway, a STAR or approach for another runway, or a transition the procedure lacks is chosen as without it, and `FlightPlan.Notes` says so ("SID LANU1F is not one from runway 11: chosen as without it").
- **CruiseFL:** the cruise level.
- **Route:** an ICAO item 15 the player typed ("LANUX DCT APRAQ", "VLM UL86 KEPAD"), flown between the SID and the STAR over the airways known (`AirwayGraph.ExpandRoute`). The SID and STAR are chosen for it: the pair with the shortest way onto the route's first fix and off its last. Procedure and airport names in it are passed over quietly. Fixes or airways not known are noted and flown direct; an airway that does not join its fixes leaves the route out, noted.

`world.PlanRequest` takes the same fields and passes them on. `PlanFlight` only plans: it reads both airports from the simulator (waiting for them, as for the World's own traffic) but loads nothing into the flight. The `.pln` is written by `FlightPlan.PLN`, separately, so a plan can be previewed and changed before it is loaded.
