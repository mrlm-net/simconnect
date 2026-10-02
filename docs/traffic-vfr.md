---
title: "VFR Traffic"
description: "v0.19 VFR traffic: light aircraft, circuits round a runway (configurable per airport), reporting points, and how VFR traffic joins and leaves."
order: 16
section: "traffic"
---

# VFR Traffic

v0.19 brings general aviation alongside the airline traffic (epic #431): light aircraft flying circuits and VFR arrivals and departures through reporting points, with the tower's VFR calls. This page grows with the version; what is here is in place.

## Light aircraft (#565)

`ProfileFor` knows five light singles, matched in the model title as the simulator's AI models name them (`Asobo PassiveAircraft C152`, `C172`, `DA40 NG`, `SR22`), and the PA-28:

| Type | Span | Length | Approach | Climb | Flaps (take-off, approach, landing) |
|---|---|---|---|---|---|
| C152 | 10.16 m | 7.34 m | 56 kt | 715 fpm | 10°, 20°, 30° |
| C172 | 11.00 m | 8.28 m | 61 kt | 721 fpm | 10°, 20°, 30° |
| P28A (PA-28) | 9.14 m | 7.10 m | 61 kt | 660 fpm | 0°, 25°, 40° |
| DA40 | 11.9 m | 8.1 m | 64 kt | 1120 fpm | T/O, T/O, LDG |
| SR22 | 11.68 m | 7.92 m | 78 kt | 1270 fpm | 50 %, 50 %, 100 % |

Span, length and rate of climb are the published figures (Wikipedia's specifications: C152, C172R, PA-28-140, DA40 XL, SR22-G5). The approach speed is 1.3 times the published flaps-down stall speed. Wheelbase, CG height, take-off distance and the rotation and climb speeds are estimates; the simulator's SimVars refine the airframe in flight (`Refine`). They are light for wake (ICAO L, RECAT-EU F) and single-engined (`EngineCount` 1). An unknown aircraft below 15 m span is taken for a light single (`GenericProfile`).

## Circuits (#567)

`NewCircuit(layout, "24", CircuitConfig{}, profile)` is a runway end's circuit for one aircraft: upwind, crosswind, downwind (its point abeam the threshold), base, final and the runway, each with its altitude and speed. `Waypoints(leg)` gives them from a leg on as MSFS AI waypoints, the corners rounded to the aircraft's turns.

Every airport, and every runway end, can have its own circuit (`CircuitConfig`). Anything left at zero takes a default:

| Setting | Default |
|---|---|
| `Side` | left-hand (`CircuitRight` for a right-hand circuit) |
| `HeightFt` | 1000 ft above the airfield (`CircuitHeightFt`) |
| `DownwindNM` | two standard-rate turn radii at the circuit speed, at least 0.8 NM (`CircuitMinDownwindNM`): about 1 NM for a C172 |
| `UpwindNM` | 0.5 NM past the departure end, then crosswind (`CircuitUpwindNM`) |
| `BaseNM` | 1 NM before the threshold, then final (`CircuitBaseNM`) |

The circuit speed is 1.25 times the approach speed (`CircuitSpeedFactor`, an estimate: about 75 kt for a C172). The upwind climbs to two thirds of the circuit height before the crosswind turn. The base starts down, and the final follows a 3° path (`CircuitGlideFtPerNM`).

`JoinDownwind()` is the 45° entry: a point a mile out from midfield on the downwind, outside the circuit and towards its upwind end. Flown to midfield, it meets the downwind at 45°; a left-hand circuit is joined with a right turn. `From(LegDownwind)` goes on from there.

## Reporting points (#566)

`ReportingPoint` is a VFR reporting point: a name and a position. The simulator's navigation data has none near LKPR (only unnamed VP* idents), so the airport map keeps them per airport as the user sets them. Use Airport tab → *VFR reporting points* → *Add on the map* and click; they are kept in `vfrpoints.json` (`GET/POST /api/vfrpoints?icao=`).

- **Arrivals:** a VFR arrival enters over one (`ArrivalRequest.CircuitEntry`). It appears there 1,000 ft above circuit height. Its first call says so: "OKARR, Cessna 172, over NOVEMBER, 3200 feet, for landing". At a controlled aerodrome the tower assigns the join (Doc 4444 12.3.4.13: the direction of the circuit and where to join, or a straight-in approach), so it never crosses the runway (`CircuitJoinFor`, `ArrivalRequest.CircuitJoin`, `PlanCircuitArrivalVia`):
  - from the final's sector (within 30° of the extended centreline beyond the threshold, `StraightInSectorDeg`): "make straight-in approach"; it lines up 3 NM out;
  - from within 60° of it (`BaseJoinSectorDeg`): "join left base" (or right), straight to the base turn;
  - from elsewhere: the downwind of the circuit on its side of the runway, by the 45° entry: "join right downwind runway 24" from the north at LKPR, though the published circuit is left-hand.
  Live, OKARR from NOVEMBER (north) flew across the runway at circuit height to the left-hand circuit's entry before.
- **Departures:** a VFR departure leaves via one (`Circuit.DepartureVia`): out of the circuit by the side the point is on, ending over it. Its route shows as "VFR via SIERRA".
- **Which point:** each flight takes one of the airport's points by its call sign, or the one named (`SpawnRequest.VFRPoint`). Without points VFR traffic comes and goes in any direction.
## Circuit arrivals (#568)

`ArrivalRequest.Circuit` (with `InjectApproach`, instead of a STAR in `Procedure`) is a VFR arrival through the circuit. `PlanCircuitArrival` plans it:

1. It appears at the 45° entry at circuit height and speed.
2. MSFS AI flies it to midfield on the downwind, abeam the threshold, to the base turn and onto the final.
3. The injected approach takes over at the final's point (`BaseNM` out) and lands it.

An airliner's injected approach takes over at least 2 NM out. For a circuit the plan lets it take over from 0.5 NM out (`ArrivalProcedure.MinJoinMeters`).

On the airport map: New flight → Arrival → Options → *VFR: join the circuit*. The circuit is the runway end's as set for the airport: `GET /api/circuits?icao=LKPR` shows it, and `POST /api/circuits?icao=LKPR&runway=24` with a `CircuitConfig` sets it (`{}` resets). The settings are kept in `circuits.json` in the user cache folder.

The radio follows Doc 4444 12.3.4.13–17 ([Phraseology](traffic-phraseology.md#11-vfr-in-the-aerodrome-traffic-circuit)):

- The pilot calls the tower "for landing", with the type, the position from the field, the altitude and the information.
- The tower answers "join left downwind runway 24" with the wind and QNH.
- The pilot reports "downwind" abeam the threshold.

### In the landing sequence (#569)

A VFR arrival in the circuit is sequenced with the IFR arrivals to the same runway. When it reports downwind, the tower gives its place (Doc 4444 12.3.4.14 b, `FollowTraffic`): "OKABC, number 2, follow the Airbus A320 on 4 mile final", or "on short final", or "in the circuit" when it follows another circuit aircraft; "number 1" alone when it lands first. When the sequence asks it to lose time, the tower extends its downwind (`CircuitInstruction`, "extend downwind", 12.3.4.15 c). `AbsorbDelay` puts all of the delay into a longer downwind for a circuit arrival, with the base turn and final further out, and gives it no speed instruction and no hold. For a circuit arrival `AbsorbDelay` flies on along the downwind and turns base further out, at circuit speed: up to `CircuitMaxExtendNM` (2 NM; each mile on adds two). At LKPR a C172 asked to lose a minute on the 24 circuit turns base 1.3 NM further out. More than that is lost in an orbit (`Orbit`, Doc 4444 12.3.4.17): a full turn where it is, to the side its circuit turns, standard rate (about 1½ minutes for a C172), said "orbit left" or "orbit right" once 45 s or more remain.

### Touch-and-goes (#569)

`ArrivalRequest.TouchAndGos` (with `Circuit`) is how many touch-and-goes a VFR arrival makes before its full stop. When its nose wheel is down it does not roll out: it takes off again from where it is, at the speed it has (an injected `TakeoffMover`), with take-off flaps. At `VFRHandoverFt` MSFS AI flies the circuit again from its crosswind leg round to the final, and the injected approach takes over there as on the first circuit. `ArrivalEvent.TouchAndGo` is set while it is on the runway for one; `TouchAndGosLeft` tells how many remain.

On the airport map the crew reports "downwind, touch and go" and the tower clears "cleared touch and go" (12.3.4.16 c); the last landing is cleared to land. After each touch-and-go it is sequenced again. New flight → Arrival → *VFR through the circuit* takes a number of touch-and-goes. Two in five scheduled VFR arrivals fly two to four training circuits (by their call sign).

### Airspace classes and traffic information (#570)

`SeparationRequired(class, a, b)` says whom ATC separates. In class C (and stricter) IFR is separated from IFR and from VFR. In D and E only IFR is separated from IFR, and in G nobody is. Where two aircraft are not separated, they are told of each other: `TrafficInformation` ("OKABC, traffic, 2 o'clock, 3 miles, opposite direction, Airbus A320, 2500 feet", with `TrafficRelative` for the o'clock, distance and direction), acknowledged "Looking out".

On the airport map the managed airports' zones are class D (`-airspace` sets C, D, E or G), out to 10 NM and up to 5,000 ft above the field; around them class E. Our aircraft fly under their rules (`ControlView.Rules`), other traffic is IFR except light singles. A predicted conflict that needs no separation gets traffic information on each of ours' frequency, at most every 3 minutes, instead of a resolution. Such a pair coming close is not logged as a loss of separation.

### Standard overhead join (#567)

With `CircuitConfig.OverheadJoin` (Airport tab → VFR circuits → *Standard overhead join*), VFR arrivals join overhead from any side (`LegOverhead`). They cross the field `OverheadAboveFt` (1,000 ft) above circuit height, descend on the dead side (the side away from the circuit) to circuit height, cross the upwind end of the runway at circuit height onto the crosswind leg, then fly the downwind, base and final. The tower says "join overhead runway 24". The procedure follows UK practice at airfields without a tower: UK Airprox Board report 2025183 quotes the Sherburn-in-Elmet AIP entry, "join overhead at 2000 FT QFE and descend in accordance with the 'Standard Overhead Join' procedure", with circuits at 1000 FT QFE and the descent "on the deadside". Without it the tower assigns the join, as at LKPR. Without reporting points an overhead join comes from 6 NM out in a direction by its call sign.

### Another circuit (#569)

`AnotherCircuit` has a circuit arrival fly round once more: on round its circuit to the final, over the runway at circuit height, then upwind, crosswind, downwind, base and final again. That is about 4 minutes more for a C172. The sequence uses it when 3 minutes or more must be lost beyond the extended downwind, saying "make another circuit" (Doc 4444 12.3.4.17 c), and an orbit for less.

### Stop-and-goes (#567)

With `ArrivalRequest.StopAndGo` each touch-and-go is a stop-and-go: it brakes to a stop on the runway at its `BrakeDecel`, stands `StopAndGoWait` (10 s, an estimate), and takes off from there. On the map the crew reports "downwind, stop and go" and the tower says "cleared stop and go" (`ClearedStopAndGo`, worded like the touch-and-go; Doc 4444 12.3.4.16 does not list it). The New flight form has a *Stop-and-go* box; a third of the scheduled training circuits are stop-and-goes.

### VFR departures

`Circuit.Departure(exitBearing)` is the way out of the circuit towards an exit point `VFRExitNM` (5 NM) from the field, `VFRExitAboveFt` (1,000 ft) above circuit height. It leaves the circuit by the side the exit is on:

- **ahead** (within 45° of the runway heading): straight out from the upwind;
- **to the circuit's side**: by its crosswind leg;
- **to the other side**: turning away from the circuit after the upwind;
- **behind**: by the crosswind and downwind on the circuit's side, or by their mirror on the other side. It never crosses the circuit.

With `TaxiRequest.VFR` the injected take-off hands over to MSFS AI at `VFRHandoverFt` (400 ft), so MSFS AI flies the turns. `VFRDepartureWaypoints` gives it the points at their altitude and the circuit speed. On the airport map the New flight option *VFR through the circuit* works for departures too: the exit is in a random direction (`SpawnRequest.ExitBearing` sets it) and the route is shown as "VFR north". A VFR departure gets no departure clearance: its first call is to ground, for start-up or taxi.

### Scheduled VFR flights

`VFRFlights(VFROptions, from, to)` adds light aircraft flying in to the focus airports through the circuit, and as many flying out of them, `VFRPerHour` (1) an hour by default, scaled by the density. Each flight has `Rules` "VFR", no origin, a light type (`VFRTypes`: C172, P28A, C152, DA40, SR22 by weight) and as call sign a registration of the airport's country (`VFRRegistration`: OKABC at LKPR, DEABC in Germany, GABCD in the UK).

- They fly by day only. It must be day (`Daylight`: the sun no lower than 6° below the horizon, civil twilight, `SunElevation`) from when the flight appears until a quarter of an hour after its STA. At LKPR on 2 October the first lands at 06:55 local and the last at 18:40.
- They fly in visual conditions only (`VFROptions.Visual`). The airport map asks for a visibility of 5 km and a ceiling of 1,500 ft or better at the user's aircraft. It checks again at the spawn, and a flight waits while the weather is worse.
- The manager has a VFR flight appear `ManagerOptions.VFRLead` (8 min) before its STA, near the airport, never en route.
- On the map it joins the circuit as a circuit arrival (no flight plan, no STAR) and parks on a GA ramp where the airport has one free, else on any stand that fits.

Not yet:

- A go-around from the circuit still flies the IFR go-around circuit.
- Touch-and-goes are still to come.
- Touch-and-goes from a straight-in or base join fly the circuit afterwards on its own side.

## Jetbridges (#572)

Jetbridges for our traffic are blocked by the simulator.

- `RequestJetwayData` lists an airport's jetways: 27 at LKPR. Each is a SimObject with a status and the aircraft attached, and its parking index is the layout's stand index.
- `TOGGLE_JETWAY` sent to one of our aircraft parked at a jetway stand did nothing.
- Our traffic is created as NonATC aircraft, the only kind whose stand we can choose. A devsupport thread (March 2026) reports that only ParkedATC and EnrouteATC aircraft use jetways.
- `types.DecodeJetwayData` reads one packed 160-byte `SIMCONNECT_JETWAY_DATA` entry (`JetwayDataSize`); the Go struct is laid out differently and must not be cast onto the wire data (#606).
