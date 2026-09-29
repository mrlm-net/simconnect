---
title: "Airborne Separation"
description: "Wake turbulence categories and separation minima: spacing on final, departure intervals and runway occupancy, the basis of the approach sequencer and the runway controller."
order: 11
section: "traffic"
---

# Airborne Separation

v0.16 separates traffic in the air as well as on the ground. This page grows with it. First come the standards everything else builds on (#389).

## Wake categories

`traffic.WakeFor(type)` gives a type's wake categories. The type is an ICAO designator (`B77W`) or a model title that `ProfileFor` understands (`FSLTL_FAIB_B738_TVS-Smartwings`). It returns two categories:

| ICAO (`WakeCategory`) | RECAT-EU (`RecatCategory`) | Examples |
|---|---|---|
| `J` super | `A` super heavy | A388 |
| `H` heavy | `B` upper heavy | B744, B748, B77W, A35K, A346, MD11 |
| `H` heavy | `C` lower heavy | B787, A330, A359, B767, A310 |
| `M` medium | `D` upper medium | A320 family, B737, B757, A220-300 |
| `M` medium | `E` lower medium | E-Jets, CRJ, ATR, Dash 8, A220-100 |
| `L` light | `F` light | C208, PC-12, bizjets and GA |

A type not in the table takes the category of its wing span: below 15 m light, below 32 m lower medium, below 52 m upper medium, below 70 m heavy, super above. When nothing is known about a type at all, it counts as medium.

## Spacing on final

`ArrivalSeparationNM(leader, follower, scheme)` is the distance a follower keeps behind the aircraft landing before it on the same runway. It is never less than `MinRadarSeparationNM` (3 NM).

**ICAO** (`SchemeICAO`, Doc 4444 §8.7.3.4), in NM:

| Leader \ follower | J | H | M | L |
|---|---|---|---|---|
| J | 3 | 6 | 7 | 8 |
| H | 3 | 4 | 5 | 6 |
| M | 3 | 3 | 3 | 5 |
| L | 3 | 3 | 3 | 3 |

**RECAT-EU** (`SchemeRecat`), in NM (– is the minimum radar separation):

| Leader \ follower | A | B | C | D | E | F |
|---|---|---|---|---|---|---|
| A | 3 | 4 | 5 | 5 | 6 | 8 |
| B | – | 3 | 4 | 4 | 5 | 7 |
| C | – | – | 3 | 3 | 4 | 6 |
| D | – | – | – | – | – | 5 |
| E | – | – | – | – | – | 4 |
| F | – | – | – | – | – | 3 |

`SeparationTime(distNM, followerKts)` turns a distance into time at the follower's ground speed, for time-based spacing that holds in a headwind.

## Departures and the runway

- `DepartureInterval(leader, follower, sameRoute)` is how long a departure waits after the one before it on the same runway (Doc 4444 §5.8.3). The default is 1 minute on diverging routes and 2 minutes on the same SID. A medium or light following a heavy waits 2 minutes. Anything following a super waits 3 minutes, or 2 if it is heavy.
- `RunwayOccupancy(wake, landing)` is a typical time on the runway. Landing, it runs from the threshold until clear: 45–70 s by category. Departing, it runs from lining up until lift-off: 40–60 s.

The figures come from ICAO Doc 4444 (PANS-ATM) and EUROCONTROL RECAT-EU (2018). The assignments of types to RECAT-EU categories follow its tables where they list a type, and its weight and span criteria otherwise.

## The landing sequence

`traffic.ApproachSequencer` is the approach controller of one runway (#390).

- **Predicted landing:** for each arrival it predicts when it would land flying on as it is: the distance to go at its ground speed now, with the last `FinalNM` (10 NM) at its final speed.
- **Order:** first come, first served by predicted landing.
- **Landing time:** the earliest time that keeps the wake spacing behind the one before (`ArrivalSeparationNM`, as time at the follower's final speed) and leaves the runway free (`RunwayOccupancy`). The difference from the prediction is the arrival's **delay**, for speed control, path stretching and holding to absorb.
- **Fixed arrivals:** some keep their place and are never delayed; the others fit around them. These are arrivals inside `FreezeNM` (8 NM, about the final approach fix), which are established, and arrivals marked `Fixed`, such as other traffic, which is not ours to delay.

```go
seq := traffic.NewApproachSequencer("24", traffic.SequencerOptions{
    Scheme:   traffic.SchemeICAO, // or SchemeRecat
    OnChange: func(c traffic.SequenceChange) { log.Println(c.Entry.Callsign, c.Entry.Number, c.Entry.Delay) },
})
entries := seq.Update(time.Now(), []traffic.ApproachAircraft{{
    Callsign:       "CSA880",
    Wake:           traffic.WakeFor("A320"),
    DistanceToGoNM: traffic.DistanceToGo(pos, starAndApproach, threshold),
    GroundKts:      280, FinalKts: 140,
}})
```

Each `SequenceEntry` has:
- its place: `Number`, `Leader`, `SpacingNM`;
- its times: `ETA`, `Landing`, `Delay`;
- `Fixed` and `DistanceToGoNM`.

`OnChange` reports a new arrival, a new number, a delay change of at least `DelayStep` (30 s), and an arrival leaving the sequence.

`DistanceToGo(pos, route, threshold)` is the track distance from a position along the route still ahead to the threshold. Before the route, it counts from the route's first point.

On the airport map, a sequencer runs per airport and arrival runway. It is fed every second with:
- our arrivals on their STAR and approach;
- our arrivals en route to the STAR entry;
- when respected, the other traffic arriving there, as fixed.

Changes go to the traffic log, and `GET /api/sequence?icao=` returns the runways and their sequences.

## Weather on final

Spacing on final follows the weather, as it does in life. `ConditionsFrom(weather, runwayHeadingTrue)` gives the `ApproachConditions`: visibility, ceiling, the headwind on final and the runway surface. Rain makes the runway wet; snow, or precipitation at or below 0 °C, makes it contaminated. `ArrivalSpacing(leader, follower, scheme, conditions, allowReduced)` applies them, in this order:

| Conditions | Spacing |
|---|---|
| minimum radar separation applies; visibility ≥ 5 km, ceiling ≥ 1000 ft, dry runway, and the airport approved (`AllowReduced`) | **2.5 NM** reduced separation (Doc 4444 §8.7.3.2) |
| contaminated runway | **+1 NM** (poor braking, longer on the runway) |
| low visibility procedures: visibility < 550 m (RVR, CAT II/III) or ceiling < 200 ft | at least **6 NM**, so the aircraft ahead is clear of the ILS sensitive area |

- **Runway occupancy** grows on the surface (`RunwayOccupancyIn`): 15 % wet, 40 % contaminated.
- **Wind:** a headwind slows the ground speed on final (`FinalGroundKts`). By default the sequencer keeps the distance, so the time between landings grows into the wind. With `TimeBased` it keeps the calm-wind time instead: time-based separation, where the distance shrinks in a headwind and the landing rate holds.

Call `SetConditions` on the sequencer with the conditions of its runway. Each `SequenceEntry` says why its spacing differs from the wake minimum (`SpacingWhy`).

The traffic manager spaces its arrival spawns the same way (`ManagerOptions.Conditions`): twice as far apart in low visibility, a third more on a contaminated runway.

The airport map takes the weather at the user aircraft (SimConnect reports no other), on the runway in use. It logs a change of conditions for each runway, and `GET /api/sequence` includes the conditions and `lvp`.
