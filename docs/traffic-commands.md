---
title: "ATC Commands"
description: "Every clearance and command for injected departures and arrivals: when it applies, what the aircraft does and the events it reports."
order: 5
section: "traffic"
---

# ATC Commands

Injected traffic (`TaxiWithInjector`, `ArrivalWithInjector`) waits at clearance gates when the request asks for it (`TaxiRequest.HoldForClearances`, `ArrivalRequest.HoldForClearance` / `HoldAtCrossings`); otherwise each gate clears itself after a short, varied wait. A clearance given before its gate means no stop there. Every command is safe to call from any goroutine; progress arrives on `Events()` (`TaxiEvent`, `ArrivalEvent`).

The [airport map](airport-layout.md#seeing-it-on-a-map) gives all of them from the aircraft cards and logs them in ICAO phraseology.

## Departures (`TaxiController`)

| Command | When | What happens | Events |
|---|---|---|---|
| `ClearPushback()` | `TaxiAwaitingPushback` | Pushback (tug if `Tug` is set), tail onto the taxiway the taxi-out leaves by | `TaxiPushback`, then `TaxiAwaitingTaxi` |
| `HoldPushback(on)` | before the push has begun (beacon not on) | Keeps the aircraft on its stand, even when cleared, until `HoldPushback(false)`: a ground stop, such as the [Traffic Manager's](traffic-manager.md#situation-checks) `AdviceHold` | none |
| `ClearToTaxi()` | awaiting taxi, taxiing, holding at a crossing | Taxi to the holding point; removes a `ClearUpTo` limit or a hold position | `TaxiTaxiing`, `TaxiHoldingShort` (`HoldingShortOf` = the runway) |
| `ClearUpTo(node)` | before or during the taxi | Taxi and hold with the nose gear on `node` ("taxi via A, hold short of B"). A limit already behind when the taxi starts holds the aircraft and reports `ErrNotOnRoute` | `AtLimit`, `LimitNode` |
| `ClearStartUp()` | with `HoldForClearances`, before the taxi | Engines start once the tug has gone; a taxi clearance without it covers the start-up too | none |
| `ClearPushbackFacing(dir)` | before the push has begun | Pushback planned to end facing a compass direction ("north", "e"…); `PushFacing()` / `PushFacingSaid()` tell the planned one. After the push began: `ErrTooLate` | as `ClearPushback` |
| `HoldPosition()` | taxiing (not while lining up: `ErrNotTaxiing`) | Stops as soon as comfortably possible (`HoldPositionDecel`, 1.2 m/s²) and waits for `ClearToTaxi` / `ClearUpTo` | `AtLimit` with `LimitNode` −1 |
| `ClearToCross()` | holding short of a runway to cross | Crosses; strobes and landing lights on between the hold-short lines | `TaxiTaxiing` |
| `ClearToLineUp()` | holding short of the departure runway | Lines up and waits | `TaxiLiningUp`, `TaxiLinedUp` |
| `ClearForTakeoff()` | holding short or lined up | Take-off (rolling if not lined up yet), hand-over to MSFS AI at `ClimbHandoverFt`, then the SID (`TaxiRequest.Departure`) | `TaxiDeparting`, `TaxiComplete` |
| `AbortTakeoff()` | lined up / lining up: cancels the take-off clearance. On the roll before V1 (`RotateKts − V1MarginKts`): rejects | Brakes at `RejectDecel` to a stop, vacates at the next exit ahead and taxis back to the holding point for new clearances. Past V1: `ErrTooLate`, the take-off continues | `TaxiTaxiing`, `TaxiHoldingShort` |
| `Expedite(on)` | waiting at a gate | The waits before taxi, line-up and take-off shrink to `RushDelayFactor` (0.35) | none |
| `ChangeRunway(runway, entry, sid)` / `ChangeEntry(entry)` | before lining up | Re-plans runway, SID, entry and taxi route from where the aircraft is; lining up or later `ErrTooLate`; an entry too short for the type `ErrEntryTooShort` | none |
| `TaxiVia(nodes)` | on the stand, pushed back, taxiing | Re-plans the taxi-out through graph nodes in order | none |
| `AvoidOccupied(occ)` | taxiing, no limit, no custom route | Re-plans round places others take (`airport.Occupied`, e.g. another departure's `Occupies()`); reports whether the route changed | none |
| `DirectTo(pos, altFt, kts, fix)` / `ClimbTo(pos, altFt, ft)` / `Reroute(route)` | handed to MSFS AI | Direct to a fix of the climb route; climb on to `ft` past the SID's top; a new route (`ClimbPlan` / `ResolvedRoute`) | none |
| `Cancel()` | any time | Removes the aircraft | `TaxiCancelled` |

## Arrivals (`ArrivalController`)

| Command | When | What happens | Events |
|---|---|---|---|
| `AbsorbDelay(delay)` | MSFS AI on the STAR, before the final | Slower on the rest of the STAR, then a longer downwind or one dog-leg (a later call lengthens the same dog-leg while it is ahead); returns what is left for a hold ([Losing a delay](traffic-separation.md#losing-a-delay)) | none |
| `EnterHold(hold, altFt)` / `HoldAltitude(altFt)` / `LeaveHold()` | on the STAR / holding | Enters the hold at a fix (`HoldFix`), changes level in the stack, goes on along the STAR ([Holding](traffic-separation.md#holding)) | none |
| `DirectTo(p)` | on the procedure | Direct to a point picked on the map: a named fix within `PointSnapNM` (1 NM) of it, else a radar vector (`Vector`) and then direct to the next fix | none |
| `JoinFinal(p)` | on the procedure | Joins the final where `p` lies along it, on a `FinalInterceptDeg` (30°) intercept; returns the distance out and the heading | none |
| `AssignSpeed(kts)` | on the procedure | Flies `kts` on the rest of the STAR, no slower than the type's minimum; 0 resumes `ProcedureSpeedKts` | none |
| `DirectToJoin()` / `Shortcut(maxSaveNM, keep)` / `StopDescent(altFt, forNM)` | on the STAR | Straight to the join point; direct to a fix further on; level off for a distance | none |
| `ChangeRunway(runway, procedure, missed)` | before the injected final | New arrival plan and procedure from where it is; later `ErrTooLate` | none |
| `ChangeStand(parking)` | before touchdown | Taxi-in planned to another stand; landed: `ErrStandFixed` | none |
| `GoAround()` | injected final, before touchdown | Climbs out; MSFS AI flies the published missed approach (`ArrivalRequest.MissedApproach`), else a left-hand circuit (`GoAroundClimbNm` 3, `GoAroundOffsetNm` 3.5, `GoAroundHeightFt` 3000), back to the join point, where the injected approach takes over again. A VFR circuit arrival goes round its own circuit. Still on the STAR: nothing to do. On the runway: `ErrTooLate` | `ArrivalApproaching`, then the approach again |
| `ReduceToFinalSpeed()` | injected final | Flies the final approach speed from now on; returns the time that gains (`FinalSlowGain()`) | none |
| `AnotherCircuit()` / `Orbit()` | VFR circuit arrival in its circuit | Another circuit; a full orbit where it is. Returns about how long it takes | none |
| `Expedite(on)` | before the rollout | Vacates `RushExitKts` (6 kt) faster at the exit | none |
| `AvoidOccupied(occ)` | awaiting taxi or taxiing, no limit, no crossing ahead | Re-plans the taxi-in round places others take (`airport.Occupied`); reports whether the route changed | none |
| `ClearToTaxi()` | after landing, taxiing | Taxi to the stand; removes a limit or a hold position | `ArrivalTaxiing`, `ArrivalParking`, `ArrivalParked` |
| `ClearUpTo(node)` | after landing, taxiing | Progressive taxi, as for departures | `AtLimit`, `LimitNode` |
| `HoldPosition()` | taxiing | Stops and waits for `ClearToTaxi` / `ClearUpTo` | `AtLimit` |
| `ClearToCross()` | holding short of a runway to cross | Crosses | `ArrivalTaxiing` |
| `Cancel()` | any time | Removes the aircraft | `ArrivalCancelled` |

## Without a clearance

Aircraft keep their distance by themselves: a taxiing aircraft stops behind traffic on its path (`TrafficGapMeters`), and where two routes cross or merge the one further from the conflict gives way ([Ground traffic](traffic-taxi.md#ground-traffic)). The controller's commands come on top of that.

## Errors

| Error | Meaning |
|---|---|
| `ErrNotInjected` | The command needs an injected controller |
| `ErrNotTaxiing` | Hold position for an aircraft not moving on the ground |
| `ErrNotApplicable` | The command does not fit the phase |
| `ErrTooLate` | Past V1, already on the runway for a go-around, a push already begun, a runway change too late |
| `ErrNotOnRoute` | A `ClearUpTo` node not ahead on the route |
| `ErrNotOnProcedure` | `AbsorbDelay`, `EnterHold`, `DirectTo`, `JoinFinal`, `Shortcut` and the like for an arrival not flying its STAR (or circuit) |
| `ErrHolding` | `AbsorbDelay`, `DirectTo`, `JoinFinal`, `Shortcut` and the like while the arrival holds |
| `ErrStandFixed` | `ChangeStand` after touchdown |
| `ErrEntryTooShort` | `ChangeEntry` to an entry leaving too little runway for the type |
| `ErrNotHolding` | `HoldAltitude` or `LeaveHold` for an arrival not in a hold |
