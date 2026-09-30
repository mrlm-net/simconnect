---
title: "Radio"
description: "What ATC says, as structured transmissions: positions, intents and parameters with the text as said, for a log, a UI or a voice library."
order: 12
section: "traffic"
---

# Radio

v0.17 puts our traffic on the radio. Every clearance our controllers give is a **transmission**: who says it, to whom, what it does and with what, plus the text as ATC says it. A log, a UI or a voice library renders it. This page grows with v0.17: transmissions first (#415), then frequencies and handoffs, the pilot side, the ATIS on its frequency, and voice through `voice-goio`.

## Transmissions

```go
t := traffic.ClearedTakeoff("CSA123", "24", false)
// t.Position == traffic.PosTower
// t.Intent   == traffic.IntentTakeoff
// t.Params   == map[string]string{"runway": "24"}
// t.Text     == "CSA123, runway 24, cleared for take-off"
```

`Transmission` has these fields:

| Field | What |
|---|---|
| `Position` | the controller's position: delivery, ground, tower, approach, departure, center, ATIS |
| `Callsign` | the aircraft |
| `Intent` | what it does: pushback, taxi, taxi limit, cross, line up, take-off, hold position, stop, cancel take-off, go around, sequence, direct, hold, leave hold, hold level, speed, level, heading, and the departure and arrival clearances |
| `Params` | its parameters as said: runway, entry, taxiways, stand, limit, SID, STAR, approach, number, delay, fix, altitude, level, speed, heading, turn, traffic, reason (`Param*` keys) |
| `Text` | the phrase in the application's normal tokens, ICAO phraseology |
| `At`, `Airport` | when and where |
| `Frequency` | the frequency, once positions have frequencies |
| `Pilot` | said by the pilot (readbacks and requests) |

The text is made from the intent and parameters by one phrasebook (`Say`), so every sender says the same thing the same way. A builder exists for each clearance:

- `ClearedDeparture` and `ClearedArrival`;
- `ClearedPushback`, `ClearedTaxiToRunway` and `ClearedTaxiToStand` (taxiways as said: `Route.SpokenTaxiways`), `ClearedTaxiUpTo`;
- `ClearedCross`, `ClearedLineUp`, `ClearedTakeoff`;
- `HoldPosition`, `Stop`, `CancelTakeoff`, `GoAround`;
- `Sequenced`, `DirectToFinal`, `HoldAt`, `LeaveHoldAt`, `HoldDescend`;
- `Resolved` for a conflict resolution: speed, level (`LevelSaid`: "flight level 210", "altitude 9000 feet") or heading.

The text is what a voice library such as `voice-goio` takes as it is: it normalises "CSA123, runway 24, cleared for take-off" into speech itself.

## The radio

A `Radio` carries the transmissions. It stamps each one (`RadioOptions.Now`, e.g. the traffic clock's `SimClock.Now`), keeps the last `Keep` (200), and hands each to `OnTransmission` in order:

```go
radio := traffic.NewRadio(traffic.RadioOptions{Now: clock.Now, OnTransmission: func(t traffic.Transmission) {
	log.Printf("%s ATC: %s", t.Callsign, t.Text)
}})
radio.Transmit("LKPR", traffic.ClearedLineUp("CSA123", "24"))
recent := radio.Recent("LKPR", 50) // oldest first
```

On the airport map every ATC line of the traffic log comes from its radio: the ground and tower clearances, the tower's automatic ones, the sequencer's delays and holds, the Approach tab's actions and the conflict resolutions. The wording is unchanged. `GET /api/radio?icao=LKPR&n=50` serves the recent transmissions.

## Frequencies and handoffs

An airport's frequencies come with its layout. `Layout.Frequencies` holds each one's kind (`FreqATIS`, `FreqClearance`, `FreqGround`, `FreqTower`, `FreqApproach`, `FreqDeparture`, `FreqCenter`, `FreqCTAF`), its MHz and the name the scenery gives it ("PRAHA TOWER"). `FrequencyFor(kind)` finds a position's frequency and falls back as ATC does where a position isn't staffed on its own:

- clearance to ground;
- ground to tower;
- departure to approach;
- approach to centre;
- tower to the common traffic frequency.

Who works an aircraft follows its state:

| | Positions |
|---|---|
| Departure (`DeparturePosition`) | delivery for the clearance → ground to its runway's holding point → tower for the line-up and take-off → departure once handed to MSFS AI |
| Arrival (`ArrivalPosition`) | approach on the STAR and approach → tower once established on the final, through the landing roll and vacating → ground to the stand |

Runway crossings stay on the ground frequency, the tower having agreed, as at most airports.

A change of position is a handoff, said by the position handing over. `Handoff` gives "CSA123, contact Praha Tower 118.105" (`StationName` makes "Praha Tower" from the scenery's name). With `RadioOptions.FrequencyOf` the radio puts each transmission on its position's frequency and says one at a time on each frequency. While one transmission is said (`SpeakingTime`: about 160 words a minute), the next is stamped for when it ends, so a voice plays them in turn.

On the airport map each aircraft's card shows who works it and on what frequency ("📻 tower 118.105"), and the traffic log reads as the radio: the clearance on each frequency, then "contact Praha Ground 121.905".
