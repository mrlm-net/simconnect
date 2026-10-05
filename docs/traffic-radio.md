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

The wording follows [Phraseology](traffic-phraseology.md): ICAO Doc 4444 chapter 12, with the UK CAP 413 for wording and order where Doc 4444 leaves them open, each phrase pinned by a test to its source (#462). The departure clearance gives the destination, the SID by its fix (`SaidProcedure`: "BALTU 7D"), the runway, the initial climb (`LevelSaidAbove`, against the transition altitude) and a squawk; start-up and pushback are two approvals, mostly given in one ("push back and start-up approved"; the map's crews ask so 85% of the time); the take-off and landing clearances carry the wind after the clearance (`WindSaid`); the tower clears the next arrival to land within 6 NM with the runway free (`RunwayClearances.Land`) and tells it on the landing roll to call ground when vacated (`WhenVacatedContact`). The crew's clearance request to delivery follows the order of CAP 413 4.9; its exact wording is not in Doc 4444.

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

On the airport map every ATC line of the traffic log comes from its radio: the ground and tower clearances, the tower's automatic ones, the sequencer's delays and holds, the Sequence section's actions and the conflict resolutions. The wording is unchanged. `GET /api/radio?icao=LKPR&n=50` serves the recent transmissions.

### Whom the controller calls first

On the airport map each controller decides whom to call next (`cmd/airport-map/agenda.go`). A clearance it has decided waits on its frequency's agenda until the controller's answer time (1–5 s). It goes out once the frequency has been quiet for `atcAnswerDelay` (1 s), one call at a time. The most urgent ready call goes first, by class:

1. a go-around;
2. a landing clearance;
3. a take-off, line-up or runway crossing;
4. an approach clearance;
5. taxi for an arrival that has vacated (it is in the way at its exit);
6. taxi for a departure;
7. pushback and start-up;
8. a departure clearance.

Within a class the call that has waited longest goes first. A call no longer wanted when its turn comes is dropped: a crew that no longer asks, or a runway clearance the runway controller no longer grants (not granted for `grantFresh`, 2.5 s). A dropped runway clearance is given again once it is granted again. Hold position, cancel take-off and the other safety calls are said at once, as before. Before, each answer went out in the order it was decided, so a landing clearance waited behind line-ups and pushbacks.

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

A change of position is a handoff, said by the position handing over. `Handoff` gives "CSA123, contact Praha Tower 118.105" (`StationName` makes "Praha Tower" from the scenery's name). `StationFor(layout, pos)` gives a position's station and frequency as the airport map says them: the AIP's unit call sign where the scenery's names do not give the unit (`AIPUnitName`; at LKPR "Ruzyne Delivery", "Ruzyne Ground", "Ruzyne Tower", "Ruzyne Radar", "Praha Radar"), else `StationName` for what the frequency is; a position with no frequency of its own (departure at LKPR) talks on the one that serves it (approach). With `RadioOptions.FrequencyOf` the radio puts each transmission on its position's frequency and says one at a time on each frequency. While one transmission is said (`SpeakingTime`: about 160 words a minute), the next is stamped for when it ends, so a voice plays them in turn.

On the airport map each aircraft's card shows who works it and on what frequency ("📻 tower 118.105"), and the traffic log reads as the radio: the clearance on each frequency, then "contact Praha Ground 121.905".

## The pilot side

Clearances follow requests in radio order (#462). Held for clearances (`TaxiRequest.HoldForClearances`), a departure's crew asks when it is ready: `TaxiEvent.Request` is "pushback" once its own wait is over (boarding, the scheduled time) and "taxi" once pushed with the tug clear. On the airport map the push request is the crew's first call to ground, with the station, stand and ATIS letter. The ground controller answers after the request has been said and a moment (1.5–3 s); the crew reads the clearance back, and the aircraft acts 2–4 s after the readback (`Radio.ClearAt`: when a frequency is clear). The tower's line-up and take-off clearances work the same way. In the ATC game you answer the requests yourself.


Our pilots talk too (#417). A pilot's transmission has `Pilot` set, on the same frequency as the controller's.

- **Requests and reports:** `RequestPushback` ("CSA123, stand A4, request push and start-up, information B"), `RequestTaxi`, `ReadyForDeparture`, `Vacated`.
- **The first call on a frequency:** `CheckIn`: "Ruzyne Tower, CSA123, holding point runway 24, ready for departure", with the ATIS letter on the first call of all.
- **Readbacks:** `Readback` reads a clearance back the ICAO way, what must be read back and then the call sign: "Taxi to and hold short of runway 24 at B via H, A, CSA123"; "Ruzyne Tower 134.56, CSA123"; "Climb flight level 210, CSA123"; "Hold position, CSA123" (the project reads hold position back as given, where Doc 4444 has "holding"), and "Continue taxi, CSA123" after it. With `RadioOptions.ReadBack` the radio has our pilots read back every clearance, after it, on its frequency.
- **Checking a readback:** `CheckReadback(clearance, heard)` compares what was read back (parameters as recognised, e.g. a voice recogniser's tags) with the clearance. It ignores case, spacing and leading zeros ("6" for "06", "FL210" for "flight level 210"). A wrong or missing item gets the controller's correction: "CSA123, negative, taxi via H, hold short of A". `SayAgain` asks a call sign, or "station calling", to say again.

On the airport map the traffic log reads as the radio, pilot lines marked `pilot:`:

```
AUA1976 ATC: AUA1976, cleared VOZ5D departure, runway 06
AUA1976 pilot: Cleared VOZ5D departure, runway 06, AUA1976
AUA1976 ATC: AUA1976, contact Ruzyne Ground 121.91
AUA1976 pilot: Ruzyne Ground 121.91, AUA1976
AUA1976 pilot: Ruzyne Ground, AUA1976, stand A1
AUA1976 pilot: AUA1976, stand A1, request push and start-up
AUA1976 ATC: AUA1976, push back and start-up approved
AUA1976 pilot: Push and start approved, AUA1976
```

You can be the pilot too. `POST /api/radio/pilot` takes a recognised call `{icao, callsign, intent, tags}`, for instance from `voice-goio`. It answers "request_taxi" with a taxi clearance from where your aircraft is to the runway in use. It checks a "readback" against the last clearance to that call sign and corrects a wrong one, and it answers anything else with "say again". It returns what ATC said.

## ATIS on its frequency

Each airport's ATIS (`nav.ATISService`, see [Weather and ATIS](nav-weather.md)) is on the radio too (#418). `ATISInformation(letter, text)` is the broadcast, on the ATIS position and so on the ATIS frequency, and it is not read back. Our pilots give the current letter on their first call of all: "Ruzyne Ground, CSA123, stand A4, information Bravo".

The airport map refreshes the ATIS of its managed airports every minute of traffic time, from the weather at the user's aircraft. A new information goes out on the radio and into the traffic log. `GET /api/radio/atis?icao=LKPR` serves the current one (letter, text, spoken form and frequency) for a voice to loop. The airport panel's 🔊 button reads it in an English voice, whatever the browser's language.

## ICAO and FAA

Where a controller is, decides how they speak (#463). `PhraseologyFor(icao)` gives the FAA in the United States and its territories (ICAO prefixes K, PA, PH, PG, TJ and TI) and ICAO everywhere else. `RadioOptions.Phraseology` overrides it.

A clearance built by `Say` is said again in FAA wording at a US airport, and `Transmission.Phraseology` records which wording was used. Readbacks follow the same wording, and the voice reads numbers and frequencies the FAA way there. The wordings are those quoted in [Phraseology](traffic-phraseology.md), so the differences there are the differences here:

| Clearance | ICAO | FAA |
|---|---|---|
| Departure clearance | "cleared to Frankfurt, BALTU 7D departure, flight planned route, runway 24, climb via SID to flight level 100, squawk 4521" | "cleared to Boston airport, KENNEDY 5 departure, then as filed, climb via SID except maintain 5000, squawk 4521" |
| Taxi | "taxi to and hold short of runway 24 via B, A" | "runway 04L, taxi via B, A" |
| Take-off | "runway 24, cleared for take-off, wind 240 degrees 8 knots" | "runway 04L, cleared for takeoff" (no wind in the civil phrase) |
| Approach | "cleared ILS approach runway 24, QNH 1013, report established" | "cleared ILS runway 04L approach" |
| Vectors (#661) | "fly heading 060, for spacing", "turn left heading 150, for base", "resume own navigation direct GOLOP"; then "turn left heading 245 to intercept, cleared ILS approach runway 24, QNH 1013, report established" | the same vectors; the approach clearance "turn left heading 245, cleared ILS runway 24 approach" (5-9-4) |
| Departure's check-in answered | "identified, climb to flight level 240" | "radar contact, climb and maintain 5000" |

On the runway, a conditional line-up ("behind the landing …, line up and wait runway 24, behind") is ICAO only. So is a conditional crossing (`ClearedCrossBehind`: "behind the landing A320, cross runway 12, behind"). The tower gives one to an aircraft holding short of a crossing when only the next arrival is in the way (`RunwayClearances.CrossBehind`). The aircraft crosses once that arrival is off the runway. The FAA does not allow conditions on the runway.

## Expedite, weather and direct

`Rushed(clearance)` gives the expedited form where one exists (#510). The aircraft then hurries: `TaxiController.Expedite` shrinks its waits at the gates, and `ArrivalController.Expedite` leaves the runway faster.

| Clearance | Expedited |
|---|---|
| Take-off | "cleared for immediate take-off" (CAP 413 4.30) |
| Line-up | "line up, be ready for immediate departure" (Doc 4444 12.3.4.10) |
| Crossing | "expedite crossing runway 12" (12.3.4.9) |
| Vacating | "expedite vacating, when vacated contact Ruzyne Ground 121.91" (12.3.4.7) |

Crews can also ask for the weather or for a shortcut:

- **Weather:** `RequestWeather` is answered by `WeatherReport`: "wind 240 degrees 8 knots, QNH 1013". The crew reads the QNH back; at a US airport the answer gives the altimeter.
- **Direct:** `RequestDirect` ("request direct GOLOP") is answered by `ClearedDirectTo` ("cleared direct to GOLOP").

No source we have read gives the wording of the two requests themselves, so it is the project's own; the answers are quoted ones.

## VFR departures and squawks

`VFRDepartureInstructions` and `VFRDepartureReadback` say a VFR departure's instructions from a controlled aerodrome, following CAP 413 (Edition 24, Figure 24, "VFR – Departure Instructions and Take-off Clearance"): "G-CD, after departure, left turn approved, climb not above altitude 2500 feet until reaching the zone boundary".

- **Wording:** the figure says "departure", not "take-off", and "approved", not "cleared", and asks for a full readback: "Left turn approved, not above altitude 2500 feet until zone boundary, G-CD".
- **Route out:** a visual reporting point is given as in CAP 413 6.7, "route via Whiskey".
- **Squawk:** added last when one is given ("squawk 7000").

Combining the route and squawk with the figure's wording is the project's own; each part is quoted.

`Squawks` hands out discrete SSR codes. No source for per-airport code banks has been found, so the scheme is simple and documented:

- **The bank:** codes from 4001 to 4777 by default (octal).
- **Which code:** the first code tried is picked from the call sign, so a flight keeps its code; then the next free one.
- **Never given:** codes in use (`Reserve` adds codes seen elsewhere, `Release` frees one), and the special codes 0000, 1200, 2000, 7000, 7500, 7600 and 7700 (`SpecialSquawk`).

## The radio panel

The airport map's **Radio** tab (#425) shows what is said on the airport's frequencies. Each frequency the scenery lists (Delivery, Ground, Tower, Approach, ATIS…) has a button with the number of transmissions heard on it. Pick the one to follow, as on a receiver: one frequency at a time (#462). Pilot lines and the ATIS are coloured apart from the controllers'. The choice is remembered.

## Voice

The airport map speaks the radio through [voice-goio](https://github.com/mrlm-net/voice-goio) (#419). The Radio tab's **🔇 Sound off** switch turns the voice on for the frequency you follow. Tuned to the ATIS, you join its continuous broadcast where it is. Each controller position has a voice and radio sound of its own, each crew its own voice, and the ATIS plays on a loop in its broadcast voice while its frequency is followed. To stay live on a busy frequency, anything not said within 20 s is dropped. The map is its own module, so the SDK keeps zero dependencies. The [airport-map README](../cmd/airport-map/README.md#voice) covers installing piper and the voice models.

## Variety

A real frequency is not a script (#721). `RadioOptions.Variety` (or `Radio.SetVariety`; nil is off) varies what is said and when, never what is cleared: every transmission keeps its intent and parameters.

- **Crew style:** each call sign gets a style from the seed: quick, normal or slow to answer (about 0.5–3 s with the breath), chatty or terse.
- **Greetings:** most crews greet on their first call to a station (chatty ones nearly always, fewer on a busy frequency), in words and places picked at random: "Ruzyne Radar, good morning, CSA1, …", "Good afternoon, Ruzyne Radar, CSA1, …", "Ruzyne Radar, CSA1, hello, …" (by the time of day, its short form, "good day", "hello").
- **Greeted back:** the controller's first answer to a first call often greets after the call sign ("CSA1, good morning, cleared …"): mostly when the crew greeted, now and then when it did not.
- **Pleasantries:** a controller may end a handoff with "good day" or "bye" when the frequency is quiet, never when it is busy. A chatty crew answers a handoff with "good day", "bye", "bye bye", "cheers" or "see you".
- **Say again:** about 2 % of clearances. The crew asks "Say again", the controller repeats the clearance (`ParamRepeat`), then the crew reads it back.
- **Readback errors:** about 1 % of clearances. The crew reads one number back wrong: frequency, heading, level, squawk, speed or altitude, never the runway. The controller corrects it with `CheckReadback` ("negative, …"), and the crew reads it back right.

The same seed and the same traffic give the same radio. The world turns it on by default (seed 721). `GET /api/radio/variety` shows it; `POST` `{"enabled":false}` turns it off; `{"enabled":true,"seed":7,"sayAgain":0.05,"readbackError":0.02}` sets it.

## Stations and controllers

An airport may have several stations per position, and one controller may work several frequencies (#722). `traffic.Station` is a position, a name, a frequency, a controller and a sector:

- **Sector:** `taxiways` (ground: by taxiway name), `area` (a polygon, an apron) or `runways` (tower). A station without a sector takes the rest of its position's traffic.
- **Controller:** stations with the same `controller` are one person. The same voice speaks on each frequency (voice-goio `Utterance.Controller`), and the radio lets one controller say one thing at a time across all of them (`RadioOptions.ControllerOf`). The default is the airport and frequency, so positions on one frequency share a controller (approach and departure on 118.31 at LKPR).

`DefaultStations` builds them from the scenery's frequencies. `StationsWith` adds an airport's own stations: a position the override gives replaces its defaults. `PickStation` picks the station that works an aircraft by its taxiways, runway and position.

In the world, each aircraft talks to the station of its sector. Taxiing into another ground station's sector, it is handed over: "CSA1, contact Ruzyne Apron 121.8". An airport's own stations live in the local settings (`stations.json` beside the other overrides), set with `PUT /api/stations?icao=LKPR` and a list of stations. `DELETE` goes back to the defaults; `GET` shows the stations as worked. A night with ground and tower combined:

```json
[{"position":"ground","name":"Ruzyne Ground","freq":"121.91","controller":"LKPR night"},
 {"position":"tower","name":"Ruzyne Tower","freq":"134.56","controller":"LKPR night"}]
```
