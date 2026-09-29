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
