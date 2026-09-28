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
| `ClearToTaxi()` | awaiting taxi, taxiing, holding at a crossing | Taxi to the holding point; removes a `ClearUpTo` limit or a hold position | `TaxiTaxiing`, `TaxiHoldingShort` (`HoldingShortOf` = the runway) |
| `ClearUpTo(node)` | before or during the taxi | Taxi and hold with the nose gear on `node` ("taxi via A, hold short of B"). A limit already behind when the taxi starts holds the aircraft and reports `ErrNotOnRoute` | `AtLimit`, `LimitNode` |
| `HoldPosition()` | taxiing, lining up | Stops as soon as comfortably possible (`HoldPositionDecel`) and waits for `ClearToTaxi` / `ClearUpTo` | `AtLimit` with `LimitNode` −1 |
| `ClearToCross()` | holding short of a runway to cross | Crosses; strobes and landing lights on between the hold-short lines | `TaxiTaxiing` |
| `ClearToLineUp()` | holding short of the departure runway | Lines up and waits | `TaxiLiningUp`, `TaxiLinedUp` |
| `ClearForTakeoff()` | holding short or lined up | Take-off (rolling if not lined up yet), hand-over to MSFS AI at `ClimbHandoverFt`, then the SID (`TaxiRequest.Departure`) | `TaxiDeparting`, `TaxiComplete` |
| `AbortTakeoff()` | lined up / lining up: cancels the take-off clearance. On the roll before V1 (`RotateKts − V1MarginKts`): rejects | Brakes at `RejectDecel` to a stop, vacates at the next exit ahead and taxis back to the holding point for new clearances. Past V1: `ErrTooLate`, the take-off continues | `TaxiTaxiing`, `TaxiHoldingShort` |
| `Cancel()` | any time | Removes the aircraft | `TaxiCancelled` |

## Arrivals (`ArrivalController`)

| Command | When | What happens | Events |
|---|---|---|---|
| `GoAround()` | injected final, before touchdown | Climbs out; MSFS AI flies a left-hand circuit (`GoAroundClimbNm`, `GoAroundOffsetNm`, `GoAroundHeightFt`) back to the join point, where the injected approach takes over again. Still on the STAR: nothing to do. On the runway: `ErrTooLate` | `ArrivalApproaching`, then the approach again |
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
| `ErrTooLate` | Past V1, or already on the runway for a go-around |
| `ErrNotOnRoute` | A `ClearUpTo` node not ahead on the route |
