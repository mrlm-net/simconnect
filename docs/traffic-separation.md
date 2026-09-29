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

## Losing a delay

An arrival the sequencer delays loses the time in the air before it would hold (#391), the way approach control does it:

1. **Speed control.** It flies slower on the rest of its STAR, down to `MinProcedureSpeedKts` (210 kt, clean) or `MinProcedureSpeedTurbopropKts` (170 kt).
2. **Path stretching.** What slowing down cannot absorb, a longer path does: a dog-leg off the longest leg ahead, as radar vectors would give. It sits on the side away from the runway's centreline and adds at most `MaxStretchNM` (30 NM).
3. **Holding.** Whatever is still left goes to the hold (#392).

The final part of the approach, the align and join points on the centreline, is never changed.

```go
a, err := arrival.AbsorbDelay(entry.Delay) // an ArrivalController flying its STAR
// a.SpeedKts, a.ExtraNM, a.Left (for the hold)
route := arrival.ProcedureRoute()          // the rest of the STAR as flown now, dog-leg included
```

- `PlanAbsorption(delay, starNM, speedKts, minKts)` is the plan on its own, and `StretchLeg(a, b, extraNM, side)` the apex of a dog-leg that makes a leg `extraNM` longer.
- `AbsorbDelay` sends MSFS AI the new waypoints. A later call adds to what was absorbed: the sequencer sees the slower, longer flight and asks only for the rest.
- On the final, or when not flying a STAR, it returns `ErrNotOnProcedure`.

On the airport map, an arrival on its STAR is asked to absorb its delay once the delay reaches 30 s, at most every 90 s, so it has slowed before the delay is looked at again. The log says it as ATC would: "CSA701, number 2, delay 2m10s: 210 kt, +3.2 NM".

## Holding

Beyond what speed and a dog-leg can take, an arrival holds (#392). The holding patterns are ours: the simulator's `HOLDING_PATTERN` facility data is unusable, and probing it crashed MSFS 2024.

**The hold.** A `Hold` is a racetrack on a fix: its inbound course, its turn direction (right unless `LeftTurns`), and legs by time.

| Altitude | Leg (`HoldLegTime`) | Speed (`HoldSpeedKts`, Doc 8168) |
|---|---|---|
| up to FL140 | 1 min | 230 kt |
| FL140–FL200 | 1.5 min | 240 kt |
| FL200–FL340 | 1.5 min | 265 kt |
| above | 1.5 min | 280 kt |

The turns are rate one (3°/s, `TurnRadiusNM`).

**The entry.** `Entry(heading)` chooses the ICAO entry from the aircraft's heading to the fix, measured off the inbound course. The sectors are for right turns, mirrored for left:

| Heading off the inbound course | Entry |
|---|---|
| 70° on the non-holding side round to 110° on the holding side | direct |
| the next 70° | teardrop: out 30° off the outbound course, then back in |
| the remaining 110° | parallel: out on the non-holding side, then back through the holding side |

**Flying it.** `Racetrack(alt)` and `EntryPoints(entry, alt)` are the points; MSFS AI flies them as a waypoint chain:
1. The entry and one lap, as a plain chain.
2. Back over the fix, the racetrack alone, sent again with `SIMCONNECT_WAYPOINT_WRAP_TO_FIRST` on its last point, so it circles until released.

**The stack.** `HoldStack` stacks a hold's aircraft at `StepFt` (1000 ft) levels from `BaseFt`, leaving from the bottom:
- `Assign` gives an aircraft the lowest free level;
- `Release` takes one out and returns the new altitudes of those above, which step down.

On an `ArrivalController` flying its STAR:

```go
h, ok := arrival.HoldFix(15)          // the first STAR fix 15 NM or more from the threshold, named after it
entry, err := arrival.EnterHold(h, stack.Assign(callsign))
// … while holding: ProcedureRoute starts at the fix; AbsorbDelay returns ErrHolding
err = arrival.HoldAltitude(6000)      // step down in the stack
err = arrival.LeaveHold()             // on along the STAR from the fix
```

On the airport map, an arrival that still has a minute or more left after speed control and path stretching holds. It uses the first STAR fix at least 15 NM out, in that fix's stack from 6000 ft, and gets an expected further clearance time. It leaves when its sequencer delay is down to a minute, and those above step down. The log reads as ATC would, for example "hold at LOMKI, teardrop entry, maintain 7000 ft, expect further clearance 20:52" and "leave the hold at LOMKI, number 3".

## The runway

`traffic.RunwayController` is the tower of one runway (#393). Each time, give it the users of the runway (`RunwayUser`) and it hands out clearances:

- arrivals on final, with the distance to the threshold and the ground speed;
- aircraft on the runway: rolling for take-off, on the landing roll, or crossing;
- departures lined up;
- departures and crossings at the holding points;
- other traffic (`Other`), which is counted but never cleared.

| Clearance | When |
|---|---|
| take-off | the runway is free; the interval after the last departure has run (`DepartureInterval`: wake, same SID); the next arrival is farther than `MinArrivalNM` (4 NM) and lands later than this departure's runway occupancy plus `Margin` (30 s) |
| line up and go | the take-off is clear from the holding point |
| line up and wait | only the interval or the traffic ahead still runs, and the next arrival leaves room |
| cross | the runway is free and the next arrival lands later than `CrossTime` (40 s) plus the margin |

Departures and crossings are first come, first served at the holding points, one on the runway at a time. `Waiting` says why each still waits: "DLH2 on a 3.0 NM final", "1m20s behind QTR1", "number 2 for departure", "DLH2 on the runway". Departures go in the gaps between arrivals, which is mixed-mode use.

A departure the controller works needs `TaxiRequest.HoldForRunway`. Its pushback and taxi go by themselves, but it stops at the line-up, take-off and runway-crossing gates until `ClearToLineUp`, `ClearForTakeoff` and `ClearToCross`. Arrivals stop at crossings with `ArrivalRequest.HoldAtCrossings`.

On the airport map, every airport runway has a controller, fed every second with our traffic and respected other traffic. It clears our departures and crossings and logs it as ATC would ("runway 06, line up and wait", "cleared for take-off", "cross runway 12/30"), with who waits and why. Aircraft spawned with *hold at every clearance* are the user's: the tower counts them but never clears them. `GET /api/runways?icao=` lists each runway's users.

## Going around

The controller also watches the next arrival on short final (#394). When it is `GoAroundAt` (30 s, about 1.2 NM at 140 kt) from the threshold and the runway is not free, it goes on the `GoAround` list with the reason in `Waiting`. The runway is not free when:

- someone is lined up;
- someone is crossing;
- an arrival is still on it after landing;
- other traffic is on it.

A departure already rolling does not count: it is airborne before the arrival arrives.

`ArrivalController.GoAround` releases the injected arrival to MSFS AI. It flies the published missed approach (`ArrivalRequest.MissedApproach`, from `airport.Procedures.MissedApproach`) at its altitude; at LKPR that is straight ahead to 4000 ft for vectors. Without one it flies a circuit instead. Either way it flies round to the join point on the final, where the injected approach takes over again.

`ApproachSequencer.Rejoin` then sequences it afresh by its new prediction, like a newcomer. It does not keep the place its first approach had, which would push everyone behind it.

On the map the tower sends our arrivals around once per approach ("go around, I say again, go around — CSA1 on the runway") and re-sequences them. The *go around* button does the same by hand.

## Keeping apart

`AirborneSeparation(aircraft, minNM, minFt)` lists every pair of airborne aircraft, closest first, marking those closer than both minima at once: `TerminalSeparationNM` (3), `EnrouteSeparationNM` (5), `VerticalSeparationFt` (1000).

The airport map keeps 5 NM:
- **On final:** its sequencers use `MinSpacingNM` 5, whatever the wake minimum is below it.
- **At the STAR entry:** arrivals appear only 6 NM clear of other aircraft, leaving a mile for the one ahead slowing down.
- **Watched:** a monitor logs every pair under 5 NM and 1000 ft when it starts and when it ends, with its closest distance. `GET /api/separation` returns the closest pairs now and the losses so far.
