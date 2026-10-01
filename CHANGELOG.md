# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- Parallel runways used together. `nav.ActiveRunways` adds the parallels of the runway in use (`RunwayUse.Departures`, `Arrivals`, `Parallel`, `SpacingM`), the mode from their spacing (ICAO AN-Conf/11-IP/3):
  - segregated from 760 m: arrivals on one, departures on the other;
  - dependent from 915 m: both mixed, 2 NM diagonally between adjacent finals;
  - independent from 1035 m: both mixed.

  Crossing runways are never used together. `RunwayLimits.Parallel` sets an airport's own mode. The ATIS names every runway in use.
- Airport map: with parallels in use, a departure takes the runway nearest its stand, and an arrival the one with fewer arrivals in its sequence, with a stand near it. Dependent finals keep 2 NM diagonally (`SequencerOptions.DiagonalNM`, `ApproachAircraft.Runway`). After a runway change, only flights on a runway no longer in use move. The runway chip shows every runway in use ("26L+26R").
- Airport map: a map button shows the whole airport again, as after loading.
- Airport map: with the simulator gone, the map blurs behind a dialog: a radar scope with a plane in the hold, a rotating funny line, how long it has been gone, and a way to look at the map anyway.
- Airport map: the simulator's own cameras: cockpit, chase, drone, fixed and free, with ◀ ▶ through their views (`CAMERA STATE` and `CAMERA VIEW TYPE AND INDEX`, the values measured live). Tower is a camera mode; the add-on fixed views are no longer offered in the director. When our camera is released, the simulator's camera returns to what it was.
- Airport map: the tower can be placed on the map and its cab height set (Airport tab, saved per airport). With no aircraft selected, the tower camera looks round the airfield in a slow swing. LKPR's tower is known (`airport.Limits.Tower`); the facility puts it elsewhere.
- Airport map: the ILS of each runway end, with its frequency, loaded from the simulator. `airport.RunwayEnd.ILS` and `ILSRegion` come from the RUNWAY record, the frequency and name from the navaid record (LKPR: 24 PR 109.10). Shown in the Airport tab, and in `/api/airportinfo` as `ils`.

### Fixed

- Airport map: an airport whose layout has no parking, taxi points or runways loads ("d.parking is not iterable"), and an aircraft's entry list asks its own airport, not the one on the map (`ControlView.ICAO`).
- Website: the examples and home pages show the airport map again (they pointed to a screenshot removed in 0.18.0).
- Voice: aircraft types are read as crews say them, "Airbus A three twenty-one", not "alpha three two one" (voice-goio).
- Airport map: the airport picker lists every airport in range, each loadable with a click (the list was cut short).
- Airport map: the wind on the runway reads "tailwind 0, crosswind 6 kt" instead of "-1 / 6".

---

## [0.18.2] - 2026-10-01

Runway and sequencing safety. Take-off waits for everyone on the runway, and a cleared take-off is cancelled if the runway stops being free. Conditional line-ups are honoured. Speeds come in tens of knots. The sequencer looks ahead on the final, slowing an arrival that closes up on its leader, or breaking it off early.

### Added

- Sequencing looks ahead on the final: SequenceEntry.ShortBy shows how much sooner than its spacing an established arrival would land behind its leader. The map acts before they meet. On the STAR or downwind it slows the arrival or extends the downwind. On the final it reduces to final approach speed (ArrivalController.ReduceToFinalSpeed, "for spacing reduce to final approach speed"). Still short and already inside its spacing, more than 3 NM out, it is sent around early, not on short final.

### Fixed

- Sequencing: speeds are assigned in tens of knots ("reduce speed to 230 knots", not 239), and an arrival already flying the speed is not told it again (live, CSA1389 heard "reduce speed to 210 knots" three times).
- Runway: a take-off waits for everyone on the runway, not the last listed: a departure lining up as a landing aircraft rolled out was cleared for take-off (live, BAW1272 behind AFR558).
- Airport map: a departure told to line up behind a landing aircraft gets no other line-up or take-off clearance until that aircraft has passed (live, EZY866 was cleared for take-off 14 s after its conditional line-up).
- Airport map: a take-off clearance is cancelled ("hold position, cancel take-off, I say again, cancel take-off") when the departure is not rolling yet and the runway is no longer free, someone on it or an arrival inside the minimum; it is cleared again once free.

---

## [0.18.1] - 2026-10-01

Fixes from a live session at LKPR. The tower no longer clears a take-off ahead of a close arrival. Go-arounds fly a clean circuit back with named track points. Approach clears the approach on the base. The map uses a fraction of the CPU: standard pushbacks are planned only where needed, paced, and saved between runs. The camera gains fixed views with switching between aircraft.

### Added

- Traffic: SaveStandardPushes and LoadStandardPushes keep an airport's standard pushes between runs (refused with ErrStandardStale for another layout). The airport map saves them to the user cache folder and loads them on later starts.
- Airport map: camera views. A fixed view (chase, cockpit, wing, front, top, tower) of the selected aircraft, or of your own with none selected; ◀ ▶ switch between our aircraft, and selecting another aircraft moves the view to it (POST /api/camera {"mode":"view","view":"chase","id":7}; id -1 is your aircraft).
- Airport map: -pprof 127.0.0.1:6060 serves Go's profiler (off by default).

### Changed

- Arrivals: approach clears the approach on the base, before the turn onto the final; the crew reports established on the final and is handed to tower then (before, all three in the same second).
- Go-around: the circuit climbs to at least the approach's last altitude constraint; the crew checks in with approach "going around, climbing 4000 feet". ArrivalController.CircuitFixes names the circuit's track points (UPWIND or the missed approach's fixes, CROSSWIND, DOWNWIND, BASE, FINAL), shown on the map.

### Fixed

- Radio: the conditional line-up says the runway with the clearance, "behind the landing Airbus A320, line up and wait runway 24, behind" (was "…, runway 24, line up and wait behind"); the readback too.
- Runway: a departure at its holding point is cleared to line up and take off only with the next arrival its line-up time (RunwayControllerOptions.LineUpTime, 60 s) farther away too; it lines up behind the arrival instead. A departure cleared for take-off is no longer also given a conditional line-up.
- Airport map: an arrival removed in the air (a scene ending) is no longer handed to ground with "runway vacated".
- Airport map: on the final, once its procedure is flown, an arrival's line runs to the threshold and down the runway.
- Airport map: a departure checks in with tower "taxiing to runway 24" ("at Z" for an intersection).
- Airport map: the radio log shows the newest call first.
- Arrivals: a delay absorbed on a STAR or a go-around's circuit re-plans from the procedure's corners, not from its already rounded turns: a downwind extended no longer cuts into a turn's arc, and no dog-leg loops are drawn (live, DLH1402 flew loops after its go-around).
- Airport map: the radio tab fills the panel and only its log scrolls (two scrollbars before).
- Airport map: CPU. Every airport loaded, flight-plan destinations included, planned its stands' standard pushbacks at once, in parallel: 1–2.5 min of a core each, 6 cores at peak. Now only airports where a departure appears, one at a time, resting after each stand (about 30% of one core), and planned once per layout: saved and loaded on later starts. The push geometry is cheaper too (no trigonometry per point of a push path): planning takes about half the time.
- Airport map: the sequence ladder's distances carry their unit (NM); the final no longer labels its arrivals (click the aircraft for its details; the dot shows whether the spacing is kept).
- Airport map: dropdown lists (the position picker) take the theme's colours in dark mode.

---

## [0.18.0] - 2026-10-01

The airport map becomes an app: a new interface for desktop, tablet and phone, played over the network one position per device, with a camera director and scripted films. The radio covers the flight gate to gate, in ICAO wording or the FAA's at US airports, with expedited clearances and crew requests. Landings crab into the crosswind and vary their touchdown; a runway change re-plans the traffic; pushbacks follow a standard per stand; EDDM, LOWW and EGLL are validated with their AIP limits.

### Added

- A departure's runway entry can be changed on the stand or while taxiing (`TaxiController.ChangeEntry`; the Entry choice in the aircraft panel, with the entries too short for the type marked); taxiing, ground says the new route.
- The MSFS 2024 add-on camera (#515): `engine` binds `CameraAcquire`, `CameraSet`, `CameraGet`, `CameraRelease`, the world locker and the rest (`types.SIMCONNECT_DATA_CAMERA`, packed); `pkg/camera` has poses relative to the world, an aircraft or the eyepoint, eased `Move` and spline `Path` shots, drone moves scaled to the aircraft (reveal rise, flyover, spiral descend, lead chase, parallax track, side dolly, head-on pass, top orbit, details of the engines, gear, cockpit, tail and lights) and a `Director` that plays them. Conventions measured live are in `docs/camera.md`.
- Airport map camera: auto director (cuts to the aircraft on the radio as the call is heard, wide and detail shots for its phase), follow the selected aircraft, and scripted scenes (`-scenes`, JSON: cast, beats, cues such as `dep:pushback` or `dep:lights:B`, the radio following the aircraft that matters) with three to start from.
- Airport map: pause/resume and simulation rate on the map; the sound output picker remembered; "Tune my COM1" (a frequency picked on the map tunes COM1 without following it); pushback facing as a select (auto by default); "Continue taxi" after hold position; "Cleared to land" only on final; buttons disabled while a command is on its way; a wider panel on large screens; controllers change voice at a shift change every 30–60 minutes.

- `pkg/traffic` standard pushback per stand: `PlanStandardPushes(graph, model, stands)` plans, in the background, the push most ends of the two longest runways take from each stand; a departure takes it whatever its runway unless it costs more than `standardPushMargin` extra (LKPR B9 pushes onto B2 for every runway). The airport map plans them when an airport loads.
- `pkg/traffic` landings vary: each injected landing moves its aiming point by up to `TouchdownSpreadMeters` (±10 m) (`ApproachMover.SetAimShift`), and in a crosswind the upwind wing goes down in the flare as the crab comes out, the upwind main gear touching first, levelled within 1.5 s (`ApproachPose.BankDeg`, at most 4°).
- `pkg/traffic` crosswind landings: `ApproachMover.SetCrosswind` / `ArrivalRequest.CrosswindKts` (positive from the right): the injected final is flown crabbed into the wind by the drift angle and straightened through the flare to touch down along the centreline. The airport map passes the wind at the user aircraft.
- docs: traffic-decisions.md — how the traffic decides, with the numbers from the code and diagrams: pushback choice (poses, push geometry, cost terms, push-and-tow, second look, standard push, worked examples LKPR A3, A5, B9, C17), ground give-way, taxi routing costs, the runway controller, spacing and sequencing, delay absorption, holds and conflict resolution, #449.
- Airport validation at EDDM, LOWW and EGLL (#376): `pkg/airport` `TestValidateAirportLayouts` (with LKPR as the reference) checks for every stand and runway end that the layout builds, every runway end has named entries and exits and hold-shorts, every stand reaches every runway end near its threshold and every exit reaches every stand, with every runway crossing between hold-shorts; `pkg/traffic` `TestValidatePushPoses` (every stand × runway end pushed to a pose), `TestValidateInjectedDepartures`, `TestValidateInjectedArrivals` (samples end to end, a crossing hold for each runway crossed) and `TestValidateArrivalExits` (the chosen exit never leads back across the runway vacated).
- `airport.KnownLimits` for EDDM, LOWW and EGLL from their AIPs, sourced in the code (#376): initial climb (EDDM FL70, LOWW 5000 ft, EGLL 6000 ft), no reverse beyond idle (EDDM, LOWW), preferential runways 27R/27L at EGLL; transition altitudes checked (5000, 10000, 6000 ft).
- `pkg/traffic` radio, the gate-to-gate flow (#462): `Identified` answers a departure's check-in ("identified, climb to flight level 240"); `ClearedApproachTo` with the QNH and "report established"; the arrival clearance with the QNH; pilots ask for the weather (`RequestWeather` → `WeatherReport`: wind and QNH) and to fly direct (`RequestDirect` → `ClearedDirectTo`).
- `pkg/traffic` phraseology by region (#463): `Transmission.Phraseology`, `PhraseologyFor` (FAA in the US and its territories, ICAO elsewhere) and `RadioOptions.Phraseology`; at a US airport the radio says the departure clearance ("then as filed", "climb via SID except maintain"), taxi ("runway 04L, taxi via B, A"), line-up, take-off ("takeoff", no wind), landing, approach and arrival clearances, radar contact and the weather the FAA way, and the readbacks follow.
- `pkg/traffic` expedite (#510): `Rushed(clearance)` says "cleared for immediate take-off", "line up, be ready for immediate departure", "expedite crossing", "expedite vacating"; `TaxiController.Expedite` shrinks the waits at the gates (`RushDelayFactor`), `ArrivalController.Expedite` leaves the runway `RushExitKts` faster.
- Airport map radio, gate to gate (#462): departure answers the climb check-in ("identified, climb to flight level 240"); approach clears the approach with the QNH and "report established", the crew reports "localizer established runway 24" before the tower; the QNH in the arrival clearance; now and then a crew asks for the weather and gets the wind and QNH (`EstablishedReport`).
- Airport map rush (#510): `POST /api/control/{id}/rush?on=1` has the crew hurry; its clearances become the expedited ones (immediate take-off, be ready for immediate departure, expedite crossing, expedite vacating).
- Airport map network play, server side (#511): start it with `-addr :8080` and open it on other devices; a client working one position (`?as=ground` or `X-ATC-Position`) may clear only the aircraft on that position's frequency; `GET /api/voice/clip` gives any transmission as a WAV in the server's voice, so each device can play the radio itself. The voice reads FAA transmissions the FAA way.
- `pkg/traffic` `ContinueTaxi` ("CSA1, continue taxi") after hold position.

### Changed

- The airport map has a new interface (`cmd/airport-map/web/`): a status strip (airport, runway, ATIS, wind, pause and rate, score, camera, frequency, network position, connection), the selected aircraft in its own panel with the next clearance first and the urgent ones always in place, sections for Traffic, Sequence, Schedule, Radio, Airport and Map, light/dark/system themes on a flat palette, and a layout for tablets and phones (bottom sheet, 44 px touch targets). The previous page stays at `/classic` for now. Network play in the UI (#511): the position this device works, Play on this device; Rush per aircraft (#510). `GET /api/status` tells a connected simulator also in its menu.
- The airport map moved from `examples/airport-map` to its own entry point, `cmd/airport-map` (module `github.com/mrlm-net/simconnect/cmd/airport-map`): it is the traffic control app and the SDK's debugger, not an example. Run it with `cd cmd/airport-map && go run .`.
- Hold position is read back as given: "Hold position, CSA1" (the project's choice over Doc 4444's "holding").

### Fixed

- Camera: after every cut the new shot holds still for `camera.CutSettle` (0.6 s) while the simulator settles on the new view, then moves; the frame event is on only while the camera is in use.
- Camera moves are slower: the auto director's shots last 9.5 s (wide) and 6 s (details), scene shots 30 % longer, and the spiral, top orbit and parallax track sweep less.
- Airport map camera: set on every rendered frame (the simulator's Frame event) instead of a timer, so its moves no longer blip.
- Airport map: the first clearance you give an aircraft takes it over (Manual), so automatic answers and tower clearances no longer clash with your clicks; the Manual toggle hands it back, a waiting request is then answered.
- Airport map: pushback and start-up in one clearance again ("with start-up" by the pushback, said "pushback and start up approved, facing …").
- Airport map: on the map's own computer, its voice and "Play on this device" exclude each other (no echo); the network address other devices open the map on is shown in the quick reference and the status menu (`GET /api/status`: `network`).
- Runway exits and entries named at airports whose connectors to the runway are unnamed paths (#376; LOWW: every exit and entry was ""): the name is taken from the taxiway the connector leads onto, followed straight on for up to 300 m.
- Reduced 2.5 NM spacing on final applies only where the radar minimum governs: a RECAT-EU pair whose 3 NM is a wake minimum (A behind A, C behind C or D …) keeps it.
- docs/traffic-taxi.md: `TrafficLookMeters` is 200 m, the default tug is `FSDT_Pushback_03`, and the tightest push radius tried is 17 m.
- Pushback: a taxi-out turning back right after the push is also caught at the first node from the nose (LKPR B9 for 24 faced east 23 m short of B2's junction, then looped 130 m round onto B1); such a push gets a second look with a larger search, after push-and-tow.
- A departure cleared for take-off while taxiing does not stop at its holding point or report holding short: it rolls onto the runway at its speed (traffic ahead still stops it).

---

## [0.17.1] - 2026-10-01

### Added

- `pkg/traffic` pushback facing a compass direction: `TaxiController.ClearPushbackFacing("east")` plans the push again among those ending within 45° of it; `PushFacing`, `CompassHeading`, `CompassName`; `WithFacing` says it ("pushback approved, facing east"). The airport map has N/E/S/W buttons beside Pushback and says the facing with every pushback.
- `pkg/traffic` `ClearedApproach` ("cleared ILS approach runway 24"): the airport map's approach clears each arrival for its approach before handing it to tower, and the arrival checks in "established ILS runway 24".
- `examples/airport-map`: "Cleared to land" for the arrivals you control; the 📻 frequency on an aircraft's card tunes the radio to it; the frequency buttons count the aircraft on each, not the calls.

### Changed

- "Holding point" is gone from the phraseology: `ReadyForDeparture(cs, runway, entry)` says "holding short runway 24 [at Z], ready for departure" (`HoldingShortSaid`).
- Airport map: a taxiing departure is handed to tower 300–700 m before its runway, checking in "taxiing to holding short runway 24", and calls ready once there; it goes to departure between 1000 and 2500 ft, a different height for each.
- Airport map: 1 to 5 s at random between transmissions, and before a controller answers.
- A stand the aircraft taxis straight out of asks for start-up, then taxi: no pushback offered (`TaxiController.FacesOut`).

### Fixed

- Airport map: a take-off clearance from the holding point is no longer followed by "line up and wait"; a taxi clearance on the stand by "pushback approved", one up to a limit by the full taxi clearance.
- Airport map: the remove button sits with the other actions.

## [0.17.0] - 2026-10-01

Radio and voice: ATC speaks in structured ICAO phrases on each position's frequency, pilots call, request and read back, the ATIS broadcasts on its own frequency, and the airport map says it all aloud through voice-goio. Pushbacks end at a planned pose on the taxiway, engines start after the tug has gone, and a change of the runway in use re-plans the traffic.

### Added

- `pkg/traffic` pushback to a target pose (#491): the push is planned to a point and facing on a taxiway (`pushpose.go`): Dubins push fitted to the pavement, clear of stands and other aircraft, empty neighbouring stands usable (`TaxiRequest.StandOccupied`); push-and-tow only where a push alone would end misaligned or in a hairpin.
- `pkg/traffic` engines off on the stand (#502): an injected departure starts its engines once the tug has gone (`EngineStartTime` each); the crew asks for start-up (`TaxiEvent.Request` "start_up", `ClearStartUp`) or for pushback and start-up together (`RequestPushbackAndStartUp`, `ClearedPushbackAndStartUp`); a taxi clearance implies the start-up.
- `pkg/traffic` runway change (#456): `TaxiController.ChangeRunway` re-routes a departure not yet lining up (a new push from the stand, or a taxi route from where it is) with its new SID; `ArrivalController.ChangeRunway` gives an arrival not on the injected final the new runway's procedure and plan; `RunwayChange` says it ("runway change, runway 06 in use, VOZ 2D departure"). The airport map re-clears its traffic when the runway in use changes.
- `pkg/traffic` ground give-way (#503): `GiveWay` ("give way to the A320 passing left to right"), `TaxiEvent.GivingWayTo`/`ArrivalEvent.GivingWayTo`; crossings report "holding short of runway 12 at F" (`HoldingShortReport`); a departure clearance without a SID says "climb to".
- `pkg/traffic` take-off and landing step timelines (#497): `TaxiController.Sequence`, `ArrivalController.Sequence` (`SequenceStep`).
- `examples/airport-map`: voice through voice-goio from installed voices only (#496); the voice follows a frequency change at once, and "follow my COM1" both ways (#499); hide the planned route (Esc) and pick the call sign when spawning (#498); start-up action; the Charts tab is now Airport (#506).
- docs: Phraseology — ICAO and FAA side by side for the gate-to-gate IFR flow, every phrase quoted from Doc 4444, CAP 413, JO 7110.65 or the AIM with its paragraph (#462).
- Radio phraseology to that reference (#462): the full departure clearance (`DepartureClearance`: destination, SID by its fix, runway, initial climb, squawk; `RequestClearance`), start-up and pushback as two approvals (`ClearedStartUp`, `RequestStartUp`), the landing clearance (`ClearedToLand`, `RunwayClearances.Land`), wind in take-off and landing clearances (`WindSaid`), `WhenVacatedContact`, levels against the transition altitude (`LevelSaidAbove`), "reduce speed to", reasons as "due traffic", one runway designator in a crossing, readbacks as the reference has them. The airport map: the handoff to departure once airborne with the passing/cleared level and SID, stations named for the frequency found and from the AIP (LKPR: Ruzyne Radar).
- Requests and clearances in radio order (#462): `TaxiEvent.Request` (the crew asks when ready), `Radio.ClearAt`; `RequestPushback` takes the station (first call). The airport map answers requests after a pause and acts after the readback; tower clearances too.
- Airport map: voice (#419). The radio is spoken through voice-goio: a voice per position, each crew its own, the ATIS on a loop; a sound switch on the Radio tab follows the frequency picked. The airport panel reads the ATIS through it. `GET`/`POST /api/voice`, `POST /api/voice/atis`, flags `-piper` and `-voices`. The map is now its own module (`cd examples/airport-map && go run .`).
- Airport map: a Radio tab (#425) with the airport's frequencies, each with what is said on it; follow one frequency or all.
- ATIS on its frequency (#418): `pkg/traffic` `ATISInformation`, the broadcast on the ATIS position. The airport map refreshes the ATIS every minute of traffic time, broadcasts a new information, and has our pilots give the letter on their first call. `GET /api/radio/atis` serves the current one for a voice to loop. The airport panel reads the ATIS in an English voice.
- `pkg/traffic` the pilot side (#417):
  - requests and reports (`RequestPushback`, `RequestTaxi`, `ReadyForDeparture`, `Vacated`), the first call on a frequency (`CheckIn`), `SayAgain`;
  - `Readback` of every clearance, the ICAO way, and `RadioOptions.ReadBack` has our pilots read back on the frequency;
  - `CheckReadback` corrects a wrong readback ("negative, …");
  - `Radio.Transmit` returns the transmission as sent;
  - on the airport map, the log reads as a conversation, and `POST /api/radio/pilot` lets the user be the pilot (request taxi, readback check, say again).
- Frequencies and handoffs (#416):
  - `pkg/airport`: the loader reads the airport's frequencies (`Layout.Frequencies`, `FrequencyFor` with ATC's fallbacks, `FormatMHz`).
  - `pkg/traffic`: `DeparturePosition` and `ArrivalPosition` give who works an aircraft in each state; `Handoff` gives "contact Praha Tower 118.105" (`StationName`, `PositionName`).
  - The `Radio` puts each transmission on its position's frequency (`RadioOptions.FrequencyOf`) and says one at a time on each (`SpeakingTime`).
  - The airport map hands aircraft from position to position and shows who works each and on what frequency.
- `pkg/traffic` transmissions (#415): what ATC says as a `Transmission` (position, call sign, intent, parameters, and the text as said), built by one phrasebook (`Say`) with a builder per clearance, and carried by a `Radio` (stamped, kept, `OnTransmission`, `Recent`). The airport map's ATC log comes from its radio with unchanged wording, and `GET /api/radio` serves it. See `docs/traffic-radio.md`.
- `pkg/traffic` `SimClock` (#413): traffic time at the simulation rate, stopped while paused (`SetRate`, `SetPaused`); `TaxiWithClock`, `ArrivalWithClock`; injected motion steps at most `MaxFrameStepSeconds` (1 s) a frame. The airport map runs all its traffic on it, fed by `SIMULATION RATE` and the "Pause" event, and shows the rate.

### Changed

- Taxi clearance: "taxi to and hold short of runway 24 [at B] via H, A" (#501).
- Take-off and landing closer to real procedures (#497): acceleration altitude 1000 ft, flaps up by speed, hand-over once clean; landing flaps by 1400 ft, strobes and landing lights off on vacating.
- Stands are picked at random among the nearly best, not always the same gate (#500).

### Fixed

- Ground: braking behind traffic starts earlier and is smooth (#503).
- Tug: drives off without jumping a wheelbase as it turns away (#504).
- A spawned departure with a flight plan asks delivery for its clearance (#503).
- Pushback: an alley push counts the crossroads of lanes it passes and ends on, whatever the lanes are called; LKPR A5 for 24 no longer pushes 134 m into the B1 crossroads (#489).
- Radio flow (#462): pushback first (the first call to ground), start-up with the push under way; the delivery exchange paced (request, clearance, readback, "readback correct", transfer to ground) and the crew's requests after it; arrivals' first call then their clearance; "flight planned route" in the departure clearance; the initial climb FL100 by default, per airport (`Limits.InitialClimbFt`) and per SID (`Limits.InitialClimbs`, `InitialClimbFor`). The radio follows one frequency; the voice shortens pauses rather than dropping calls (60 s), and tuned to the ATIS joins its continuous broadcast where it is.
- Airport map: conflicts between our arrivals on their STARs are resolved: the one landing later loses time (speed, then a dog-leg), said on the frequency, and holds if the conflict is still predicted 90 s on (#455; before, only en route aircraft were steered).
- Turnarounds: the arrival's stand passes to the departure (`StandAllocator.Transfer`) instead of being released and taken again, which failed with ErrStandTaken against the parked aircraft itself; on the airport map turnaround departures never spawned (#470).
- Airport map: an overflight appears where its plan enters the area, moved on by the time since its entry time (not by its STD along the plan: RYR1850 appeared 230 NM out); a plan that never enters the area is not spawned (#469).
- An arrival whose reserved stand is taken by other traffic before it lands goes to another stand: `StandAllocator.TakenFrom`, `ArrivalController.ChangeStand`; the airport map re-checks every 10 s (#479).
- Overflights cross the area along their great circle, not a straight line in latitude and longitude (`calc.IntermediatePoint`): no Dublin–Seoul over Prague (#468).
- Ground: a push under way no longer stops for an aircraft giving way to it (the wing clearance of #446 applies before a push starts only), and a finished push shows its planned taxi in the same frame: no mutual wait (#466).
- `nav.RunwaySelector` chooses a runway only 2 kt within its wind limits (`RunwayChoiceMarginKts`) and keeps it up to the limits: no runway chosen at its tailwind limit and dropped at the next gust (#460).
- Airport map: the schedule waits for the first weather sample (at most 30 s) before spawning, so the first flights do not take the preferred runway when the wind says the other (#458).
- ATIS: the runway in use is kept through wind shifts near a limit (`RunwaySelector`), and on the airport map it is the traffic's own (`ATISWithSelector`): no new letter and runway every minute (#454).
- Pushback: an alley push counts the junctions of other taxiways it passes, and a taxi-out turning back sharply right after the push costs more (#441; LKPR A3 no longer tows 190 m along Z, A5 faces its way out).
- Ground: facing oncoming traffic, an aircraft keeps the junction before it clear, so the other can turn off there (#444).
- Pushback: a moving aircraft (pushing, taxiing) must be clear of the push corridor by both half-spans, not only its fuselage (#446).
- Ground: an aircraft waiting for its taxi clearance shows its planned way, so a neighbour does not push into it; beside a push under way only an aircraft the push stops for goes on (#452).
- `pkg/traffic`: a pushback does not leave the nose facing back at the stand (#436). A branch less than 45° off the straight push is not taken across a named taxiway behind the stand; along an unnamed lead-in it still is. Live, E190s at LKPR A4 were pushed straight across B1 and faced the dead-end lead-in.
- `pkg/airport`: runway 24 at LKPR lists entry Z (#433). The search for the ways off a runway bounded the whole path, including the edge leaving the surface. Z leaves A's long lead-in at the runway edge 137 m from its first node off the runway, so it was cut and merged into A. The bound is now on the way across the surface only.
- `pkg/traffic`: a pushback does not leave the aircraft blocking other taxiways (#429). Each junction of another taxiway it would sit on costs 400 m in the choice. A wider swing, up to 125°, is a fallback where no ordinary push is clear. Live, RYR1455 pushed from LKPR A4 stood across H; over all LKPR stands, pushes ending on another taxiway went from 31 of 103 to 9.

---

## [0.16.0] - 2026-09-30

Airborne ATC: traffic separated in the air as well as on the ground. Wake separation and spacing on final follow the weather. There is a landing sequence per runway. Arrivals lose their delays by speed, a longer downwind and holding stacks. A tower per runway clears line-ups, take-offs and crossings in mixed mode and sends arrivals around. Conflicts in the air are predicted and resolved. Turns follow the airframe's standard bank, and the map has an Approach tab to work it all by hand.

### Added

- `pkg/traffic` wake turbulence separation (#389): `WakeFor` (ICAO L/M/H/J and RECAT-EU A–F by type, else by span), `ArrivalSeparationNM` (ICAO Doc 4444 and RECAT-EU minima on final, at least `MinRadarSeparationNM`), `SeparationTime`, `DepartureInterval` (wake and same-route intervals), `RunwayOccupancy`. See `docs/traffic-separation.md`.
- `pkg/traffic` `ApproachSequencer` (#390): the landing sequence of a runway, with predicted and sequenced landing times, wake spacing and runway occupancy, and each arrival's delay. Established arrivals (`FreezeNM`) and other traffic are fixed; `OnChange` reports changes. `DistanceToGo`.
- `examples/airport-map`: landing sequences per airport and runway, fed with our arrivals (controlled and en route) and respected other traffic; changes in the traffic log; `GET /api/sequence`.
- `pkg/traffic` spacing follows the weather:
  - `ApproachConditions` and `ConditionsFrom` (visibility, ceiling, headwind on final, runway dry, wet or contaminated);
  - `ArrivalSpacing`: reduced 2.5 NM only in good conditions on a dry runway, +1 NM contaminated, at least 6 NM in low visibility procedures;
  - `RunwayOccupancyIn` and `FinalGroundKts`;
  - the sequencer's `SetConditions`, `AllowReduced`, `TimeBased` and `SpacingWhy`;
  - `ManagerOptions.Conditions` spaces arrival spawns likewise;
  - the map feeds the weather at the user aircraft.
- `pkg/traffic` delay absorption (#391):
  - `ArrivalController.AbsorbDelay` slows an arrival on its STAR down to `MinProcedureSpeedKts`, then adds a dog-leg of at most `MaxStretchNM`, and returns what is left for the hold; the final is never changed;
  - `PlanAbsorption`, `StretchLeg`, `ProcedureRoute`;
  - the map has arrivals absorb their sequencer delays and logs it as ATC would.
- `pkg/nav` `RunwaySelector`: the runway in use holds through wind shifts. It changes when out of limits (gusts included), or when another has been better for `RunwayChangeAfter` (10 min). The map uses it for traffic and the Charts panel.
- `pkg/traffic` holding patterns (#392), our own (the sim's data is unusable):
  - `Hold` with ICAO leg times and speeds, rate-one turns and the direct/teardrop/parallel entry by heading (`Entry`, `EntryPoints`, `Racetrack`);
  - flown as a waypoint chain that wraps once the entry lap is done;
  - `HoldStack`: 1000 ft levels, leaving from the bottom, those above stepping down;
  - `ArrivalController.HoldFix`, `EnterHold`, `HoldAltitude`, `LeaveHold`, `Holding`;
  - on the map, arrivals hold with what speed and stretching cannot absorb, and are released by the sequencer.
- `examples/airport-map`: a selected arrival in the air shows the route it still flies (dashed, with any dog-leg) and its hold with the level. 🎯 follows an aircraft: the map keeps it in the middle. `/api/control` has `airRoute` and `hold`.
- `pkg/traffic` `RunwayController` (#393): take-off, line-up and crossing clearances by the runway free, the wake/route interval after the last departure, and the next arrival far enough out (mixed mode). First come first served, and `Waiting` reasons. `TaxiRequest.HoldForRunway`: pushback and taxi go by themselves, and the runway gates wait for clearances. The map runs a tower per runway (`GET /api/runways`).
- `pkg/traffic` `ModelsForFlight`: equally good titles for a flight's airline and type are taken in turn by call sign, so a fleet shows its liveries (each flight keeps its own). `ModelsFor` puts titles carrying the airline's ICAO code before those matching only its name (a sister airline: TVS before "TVP-Smartwings Poland"). The A220-300 (`BCS3`) is a known type; Czech Airlines flies A320s and A220s instead of ATR 72s.
- `examples/airport-map`: our aircraft are coloured by what they are — under our control (a card), arriving en route, overflying, departed — with a legend, labels and popups saying so.
- `pkg/traffic` `AirborneSeparation` and the minima (`TerminalSeparationNM`, `EnrouteSeparationNM`, `VerticalSeparationFt`); the sequencer's `MinSpacingNM`. The map keeps 5 NM (sequencers at 5 NM, spawns 6 NM clear) and logs every pair under 5 NM and 1000 ft (`GET /api/separation`).
- Documentation: `docs/examples.md` makes the airport map the main example and debugging tool, with a tour and screenshots, and lists every other example; the README, getting started, the website's landing and examples pages lead with the map, and the `examples/airport-map` README covers the current tabs and HTTP API.

- `pkg/traffic` automatic go-around (#394):
  - `RunwayController` sends the next arrival around when it is `GoAroundAt` (30 s) out and the runway is not free (someone lined up, crossing, still on it after landing, or other traffic on it), and lists it in `GoAround` with the reason;
  - `GoAround` flies the published missed approach when `ArrivalRequest.MissedApproach` has one, else the circuit;
  - `ApproachSequencer.Rejoin` sequences the go-around afresh;
  - on the map, the tower sends our arrivals around, and both the tower and the button re-sequence them; our aircraft taxiing across a runway count as on it.

- `pkg/traffic` airborne conflicts (#395): `PredictConflicts` (a pair losing 5 NM, or 3 NM in a terminal area, and 1000 ft within a 5 min look-ahead, flying on as it is), `ResolveConflict` (the least disturbing speed, level or heading change to one of ours that keeps it clear of everyone; other traffic is never steered) and `ResolvedRoute`. The map resolves conflicts for our en-route aircraft and logs them as ATC; `/api/separation` has `conflicts` and `resolutions`.

- `examples/airport-map` approach view and controls (#396). The **Approach** tab has the landing sequence per runway with ▲▼ order, ⤳ direct, 🐢 slow, ⟳ hold, ⏵ leave and ↺ go around (`POST /api/approach/{icao}/{callsign}/{action}`), the tower and the conflicts. The final's spacing is drawn on the map, green kept, red short. The game costs 25 for spacing on final below the minimum. `pkg/traffic` adds `ApproachSequencer.Move` (`ErrEstablished`, `ErrNotSequenced`) and `ArrivalController.DirectToJoin`.

### Fixed

- `examples/airport-map`: an air route is drawn smooth. The few waypoints of a rounded turn showed as corners in the dashed line.
- `examples/airport-map`: an arrival on the final is sequenced by its straight distance to the threshold. The planned approach, measured from its nearest point, put TST2 2 NM further out than it was, which would have spaced the one behind it wrongly.
- `pkg/traffic` `AbsorbDelay`: after slowing down, a delay is lost the way a controller would: a longer downwind, going on along it past the STAR's last point and joining the final that much further out, once an approach. Only a STAR without a downwind (straight in) gets a dog-leg. Live, TST1 flew a 4.9 NM dog-leg that looked like an artifact. Rounding no longer rounds a rounded chain's arc points again.
- `examples/airport-map`: a selected aircraft's air route has dots, with names, only at its procedure's fixes still ahead (`airFixes`). The dashed line runs through the points of the rounded turns too, which are not fixes.
- `pkg/airport` `Route.SpokenTaxiways`: a taxi clearance names the taxiways as a controller would, leaving out stubs under `SpokenMinMeters` (150 m) that only lead onto the next one. At LKPR from N58 "via H, L, G, F" (270 m of three stubs curving onto F) is now "via F".
- `examples/airport-map`: a second aircraft with a call sign already flying is refused (a turnaround still adopts its own). It would have shared its stand reservation.
- `pkg/traffic`: an aircraft beside a pushback under way waits where it is unless it is already in the push corridor. At LKPR DLH977, waiting at the edge of TVS1960's push, drove into it; each then stopped for the other for 8 minutes and they finished too close.
- `pkg/traffic`: an injected take-off builds up as the engines spool from idle to take-off thrust (`TakeoffProfile.SpoolSeconds`, 7 s), instead of full acceleration from the first frame.
- `examples/airport-map`: a departure handed to MSFS AI shows the SID it still flies when selected, as an arrival shows its STAR (`TaxiController.ClimbRoute`). The Approach panel's message no longer shares its id with the Charts panel's airport info.
- `pkg/traffic`: turns in the air are an airliner's. The corners of our waypoint chains (STAR and approach, go-around circuit, delay absorption, en route, departure) are rounded into fly-by arcs of the aircraft's standard turn: rate one (3°/s), at most `MaxBankDeg` for its airframe (25° jets, 30° turboprops). That is about 1 NM at 180 kt and 2 NM at 250 kt for a jet (`StandardBankDeg`). MSFS AI turned at each point, late and hard (live, up to 6.5°/s, some 45° of bank). A hold at a rounded corner is at the STAR fix itself.
- `pkg/traffic`: no height step when the injected final takes over from MSFS AI. The blend is measured from the aircraft's MSL altitude (`PLANE ALTITUDE` in the arrival monitor) instead of the ground the injector last saw under it. Live, a 341 ft step at the join.
- `pkg/traffic`: the picture measures our aircraft's vertical speed from their altitude between scans. An injected aircraft's is meaningless (live, +560 fpm descending on the glide path), which misled the conflict prediction.
- `examples/airport-map`: aircraft markers glide between the one-second updates, dead-reckoned on heading and speed, instead of stepping 90 m at a time at 180 kt.
- `pkg/traffic`: a pushback tug is moved at every frame while it drives (in, pushing, backing off and away), wherever it is. After the push the aircraft stands still, and its level of detail dropped to a still aircraft's rate, so the tug's drive-off stuttered.
- `pkg/traffic`: a departure just airborne ahead of an arrival on final at the same airport is the tower's runway separation, not a loss (`TowerPair`, `TowerBelowFt` 2500 ft). Live, AFR1059 taking off 4.2 NM ahead of AFR554 on final was logged as a loss. The same call sign twice (an enroute aircraft handed over to a new object) is not a pair.
- `examples/airport-map`: a tower wait is logged when it changes, not every second as its countdown runs.
- `pkg/traffic`: a go-around flies its missed approach and circuit before it joins the final again. Climbing out along the centreline it looked established and was taken straight back onto the final: live, TVS1986, sent around at 3 NM, landed.

- `examples/airport-map`, `pkg/traffic`: a go-around is sequenced by the circuit it still flies. The map measured every arrival along its planned approach from its nearest point, ignoring dog-legs and holds too. The controller's next waypoint was the nearest one, which on a circuit looping back past the final is the wrong one. Live, an arrival that had just gone around stayed number 1 with 4 NM to go. The map now uses `ProcedureRoute` with `DistanceVia`, and the controller tracks its waypoints forward.
- `pkg/airport`: entries onto a runway may turn up to `MaxEntryAngle` (135°) — threshold entries often meet the runway square or slightly back (LKPR 12 at L, 120°), which the 90° exit limit left out. A taxiway dead end on another node (within 3 m) is joined to it: at LKPR the F lead-in ends on the 06 centreline beside the runway node without sharing it, so 06 had no full-length entry.
- `pkg/traffic`: an injected line-up follows the painted lead-in from the hold-short onto the runway also where no listed entry starts (a breadth-first walk of the taxi graph to the centreline), instead of turning straight at the runway.
- `pkg/traffic`: injected approaches fly the glide path over the runway elevation, not the terrain below; over hills and valleys the aircraft had bumped up and down before the threshold. The last 100 ft blend to the ground.
- `pkg/traffic`: a pushback waits while a neighbour's pushback under way sweeps its corridor. At LKPR A1 and A3 pushed at once and each stopped for the other's body for good.
- `examples/airport-map`: a departure's stand is freed once it taxis, not when its pushback starts; a push held for traffic had new departures spawned on top of it (two aircraft on A1 and A3).
- `examples/airport-map`: a tower clearance is logged once (a take-off clearance is also the line-up).
- `pkg/traffic`: the manager reports a flight delayed again only when its retry moves by a minute or its reason changes.

- `pkg/traffic` `ApproachSequencer`: first come, first served by the unconstrained time. Each arrival keeps the prediction it had when it joined, so losing a delay (slower, longer, holding) no longer costs it its place to a newcomer. Live, four newcomers had been sequenced ahead of eight delayed arrivals.
- `examples/airport-map`: nobody appears on top of other traffic. Every arrival spawned at a STAR entry (by hand, scheduled or handed over from en route) waits while an airborne aircraft is within 5 NM and 2000 ft of it, or one appeared there in the last minute. Live, arrivals spawned by hand at one fix within seconds had flown on top of each other.
- `pkg/traffic` `ApproachSequencer`: arrivals already in the sequence keep their order unless their predictions part by more than `SwapMargin` (90 s), and never land before the one ahead. Live, three arrivals appearing together swapped places every second.
- `pkg/nav` tests: an import cycle (nav tests → manager → traffic → nav, since `traffic.ConditionsFrom`) broke them. The manager check moves to an external test package.
- `examples/airport-map`: an aircraft can be deselected (click its card again, or Esc); nothing selects one back by itself, and removing the selected one leaves none selected.
- `pkg/traffic`: a flight's estimate moves by whole minutes; the manager no longer reported the same estimate again every tick as the prediction drifted by seconds.
- `pkg/traffic` `CheckLandingFlow`: with departures waiting, one gap is opened in the arrival stream instead of doubling every gap; at density 2 the doubled gaps had pushed estimates hours out.

### Known issues

- #395's long-run criterion (no two of our aircraft below the minima over a long busy run) was checked only on light night traffic at LKPR, with no losses.
- Traffic runs on wall-clock time: a simulation rate other than 1× or a pause puts it out of step (#413).
- Conflict resolutions steer only our en-route aircraft; our arrivals and departures near the airport are kept apart by the sequencer and the tower.

## [0.15.0] - 2026-09-29

The whole traffic picture: every aircraft around a centre of the world, timetables for its airports, a traffic manager that turns them into traffic and adjusts to what it sees, enroute traffic and overflights, dozens of aircraft at once, and a world view on the map.

### Added

- `pkg/traffic` `TrafficPicture` (#366): all traffic around a configurable centre of the world (an airport, a position, or following the user with `RecentreNM` hysteresis; radius `DefaultPictureRadiusNM` 250 NM). It tracks ours, MSFS AI and the user with phase and airport, knows the airports in range (`AirportLister`), and sends enter/leave/recentre events. It keeps one `GroundPicture` per airport (`Ground`) and feeds `StandAllocator`s (`Allocate`) from a single scan. See `docs/traffic-picture.md`.
- `pkg/traffic` `Schedule` (#367): scheduled flights (call sign, airline, type, origin, destination, STD/STA) for the focus airports in a time window. Airlines have fleets, bases and regions; home carriers (bases, or the airlines the stands name) get most of the traffic; the type fits the distance and both runways; movements follow time-of-day waves by local solar time, scaled by airport size and `Density`. Deterministic for a seed. `DefaultScheduleConfig` is built in (17 European and long-haul airlines, about 80 airports); `SaveScheduleConfig`/`LoadScheduleConfig` export and read it as JSON to edit. See `docs/traffic-schedules.md`.
- `examples/airport-map`: the traffic picture in Layers (centre, radius, airports and aircraft by phase); ICAO fields are dropdowns of the airports in range; the aircraft scan reaches SimConnect's 200 km maximum; `GET/POST /api/world`
- `pkg/traffic` `TrafficManager` (#368): runs a schedule. Departures are spawned on a stand before their STD and push at it (`TaxiRequest.PushbackAt`). Arrivals appear at their STAR entry before their STA. Arrivals turn around into later departures of the same airline and type (the departure adopts the aircraft). Departed and parked aircraft are removed. There are limits in total and per airport, spawn spacing, retries with another model or stand (`ErrSpawnBlocked` waits without counting an attempt), and late flights are cancelled. A `Spawner` does the simulator side; `Board(icao)` gives departures and arrivals. See `docs/traffic-manager.md`.
- `pkg/traffic` situation checks: each Tick the manager looks at each airport, predicts and adjusts. `CheckLandingFlow` keeps landing gaps and opens gaps for waiting departures. `CheckGroundCongestion` holds boarding departures on their stands during a ground stop (`TaxiController.HoldPushback`). `CheckTurnaround` estimates a departure late when its inbound is. `CheckStuck` removes aircraft that stopped making progress. Checks are pluggable (`SituationCheck`, `Advice`).
- `pkg/traffic` other traffic: with a `Picture` the manager respects the traffic that is not ours (MSFS AI, other add-ons, the user) in its checks, or ignores it (`OtherRespect`/`OtherIgnore`, `SetOthers`, `Others(icao)`).
- `pkg/traffic` manager lifecycle events (`ManagerEvent`: added, turnaround, status, retry, blocked, delayed, estimated, held, released, removed, enabled, disabled) through `ManagerOptions.OnEvent` and `Events()`, for the app's own state machine.
- `pkg/traffic` `ModelsFor`: ranks the simulator's aircraft titles for an airline and type (the airline's livery first, FSLTL titles understood).
- `examples/airport-map`: **Scheduled traffic** in the Traffic tab (start/stop, density, max aircraft, departure and arrival boards with estimates, holds and turnarounds). Scheduled flights get generated flight plans and airline liveries. **Other traffic** in Layers is off by default, drawn in blue and listed apart from ours, with respect/ignore and ✕ remove. `GET/POST /api/schedule`, `GET /api/boards`, `POST /api/world/remove`; `/api/traffic` marks `ours`.
- `pkg/traffic` enroute traffic (#369):
  - Arrivals appear en route `EnrouteLead` before their STAR entry, flown by MSFS AI on the rest of their plan, and are handed to the arrival controller at the entry (`FlightEnroute`). If the enroute spawn fails, the arrival appears at the entry, with no attempt lost.
  - Overflights between airports outside the area cross it (`Overflights`, `OverflightOptions`, `Flight.Enter`/`Exit`, `ManagerOptions.Overflights`, `MaxOverflights`).
  - Departures fly on after their SID.
  - Airborne aircraft of ours are removed once they leave the area (`LeftAfter`, `Attach`).
- `pkg/traffic` `EnrouteStart`, `RoutePoint`, `EnrouteSpeedKts`: make an aircraft appear airborne mid-route (`RequestNonATC`) with the rest of its flight as waypoints.
- `pkg/nav` `FlightPlan.PositionAt`: the point, planned altitude and track at a distance along a plan.
- `examples/airport-map`: enroute arrivals and overflights, labels with call sign, flight level and destination, an Overflights board, and `alt` (MSL) in `/api/traffic`.
- `pkg/traffic` level of detail (#370): `Detail` with `TaxiWithDetail` and `ArrivalWithDetail`. An injected aircraft is driven every frame near the viewer and on the runway, every 2nd or 4th frame farther away, and twice a second while standing still. `SetViewer`; `Load()` reports the aircraft driven and their updates a second. `BenchmarkDepartureTaxiFrame`: one aircraft-frame costs about 0.6 µs and 1.2 SimConnect writes.
- `pkg/traffic` `IDBlocks`: controller ID blocks handed out and taken back, so a long session reuses a fixed range. Controllers on a reused block clear their definitions first (the Fleet remembers them). The injector drives up to 96 aircraft and tugs (was 50).
- `examples/airport-map`: controllers use 128 reusable ID blocks and level of detail (the viewer is the user aircraft). The traffic picture shows the load. Enroute request IDs move to 41000+ (they overlapped the library's default ranges).
- `examples/airport-map` world view (#371):
  - 🌐 zooms out to the traffic picture's circle: the airports in range and every aircraft, coloured by phase and labelled with call sign, level, phase and destination;
  - the picture can be centred on the map centre;
  - scheduled traffic runs at several airports, with a board per airport;
  - `POST /api/schedule` takes `airports` and loads them;
  - Playwright `world-view.spec.ts`.

### Changed

- `examples/airport-map`: the runway defaults to **Active (…)**, the runway in use from the weather (departures and arrivals may differ). The route follows it when it changes, and picking a runway overrides it. Spawns, flight plans and the ATC game resolve "active" when each flight starts.
- `examples/airport-map`: the flight plan is a row in New flight that reads LKPR → [destination] for a departure and [origin] → LKPR for an arrival, instead of a field hidden under Options

### Fixed

- `docs/traffic-guide.md`, `EnrouteOpts.Phase`: MSFS 2024 ignores the enroute phase (an enroute ATC aircraft appears at its departure airport, which must be loaded). The phase-0.99 workaround does not work there.
- `pkg/traffic`: an aircraft holding at a limit or hold-short reports no path ahead, so it no longer makes nearby moving traffic brake to a stop for it (give-way).
- `pkg/traffic`: `AbortTakeoff` while lining up or lined up holds the aircraft until the next `ClearForTakeoff`, also without held gates; `HoldPosition` is refused while lining up (it could not be lifted there).
- `pkg/traffic`: after a rejected take-off the vacate path gets its own runway crossings and no old limit.
- `pkg/traffic`: a de-icing pad not on the taxi path (passed during the pushback) de-ices in place instead of being skipped.
- `pkg/traffic`: a partly filled `FlapSchedule` gets the missing values from the defaults field by field.
- `pkg/airport`: a custom route with `Taxiways` keeps following the listed taxiway it is already on after a pushback or runway exit (`RouteOptions.CurrentTaxiway`, set by `RemainingOptions`).
- `examples/airport-map`: a STAR is entered at the first fix of its common route (else of its runway transition); a removed arrival no longer departs on its turnaround.
- Two traffic log files committed by mistake are removed; `traffic-*.log` is ignored.

### Known issues

- Removing a parked or departed aircraft can draw a SimConnect "unrecognised ID" exception (3): harmless, to be looked at.
- #370's live criterion (40 aircraft at once without a stall) reached 38 aircraft (LKPR, LKTB, LKMT at density 3) with no stall; 40 is still to be shown.

## [0.14.0] - 2026-09-29

Fine tuning after live sessions at LKPR: ground traffic that behaves like real traffic, natural timing, de-icing, and an airport map built for playing.

### Added

- `pkg/traffic` pushback and traffic behind the stand (#334):
  - A cleared pushback waits while another aircraft is in, or taxiing through, the corridor it sweeps (`PushClearMarginMeters`, `TaxiEvent.PushbackHeld`).
  - Under way it has priority: it reports what it still sweeps (`GroundPicture.ReportPush`), taxiing traffic whose path crosses it gives way, and it stops only for an aircraft actually in the way.
- `pkg/traffic` natural timing (#343): each aircraft draws its own factor for the beacon lead, taxi-light delay, tug disconnect, flap timing, gear-up delay, taxi speed and pushback pace.
  - Spreads: `BeaconLeadSpread`, `TaxiLightSpread`, `TugDisconnectSpread`, `FlapsSpread`, `GearUpSpread`, `TaxiSpeedSpread`, `PushbackSpeedSpread`; a spread of 0 gives the tunable.
  - `TaxiWithSeed` / `ArrivalWithSeed` for reproducible runs; `SimObjectTug.SetDisconnectDelay`.
- `pkg/traffic` de-icing (#323): `TaxiRequest.Deice`, either on the stand before the push, or at a pad the route passes (stop with engines running and the taxi light off, treated, then on).
  - Types and helpers: `Deicing`, `DefaultDeicingDwell`, `TaxiEvent.Deicing`; `airport.DeicingPad`, `Limits.DeicingPads`, `Graph.NearestNode`; `nav.IcingConditions` (at or below +3 °C with visible moisture).
- `examples/airport-map`:
  - De-icing: pads picked from the taxi points (Charts → De-icing pads, kept per airport in `deicing.json`, `GET/PUT /api/deicing`, ❄ badges); a De-icing spawn option (off, automatic from the weather, on the stand, at a pad); the game de-ices automatically.
  - Charts: the airport (elevation, variation, runways with their best approaches, transition altitude, preferred runways), the weather at the user aircraft with the runway in use and wind components, and the ATIS with a Listen button (`GET /api/airportinfo`).
  - Locate buttons (✈ your aircraft, 📍 traffic and controlled aircraft); ⛶ full screen with the panel, ◨ hides or shows the panel.

### Changed

- `examples/airport-map` GUI revision (#357):
  - One tab per task: Traffic, Charts, Layers, and ? (a quick reference handbook).
  - Options and custom routes folded away; one-line hints with details behind ⓘ.
  - Aircraft cards: who waits for a clearance comes first; state and commands, urgent ones first; the model and lights on hover.
  - Map: compact aircraft labels (details on click), taxiway names only when zoomed in, the game score over the map while playing.
- `examples/airport-map` approaches are listed per entry, via each transition and direct (vectors to the final), each with the same final and missed approach; the selected aircraft's own icon is highlighted instead of a dot under it.

### Fixed

- `pkg/traffic`: the pushback-held flag clears once the push is done.
- `examples/airport-map`: the Charts summary counts what it draws; the procedure list says to tick a kind when none is ticked.

## [0.13.0] - 2026-09-28

Milestones v0.9 to v0.13 were built and released together: procedures, navigation data, weather, flight plans, aircraft profiles and the ATC game build on each other.

### Added

#### Procedures — v0.11 (#312–#316)

- `pkg/airport` `ProcedureLoader`: SIDs, STARs and approaches with their runway, enroute and approach transitions and every leg (ARINC 424 type, fix and position, turn direction, course, distance, altitude and speed constraints, IAF/FAF/MAP), plus the airport's MAGVAR. `Leg.Constraint()` gives chart text (`≥4000`, `FL070`, `≤210KT`)
- `pkg/airport` `ProcedurePath` (map geometry: straight between fixes, Dubins turns where flown by heading) and `navlegs.go`: `ResolveSID`, `ResolveSTAR`, `ResolveApproach`, `MissedApproach` → `[]NavPoint` with altitude and speed limits; ATC-style selection `SIDsFor`, `STARsFor`, `ApproachesFor`, `SIDToward`, `STARFrom`, `BestApproach` (ILS > RNAV > LOC > VOR > NDB), and `Arrival(runway, entryFix)`: the STAR plus the best approach through the transition where it ends
- `pkg/calc` `Dubins`: shortest path at a turn radius, with an optional first-turn direction
- `pkg/traffic` flying procedures: `TaxiRequest.Departure` — MSFS AI flies the SID (and the rest of a plan) after the injected climb (`DepartureWaypoints`); `ArrivalRequest.Procedure` — the aircraft appears at the STAR's entry, MSFS AI flies STAR and approach transition to a join point on the centreline, and the injected approach takes over there with the offset blended out (`PlanArrivalProcedure`, `ProcedureJoinNm`, `JoinBlendSeconds`)
- `examples/airport-map`: procedures panel drawn as charts (one procedure at a time, grouped by kind; VOR/NDB/waypoint symbols, constraints, tracks and distances, direction arrows, radar-vector endings, missed approaches); a multi-runway procedure shows only the chosen runway's transition

#### Navigation, weather, flight plans — v0.13 (#328–#332)

- `pkg/nav` (new): `NavLoader` and `AirwayCrawler` (WAYPOINT/ROUTE, VOR, NDB facility data within a radius) → `AirwayGraph` with `Route` (A*), `RouteOrDirect`, JSON cache; a captured LKPR-area graph in `pkg/nav/testdata`; `examples/spike-airways`
- `pkg/nav` weather and ATIS: `WeatherReader` (ambient SimVars at the user aircraft), `ActiveRunways` (wind components, tailwind/crosswind limits, preferential runways), `ATIS` `Text()` and `Spoken()` (digits for a voice), `TransitionLevel`, `ATISService` (information letters); `examples/atis`
- `pkg/nav` flight plans: `Plan(FlightPlanRequest, *AirwayGraph)` — runways from the weather, SID, airways (or direct), STAR and approach, cruise level (semicircular rule, capped for short hops), TOC/TOD, ETE and fuel (`PerformanceFor` per type); `FlightPlan.PLN()` writes an MSFS .pln; `examples/flight-plan`
- `examples/airport-map`: traffic flies generated flight plans — a departure with a destination flies the SID, airways and levels; an arrival from an origin flies the STAR and approach that plan chooses (`-airways`)

#### Aircraft profiles — v0.12 (#308, #324–#326)

- `pkg/traffic` `AircraftProfile` and `ProfileFor(model)`: one profile per type (airframe, ICAO code letter, motion, take-off, approach, rollout, nose offset, flaps, pushback speed) for 30 types with a size-based fallback; `TaxiRequest.Aircraft` / `ArrivalRequest.Aircraft` fill whatever the request leaves zero
- `pkg/traffic` `ProfileReader` and `Refine`: SimVars of a spawned aircraft (span, design speeds, weights, engines, CG height; AI objects report only some reliably); `Recorder`: movement telemetry as JSON lines and per-type summaries
- `pkg/airport` `Limits` and `LimitsFor(layout, procedures)` (#335): transition altitude, climb-out hand-over from the SIDs' initial climb, taxi and apron speed limits and preferential runways (`KnownLimits` table); `TaxiRequest.Airport` / `ArrivalRequest.Airport` use them, `nav.RunwayLimitsFrom` feeds `ActiveRunways`, `Graph.Apron`

#### Flight plans and command helpers — v0.9 (#270, #281, #293, #296, #334, #337, #338, #340)

- `pkg/traffic` ATC commands: `HoldPosition` (departures and arrivals), `ArrivalController.GoAround` (climb-out and a circuit back to the join point for another approach), `TaxiController.AbortTakeoff` (before V1: stop, vacate at the next exit, back to the holding point); `ErrTooLate`, `ErrNotTaxiing`, `ErrNotApplicable`. See `docs/traffic-commands.md`
- `pkg/traffic` turnaround: `TaxiRequest.ObjectID` adopts an aircraft already on its stand; the map departs a parked arrival again after its dwell
- `pkg/traffic` give way: where taxi routes cross or merge, the aircraft further from the conflict stops short of it (`GroundPicture`, `GiveWayLookMeters`)
- `pkg/traffic` a departure waits for the pushback tug to drive clear before it taxis
- `pkg/airport` custom routes: `RouteOptions.Via` (nodes to pass, no turning back there) and `RouteOptions.Taxiways` (names to follow in order), always within the aircraft's size; `RouteError`, `ValidateRouteOptions`, `TaxiwayNames`, `RemainingOptions`
- `pkg/traffic` intersection departures only where the runway ahead is long enough for the type: `RequiredTakeoffRun(profile, TakeoffConditions)`, `ErrEntryTooShort`; the map offers only long-enough entries (`/api/entries?model=`)
- `examples/airport-map`: custom taxi routes (via points picked on the map, taxiways), the picked arrival exit used when spawning, locate buttons (✈ your aircraft, 📍 traffic and controlled aircraft), the remove button apart with a confirmation, clearance phraseology for procedures, hold position, go around and abort take-off

#### ATC game — v0.10 (#272, #282)

- `examples/airport-map` ATC game: departures on SIDs and arrivals on STARs appear on a timer, every one holding for each clearance; scored (+10 per flight, −1 per 30 s of waiting over a minute, −50 lost wingtip separation, −100 two aircraft on a runway). See `docs/atc-game.md`

### Fixed

- `pkg/traffic`: a `ClearUpTo` limit already behind the aircraft when the taxi starts holds it and reports `ErrNotOnRoute` instead of being dropped (#337)
- `pkg/traffic`: injected departures report their position from the first frame, before they move
- `docs/traffic-guide.md`: the known limitations no longer claim there is no ground routing


## [0.8.0] - 2026-09-28

### Added

- `pkg/airport` stands: `Parking.Size()` (`StandSmall`/`Medium`/`Heavy` from TYPE and RADIUS), `Layout.SuitableStands(minRadius, types...)`, `Layout.ParkingConflicts(i)` (overlapping RADIUS circles, e.g. split stands) and `Parking.Airlines` / `ServesAirline` from the `TAXI_PARKING_AIRLINE` records the loader now requests (#291)
- `pkg/traffic` `StandAllocator`: stand reservations with span-aware blocking of overlapping stands, detection of aircraft standing on stands (AI and user, real wing span), `Assign` by span, TYPE, airline and taxi-in length, warning-only taxi route reservation (#292)
- `examples/airport-map`: spawn onto a free stand (**Assign a free stand**), refuse taken stands, **Occupied stands** layer, `GET /api/stands` (#292)
- `pkg/traffic` pushback tug (#304): `TaxiRequest.Tug` takes a `PushbackTug`; `SimObjectTug` spawns a ground vehicle model (default `DefaultTugTitle`, GSX's towbarless `FSDT_Pushback_Trepel_280`) at the nose gear when the pushback is cleared, moves it with the aircraft, then drives it off and removes it. The interface lets a third-party integration (e.g. GSX) take its place. Map: **Pushback tug** option
- `pkg/traffic` ground traffic: `GroundPicture` shared by injected departures and arrivals (`TaxiWithGroundPicture`, `ArrivalWithGroundPicture`); a taxiing aircraft stops `TrafficGapMeters` behind another aircraft's body on its path, so aircraft queue at holding points and follow at a safe gap; the map adds the sim's AI and the user's aircraft (#334)
- `pkg/traffic` push up the alley: from a dead-end stand (LKPR A7, B9, C26) the tug pushes the aircraft back out along its taxilane to the next taxiway and swings the tail there; the turn from the stand onto the lane is a Dubins curve onto its first straight stretch. Pushes stay on the pavement (stand circles and taxi path strips: no buildings or grass) and out of the terminal zone ahead of the gates. Push-and-turn on the apron remains the last resort, ending past the junction on the taxi-out. `NewSmoothPath`
- `pkg/traffic` pushback fits the aircraft: the tail only goes onto taxiways the aircraft fits (`Graph.Fits`), and the push may continue straight back past the first junction to a later one on the stand axis (a 777 at LKPR B14 is pushed back to J, not onto JO)
- `pkg/traffic` push and turn: stands whose only taxiway at the junction is the way out (LKPR A7, B9) turn the aircraft on the apron along a Dubins path, ending short of the junction facing the taxi-out (#341)
- `pkg/traffic` pushback direction: the tail goes onto the branch from which the taxi-out is cheapest (`planPushback`: each branch's taxi-out planned from the junction), and the route becomes stand → junction → that taxi-out; LKPR C17 no longer ends facing away from its route. `TestPushbackFacesRoute` checks every LKPR pushback stand (A7, B9: #341)
- `pkg/airport` routes by aircraft size: `RouteOptions.HalfSpan`, `WingtipMargin`, `OwnStands`, `TaxiwayMaxSpan` (default `KnownTaxiwayMaxSpan`, LKPR JO/JB code C), `Edge.Clearance` (free half-width to the nearest stand circle), `Route.Tight`; `pkg/traffic` controllers route with their aircraft's span, so a 777 leaves LKPR B14 by J
- `pkg/airport`: `Graph.RouteToRunwayFrom(from, prev, …)`, `Graph.RouteFromNodes`, `Route.Cost`, `RouteOptions.OwnApronMeters` (the apron penalty is waived around the start)
- `pkg/traffic` tug, live-tuned: default `FSDT_Pushback_03` (GSX's classic towbar tug), facing the aircraft (`TugYawDeg` 180) with the bar on the nose wheel; it connects while the aircraft waits for pushback; the bar swings with the nose wheel's travel through the arc (`TugMaxBarDeg`, `TugBarSeconds`); after the push it backs off the nose, then turns away. `MotionProfileFor(model)` gives the airframe per family (wheelbase, span, tail) since MSFS 2024 reports no gear contact points; the map uses it for departures, arrivals, the tug and stand spans
- `pkg/traffic` `Injector.PlaceMoving`: places an object with its ground speed as its speed, so vehicles animate their wheels; the pushback tug uses it (checked live)
- `pkg/traffic` `GroundMover`: a move that starts from a standstill within `StopApproachMeters` of its stop point (short hops, a tug backing off) pulls away instead of standing still forever
- `pkg/traffic` take-off without tail strikes: `TakeoffProfile.TailstrikePitch`; on the runway the pitch stays `TailstrikeMarginDeg` below it (a small pull to lift off), after lift-off it is held until a positive climb (`PositiveClimbFt`) and then rises no faster than the tail clears the runway. Gear up on a positive climb (`GearUpDelaySeconds`, `GearUpFpm`). `TakeoffProfileFor(model)` picks figures by family (777-300, 777, 787, 747, A380, A350, A330, A321, 737, regional jets, turboprops); the map uses it
- `examples/airport-map`: a completed departure (handed to MSFS AI) no longer leaves a marker at its hand-over point
- `examples/airport-map`: the traffic log gives clearances in ATC phraseology, with the taxiways ("AFR1383, taxi to holding point runway 24 via B2, H, A"), also those the controller gives itself with gates off; **Safe zones** checkbox: half the wing span plus 3 m around every aircraft on the ground, red where two overlap; `/api/traffic` reports the span
- `examples/airport-map`: one **▶ Spawn** button that follows the route mode and names what it spawns ("▶ Spawn departure: C22 → runway 24 at B"), with only that mode's options
- `examples/airport-map`: the occupied-stands layer is drawn on the shared canvas and not interactive; as a separate SVG layer it swallowed clicks on the stands
- `pkg/types`: `SIMCONNECT_FACILITY_DATA_VDGS`, `_HOLDING_PATTERN`, `_TAXI_PARKING_AIRLINE`
- `GetLastSentPacketID` on `engine.Client` and `manager.Manager`: record the send ID of a request to attribute a later `SIMCONNECT_RECV_EXCEPTION` (`DwSendID`) to it. See "Attributing Exceptions" in `docs/usage-client.md` (#301). **Breaking** for custom implementations of `engine.Client`.
- `pkg/traffic` injected ground movement: `GroundPath`, `GroundMover` (turn-radius speed planning, jerk-limited speed, nose-gear steering with a trailing main gear, holds) and `Injector` (takeover and freeze, `Place` on the ground at 60 Hz, `SetLights` with phase presets, `Release`). Lights stay as set, which MSFS AI does not allow. See `docs/traffic-motion.md` (#309)
- `pkg/airport` turn-aware routing: costs for turns at junctions, taxiway changes, runway crossings, turning back and apron taxilanes (`RouteOptions.TurnPenalty`, `TaxiwayChangePenalty`, `RunwayCrossingPenalty`, `ApronPenalty`), so routes prefer fewer turns and taxiways without stands even when a little longer (#307)
- `pkg/traffic` hybrid arrival: `ArrivalWithInjector` — MSFS AI lands, the injector takes over clear of the runway without a jump and drives vacate stop, taxi-in and parking with consistent lights; `Injector.Watch`, `NewGroundMoverFrom`, `NoseGear`; `ai-arrival -inject` (#309)
- `pkg/airport` runway entries: `RunwayEntries`, `RouteToRunwayEntry` ("24 at B"), `Route.Entry`, `ErrUnknownEntry` (#306)
- `examples/airport-map`: departure and arrival route modes with an entry/exit picker (panel and map markers)
- `pkg/traffic` injected approach: `ApproachMover` (glide path, speed schedule, flare, touchdown rate, de-rotation), `ArrivalRequest.InjectApproach`, `Injector.PlaceAir`, `SetGear`, `SetFlaps`; `ai-arrival -inject-approach` (#318)
- `pkg/traffic` injected departure: `TaxiWithInjector` drives pushback (tail first onto the taxiway, `NewPushbackMover`), taxi out, line-up along the entry taxiway, take-off (`TakeoffMover`) and the initial climb, handing over to MSFS AI at 1500 ft. Clearance gates `ClearPushback`, `ClearToTaxi`, `ClearToCross`, `ClearToLineUp`, `ClearForTakeoff` (`HoldForClearances`, otherwise automatic), rolling take-offs, take-off flaps, lights by phase; `TaxiRequest.Entry` ("24 at B"); `ai-taxi -inject -gates -entry` (#320)
- `pkg/traffic` ground spoilers at touchdown (`Injector.SetSpoilers`) and approach flaps 3, running to full at 1000 ft, on injected landings (#318)
- `pkg/traffic` arrival details: runway-crossing clearance gate (`HoldAtCrossings`, `ClearToCross`, `ArrivalHoldingShort`), rolling clearance (`RollThroughChance`), dwell variation, crossing lights between hold-short lines, slow stand entry (#309)
- `pkg/traffic` progressive taxi: `ClearUpTo` a route node for departures and arrivals; the aircraft holds with its nose gear on the node (`TaxiEvent.LimitNode`, `AtLimit`) (#322)
- `pkg/traffic` pushback fitted to each stand: straight back along the stand axis, then the widest arc (`PushbackMinArcMeters`–`PushbackArcMeters`) the distance to the taxiway and its straight run allow, tighter where the tail or a wingtip would swing into a neighbouring stand; it ends aligned on the taxiway, never in a bend. `MotionProfile.SpanMeters`, `TailMeters` (#322, #304)
- `examples/airport-map` traffic control: spawn departures and arrivals at stands with a searchable model and livery list, clearance buttons and progressive taxi from the map; `/api/control`, `/api/models`, `/api/control/log`; a traffic log panel and file (`-log-dir`) with every spawn, state and light change (#322)
- `docs/traffic-arrival.md`: arrivals guide — AI, hybrid and injected approach, runway exits, rollout, crossings, stands (#280)

### Changed

- `airport.MaxExitAngle` is 90° (was 100°): exits and entries pointing back along the runway are left out.

### Fixed

- `pkg/airport`: `TaxiPath.RunwayNumber` / `RunwayDesignator` are zero on non-runway paths; MSFS leaves them uninitialised there
- `traffic.EnrouteOpts.Phase` doc: `dFlightPlanPosition` is the waypoint index plus the fraction along the next leg, not 0–1 (#300)
- `examples/read-objects`, `examples/airport-map`: simobject and livery enumeration read entries at the wrong offset (the list header is 28 bytes; entries are a fixed 512), which garbled titles and liveries
- `pkg/traffic`: `ArrivalController.Cancel` and `TaxiController.Cancel` also remove the aircraft after the controller finished (parked, or handed to MSFS AI); the airport map can remove finished aircraft
- `pkg/traffic`: departures start on the stand's stop mark, not the stand circle centre; pushbacks follow an arc instead of pivoting (#304)
- `SIMCONNECT_EVENT_FLAG_*` had sequential (`iota`) values instead of the SimConnect bit flags. `SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY` was 3 (both repeat timers) instead of `0x10`, so `TransmitClientEvent` with a priority as group ID failed with `SIMCONNECT_EXCEPTION_ERROR` (parameter 5); `FAST_REPEAT_TIMER` and `SLOW_REPEAT_TIMER` were swapped. Affected `simvar-cli emit` and the REPL too (#310)
- `pkg/airport`: taxiway edges along a runway surface were excluded from routing, which cut off every runway at LROP, where the taxiways cross 08R/26L along its end. They are now allowed at `AlongRunwayFactor` × their length.

## [0.7.0] - 2026-09-27

### Added

#### `pkg/airport` — airport ground layout and taxi routing (#268, #243)

New package that turns SimConnect facility data into a typed model of an airport's ground layout and a routable taxi graph.

| API | Description |
|-----|-------------|
| `Layout` | Runways (both ends, headings, thresholds), parking stands (`Label()` → `C22`, `S22A`), taxi points (`IsHoldShort()`), taxi paths, taxiway names; items keep their SimConnect index |
| `BuildLayout(RawAirport)` | Decode raw facility records; positions resolved from `BIAS_X`/`BIAS_Z` |
| `Loader` | Request an airport's facility data and assemble it from the application's message loop via `Handle(msg)` / `Expire(now)`; never reads the engine stream. Works with `engine.Client` and `manager` |
| `Cache` | Layouts per ICAO, each graph built at most once |
| `BuildGraph(*Layout)` | Taxi graph: TAXI/PATH taxiway edges, PARKING paths to stand nodes, RUNWAY edges flagged; hold-shorts associated with their runway |
| `Graph.RouteToRunway` / `RouteToParking` / `Route` | Dijkstra routes with taxiway name sequence and runway crossings |
| `Layout.GeoJSON()`, `Route.Feature()` | GeoJSON export |

Facility data semantics verified against MSFS 2024 (LKPR): PARKING path `END` indexes the parking list; hold-short points are identified by `TAXI_POINT.TYPE`; `SUFFIX` distinguishes stands sharing name and number. See `docs/airport-layout.md`.

#### `pkg/traffic` — departure taxi controller (#243)

| API | Description |
|-----|-------------|
| `TaxiController` | Spawn an AI aircraft at a stand, push back, taxi to the hold-short, then line up and take off after `ClearForTakeoff()`; driven by `Handle(msg)`, progress on `Events()`, `Cancel()` removes the aircraft |
| `TaxiWaypoints`, `LineUpWaypoints` | Build the AI waypoint chains from an `airport.Route` |
| `tunables.go` | Taxi speeds, turn and hold-short distances, controller IDs |

MSFS AI cannot steer while reversing, so pushback is a single straight reverse leg followed by a forward turn onto the taxiway. See `docs/traffic-taxi.md`.

#### Examples

- `examples/airport-map` — interactive Leaflet map of an airport's layout with raw facility values, departure route viewer and overlapping-stand highlighting; `-dump`/`-file` for offline use (#267, #278)
- `examples/ai-taxi` — AI departure from LKPR C22 to runway 24 (#254)

#### Docs

- `docs/airport-layout.md` (new **Airport** section on the website) and `docs/traffic-taxi.md` (#279)

### Changed

- **Breaking:** minimum Go version raised to 1.27.1 (root module and `simvar-cli`). Go 1.25 no longer receives security fixes (#265)

## [0.6.1] - 2026-09-27

### Fixed

- `AddToDataDefinition` now passes `NULL` to SimConnect when `unitsName` is empty, so SimConnect uses the variable's default unit instead of raising `SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID` (#263, thanks @aut0mater)
- `pkg/registry`: `TRANSPONDER CODE` is now `Indexed` and accepts the `bcd16` unit (#263)
- `pkg/registry`: `Validate` accepts an empty unit for any known SimVar, matching the SimConnect default-unit behaviour (#263)
- `internal/simconnect`: fix `unsafe.Pointer` lifetime issue in `stringToBytePtr` (#260)

### Security

- Bump Go to 1.25.8 (root module and `simvar-cli`) to fix GO-2026-4601 (`net/url` IPv6 parsing)
- Bump Go to 1.25.14 to fix standard-library vulnerabilities in `crypto/tls`, `crypto/x509`, `encoding/asn1`, `net`, `net/http` and `net/textproto` reported by `govulncheck` (GO-2026-4870, GO-2026-4946, GO-2026-4947, GO-2026-4971, GO-2026-5037 and others)
- Add `govulncheck` CI workflow (#259)

## [0.6.0] - 2026-03-14

### Added

#### `pkg/manager` — Client Data Area API parity (#233, #234, #235)

Five Client Data Area methods added to the `Manager` interface and `*Instance`, completing CDA support in the manager layer. All methods follow the `m.engine == nil → ErrNotConnected` guard pattern.

| Method | Description |
|--------|-------------|
| `CreateClientData(clientDataID, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error` | Register a CDA with the given ID and byte size |
| `AddToClientDataDefinition(defineID, dwOffset, dwSizeOrType uint32, epsilon float32, datumID uint32) error` | Add a typed field to a CDA definition |
| `ClearClientDataDefinition(defineID uint32) error` | Remove all field definitions for a CDA definition ID |
| `RequestClientData(clientDataID, requestID, defineID uint32, period, flags, origin, interval, limit ...) error` | Subscribe to CDA updates |
| `SetClientData(clientDataID, defineID, flags, dwReserved, cbUnitSize uint32, data unsafe.Pointer) error` | Write data to a CDA |

`ClearClientDataDefinition` also required a new raw DLL syscall binding in `internal/simconnect/clientdata.go` and a new `*Engine` implementation in `pkg/engine/clientdata.go`.

**Breaking change:** `pkg/engine.Client` interface now declares all five CDA methods. Any external code that implements `Client` as a mock must add these methods. Acceptable under the pre-1.0 versioning policy.

#### `pkg/manager` — Input Events API (#236)

Six Input Event methods added to the `Manager` interface and `*Instance`. MSFS 2024 only; the engine layer already implemented these.

| Method | Description |
|--------|-------------|
| `EnumerateInputEvents(requestID uint32) error` | Request enumeration of all registered input events |
| `GetInputEvent(requestID uint32, hash uint64) error` | Request the current value of an input event by hash |
| `SetInputEventDouble(hash uint64, value float64) error` | Set a double-typed input event value |
| `SetInputEventString(hash uint64, value string) error` | Set a string-typed input event value |
| `SubscribeInputEvent(hash uint64) error` | Subscribe to change notifications for an input event |
| `UnsubscribeInputEvent(hash uint64) error` | Cancel a subscription |

Subscriptions are not restored on reconnect — callers should resubscribe in an `OnOpen` or `OnConnectionStateChange` handler.

#### `pkg/registry` — `navigation` and `autopilot` SimVar categories (#237, #238)

Two new categories added to the compile-time SimVar registry.

- **`navigation`** (9 entries): `COM STANDBY FREQUENCY`, `NAV ACTIVE/STANDBY FREQUENCY`, `ADF ACTIVE FREQUENCY`, `ADF RADIAL`, `GPS GROUND SPEED`, `GPS GROUND MAGNETIC TRACK`, `GPS POSITION LAT/LON`
- **`autopilot`** (8 entries): `AUTOPILOT HEADING/ALTITUDE LOCK`, `AUTOPILOT VERTICAL HOLD (+ VAR)`, `AUTOPILOT AIRSPEED HOLD (+ VAR)`, `AUTOPILOT NAV1 LOCK`, `AUTOPILOT APPROACH HOLD`

Registry total: 121 entries across 5 categories.

#### `pkg/datasets/navigation` — new sub-package (#239)

Pre-built dataset builders for radio stack and GPS data.

| Constructor | Struct | Fields |
|-------------|--------|--------|
| `NewRadioDataset() *datasets.DataSet` | `RadioDataset` | COM1 active/standby, NAV1 active/standby, ADF1 active frequencies |
| `NewGPSDataset() *datasets.DataSet` | `GPSDataset` | GPS latitude, longitude, ground speed, ground magnetic track |

#### `simvar-cli` — `list` command (#240)

New `list` subcommand enumerates the SimVar registry without connecting to the simulator.

```
simvar-cli list                            # all entries
simvar-cli list --category navigation      # filter by category
simvar-cli list --search altitude          # substring match on Name and Description
simvar-cli list --category autopilot --search hold  # combined (AND)
simvar-cli --format json list              # NDJSON output
simvar-cli --format csv  list              # CSV with header row
```

#### Documentation (#241, #242)

- `docs/manager-client-data-area.md` — complete reference for the 5 CDA manager methods, including a full reader example, method reference table, ID range guidance, and `ErrNotConnected` notes
- `docs/manager-input-events.md` — complete reference for the 6 Input Event manager methods, including enumeration workflow, Double vs String guidance, and no-auto-resubscribe warning

## [0.5.1] - 2026-03-14

### Added

#### CI — `simvar-cli` Windows binary release automation

A new GitHub Actions workflow (`.github/workflows/release-cli.yml`) runs on every published release and automatically builds and attaches a pre-built Windows binary to the release assets.

| Asset | Description |
|-------|-------------|
| `simvar-cli-vX.Y.Z-windows-amd64.zip` | Binary (`simvar-cli.exe`) + README, stripped of debug symbols (`-s -w`) |

### Changed

- `docs/simvar-cli.md` — Installation section now lists the pre-built binary download as the recommended path; build-from-source instructions retained under a separate heading.

## [0.5.0] - 2026-03-13

### Added

#### `pkg/registry` — typed SimVar metadata catalogue (#230)

Cross-platform (no `//go:build windows`) package providing a compile-time catalogue of 104 SimVar entries across three categories (`aircraft`, `environment`, `simulator`).

| Function | Description |
|----------|-------------|
| `Lookup(name string) (SimVarMeta, bool)` | Case-insensitive lookup; strips `:N` indexed suffix |
| `All() []SimVarMeta` | Snapshot of all 104 entries |
| `Validate(name, unit string) error` | Checks name existence and unit acceptability |
| `ByUnit(unit string) []SimVarMeta` | Filter entries by unit string |
| `ByCategory(category string) []SimVarMeta` | Filter by category (`aircraft`, `environment`, `simulator`) |

`SimVarMeta` carries `Name`, `Units []string`, `DefaultUnit`, `Type`, `Category`, `Writable`, `Indexed`, and `Description`. The `init()` function builds the in-memory map and panics on duplicate keys as a programming-error guard.

#### `pkg/datasets` — aircraft, environment, simulator, and objects packages

Four new dataset packages providing pre-built `DataSet` definitions for common SimVar groups:

- `pkg/datasets/aircraft` — position, attitude, speed, engine, control surfaces, autopilot
- `pkg/datasets/environment` — weather, ambient conditions, time
- `pkg/datasets/simulator` — camera, realism, simulation state
- `pkg/datasets/objects` — generic SimObject position and identity fields

#### `pkg/engine` — SimConnect Client Data Area API (#227)

Six new methods on the `Client` interface expose the full Client Data Area lifecycle:

| Method | Description |
|--------|-------------|
| `MapClientDataNameToID(name string, clientDataID uint32) error` | Maps a named shared memory area to a numeric ID |
| `CreateClientData(clientDataID, size uint32, flags uint32) error` | Allocates a new client data area |
| `AddToClientDataDefinition(defineID, offset, sizeOrType uint32, epsilon float32, datumID uint32) error` | Adds a field to a client data definition |
| `ClearClientDataDefinition(defineID uint32) error` | Removes all fields from a client data definition |
| `RequestClientData(clientDataID, requestID, defineID uint32, period, flags uint32, origin, interval, limit uint32) error` | Subscribes to periodic client data updates |
| `SetClientData(clientDataID, defineID uint32, flags uint32, data unsafe.Pointer, size uint32) error` | Writes a value to a client data area |

#### `cmd/simvar-cli` — watch command, structured output, JSON config

Interactive CLI tool promoted from `examples/` to `cmd/` as a first-class tool:

- **`watch` command** — continuous SimVar streaming with `--interval second|visual-frame|sim-frame` and `--changed` (print only when value changes)
- **`--format table|json|csv`** — aligned table (default), NDJSON, or RFC 4180 CSV output
- **`--config`** — JSON config file with 4-step resolution (`--config` flag → `SIMVAR_CLI_CONFIG` env → `%APPDATA%\simvar-cli\config.json` → `.\simvar-cli.json`)
- Zero external dependencies beyond CURE — config uses `encoding/json` (stdlib)

### Fixed

#### `pkg/datasets` / `pkg/manager` — unit string alignment (#229)

Two SimVar unit strings were mismatched between `pkg/datasets` and the SimConnect SDK:

- `AMBIENT PRESSURE` — `"millibars"` → `"inches of mercury"` (SimConnect returns inHg, not mbar)
- `VERTICAL SPEED` — `"feet/minute"` → `"feet per minute"` (SDK unit string requires the spelled-out form)

Both mismatches caused silent zero reads when using the pre-built dataset definitions with the manager's SimState.

### Changed

- `cmd/simvar-cli` promoted from `examples/simvar-cli` — module path updated to `github.com/mrlm-net/simconnect/cmd/simvar-cli`
- `cmd/simvar-cli` config format changed from TOML to JSON — zero external dependencies

## [0.4.3] - 2026-03-05

### Fixed

#### `pkg/types` — wire struct alignment bugs (second pass)

Two additional Go-vs-wire alignment bugs discovered by systematic review, following the same
`#pragma pack(1)` vs Go alignment-padding pattern fixed in v0.4.2.

**`SIMCONNECT_DATA_RACE_RESULT` — critical data corruption on float64 fields**

`FTotalTime float64` and `FPenaltyTime float64` were at wire offset 1060 and 1068 respectively.
The prefix before `FTotalTime` is `DWORD(4) + GUID(16) + 4×char[260](1040) = 1060 bytes`;
`1060 % 8 = 4`, so Go inserts 4 bytes of padding, shifting both fields 4 bytes past their
wire positions. Any cast of a raw SimConnect buffer to this struct would silently produce
garbage values for both timing fields and `DwIsDisqualified`.

Fix: `FTotalTime float64` → `FTotalTimeBytes [8]byte` and `FPenaltyTime float64` →
`FPenaltyTimeBytes [8]byte` (alignment 1, no padding). Decode with
`math.Float64frombits(binary.LittleEndian.Uint64(r.FTotalTimeBytes[:]))`.

Note: This is a **breaking rename** of public fields. Any code reading `.FTotalTime` or
`.FPenaltyTime` directly will fail to compile — this is intentional, as silent misreads
are more dangerous than a compile error.

**`SIMCONNECT_DATA_FACILITY_VOR` — compound misalignment documented**

The VOR struct has two independent misalignment layers that compound:

1. Airport base (already documented on `SIMCONNECT_DATA_FACILITY_AIRPORT`): the
   `ident+region` byte prefix before `Latitude` is not 8-byte aligned, causing Go to
   pad before all float64 fields.
2. VOR-internal (previously undocumented): `Flags DWORD` immediately precedes
   `FLocalizer float64`. `NDB` Go sizeof = 56; `Flags` ends at offset 60; `60 % 8 = 4`;
   Go pads 4 more bytes. `FLocalizer` lands at Go offset 64 vs wire offset 52 (MSFS 2024)
   — a 12-byte total discrepancy affecting all five VOR float64 fields.

Fix: detailed `WARNING` godoc block added to `SIMCONNECT_DATA_FACILITY_VOR` documenting
both misalignment levels and exact Go vs wire offsets. No field changes (struct is only
used via runtime stride arithmetic per the AIRPORT pattern).

### Added

#### `pkg/engine` — MSFS 2024 Input Event API (#143)

Six new methods on the `Client` interface expose the full Input Event lifecycle. This API
is available in MSFS 2024 only — the underlying DLL functions are not present in MSFS 2020.

| Method | Description |
|--------|-------------|
| `EnumerateInputEvents(requestID uint32) error` | Requests a paginated list of all input events known to the simulator; responses arrive as `SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` messages |
| `GetInputEvent(requestID uint32, hash uint64) error` | Requests the current value of a single input event by 64-bit hash; response arrives as `SIMCONNECT_RECV_GET_INPUT_EVENT` |
| `SetInputEventDouble(hash uint64, value float64) error` | Sets an input event value from a Go `float64`; owns the stack-allocated buffer for the synchronous DLL call duration |
| `SetInputEventString(hash uint64, value string) error` | Sets an input event value from a Go `string`; same buffer-safety guarantee as the double variant |
| `SubscribeInputEvent(hash uint64) error` | Subscribes to value-change notifications for an input event; updates arrive as `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` messages |
| `UnsubscribeInputEvent(hash uint64) error` | Cancels a previous subscription |

Four package-level value extractor functions are provided in `pkg/engine` to decode the
inline byte buffers returned by the DLL without exposing `unsafe.Pointer` to callers:

| Function | Description |
|----------|-------------|
| `InputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (float64, bool)` | Reads the first 8 bytes of `Value` as a little-endian IEEE 754 `float64`; returns `false` if `EType` is not `SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE` |
| `InputEventValueAsString(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (string, bool)` | Reads `Value` to null terminator as a UTF-8 string; returns `false` if `EType` is not `SIMCONNECT_INPUT_EVENT_TYPE_STRING` |
| `SubscribeInputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (float64, bool)` | Same as above for subscribe receive type |
| `SubscribeInputEventValueAsString(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (string, bool)` | Same as above for subscribe receive type |

Three new `As*` helpers on `*Message` follow the existing nil-guard-then-cast pattern:

| Method | Description |
|--------|-------------|
| `(*Message).AsEnumerateInputEvents() *types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` | Casts dispatch buffer to enumerate response; returns `nil` if message ID does not match |
| `(*Message).AsGetInputEvent() *types.SIMCONNECT_RECV_GET_INPUT_EVENT` | Casts dispatch buffer to get-event response |
| `(*Message).AsSubscribeInputEvent() *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` | Casts dispatch buffer to subscribe notification |

#### `pkg/types` — receive struct fixes and new type (#143)

- `SIMCONNECT_RECV_GET_INPUT_EVENT.Value` corrected from `unsafe.Pointer` to `[260]byte` —
  the original type was incorrect because the bytes are inline in the DLL dispatch buffer,
  not a heap pointer. `[260]byte` covers both DOUBLE (8 bytes, read via
  `math.Float64frombits`) and STRING (up to 32 chars per MSFS 2024 SDK) and is safe for
  the GC.
- `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` restructured to flat fields (no embedded
  `SIMCONNECT_RECV`) — confirmed via MSFS 2024 SDK and FlyByWire Rust bindgen output that
  `SimConnect.h` wraps this struct in `#pragma pack(1)`. Go's natural alignment would
  insert 4 bytes of padding before `Hash` (UINT64 at wire offset 12), producing incorrect
  field reads. The flat layout matches the wire format exactly. `Value` corrected from
  `unsafe.Pointer` to `[260]byte` (max 256 chars for STRING per MSFS 2024 SDK).
- `SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` added — embeds `SIMCONNECT_RECV_LIST_TEMPLATE`
  (28 bytes) with a sentinel `RgData [1]SIMCONNECT_INPUT_EVENT_DESCRIPTOR` field; iterate
  over `DwArraySize` elements via the `AsEnumerateInputEvents()` engine helper using
  unsafe pointer arithmetic, identical to the existing airport/NDB/VOR list patterns.

#### `internal/simconnect` — five new DLL bindings (#143)

`internal/simconnect/inputevent.go` adds raw syscall wrappers for
`SimConnect_EnumerateInputEvents`, `SimConnect_GetInputEvent`, `SimConnect_SetInputEvent`,
`SimConnect_SubscribeInputEvent`, and `SimConnect_UnsubscribeInputEvent`. The 64-bit hash
parameter is passed as `uintptr(hash)` at the syscall boundary (amd64 Windows, no
split-register concern). `SetInputEvent` accepts `unsafe.Pointer` at the internal API
boundary only; the `pkg/engine` typed wrappers above keep `unsafe.Pointer` off the public
`Client` interface entirely.

---

## [0.4.2] - 2026-03-05

### Fixed

#### `pkg/manager` — critical `simStateDataStruct` alignment bug

`simStateDataStruct` mixed `int32` and `float64` fields. SimConnect packs data definition
buffers with no alignment padding; Go inserts padding before `float64` fields not at
8-byte-aligned struct offsets. This produced four silent misalignment gaps:

- Before `Latitude` (+4 bytes): all position and speed fields read from wrong offsets
- Before `MissionScore` (+8 bytes cumulative)
- Before `ZuluSunriseTime` (+12 bytes cumulative): time zone fields corrupted
- Before `EnvSmokeDensity` (+16 bytes cumulative): all extended environment fields corrupted

The bug produced garbage floating-point values (e.g. `Lon = -2.6e+67`) in every Manager
SimState update. Introduced in commit `06e735b` (v0.2.0, 2026-02-08) when `SurfaceType`
(`int32`) was added immediately before the `Latitude` (`float64`) block.

Fix: all fields in `simStateDataStruct` changed to `float64`. SimConnect automatically
converts integer SimVars to `float64` when `SIMCONNECT_DATATYPE_FLOAT64` is requested,
so no data is lost. All `AddToDataDefinition` calls in `simstate_registration.go` updated
to match (`SIMCONNECT_DATATYPE_INT32` → `SIMCONNECT_DATATYPE_FLOAT64`). Cast sites in
`dispatch-simstate.go` updated with `int32(stateData.X)` where the public `SimState`
field is `int32`.

#### `pkg/types` — wire struct alignment fixes

- `SIMCONNECT_RECV_SYSTEM_STATE.FFloat float64` → `FFloatBytes [8]byte` — `float64`
  (alignment 8) after `SIMCONNECT_RECV` (12 B) + `DwRequestID` (4 B) + `WInteger` (4 B)
  = 20 bytes total; Go would pad to offset 24. `[8]byte` (alignment 1) places the field
  at wire-correct offset 20. Use `engine.SystemStateFloat64(recv)` to decode.
- `SIMCONNECT_JETWAY_DATA.ParkingIndex`, `Status`, `Door` changed from `int` (8 bytes on
  64-bit Go) to `uint32` (4 bytes, matching the SDK DWORD). The previous `int` fields
  doubled the size of each, corrupting all subsequent field offsets.

### Added

#### `pkg/engine` — value extractor helpers

- `SystemStateFloat64(recv *types.SIMCONNECT_RECV_SYSTEM_STATE) float64` — decodes
  `FFloatBytes [8]byte` via `binary.LittleEndian`; eliminates manual bit conversion at
  call sites.
- `SubscribeInputEventHash(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) uint64` —
  decodes `HashBytes [8]byte` at wire offset 12; callers no longer need to write
  `binary.LittleEndian.Uint64(recv.HashBytes[:])` directly.

---

## [0.4.1] - 2026-03-01

### Fixed

- `pkg/convert/position.go` — aligned variable name `w` → `W` in `LatLonToOffset` to match `OffsetToLatLon` naming convention (#217)
- `examples/locate-airport/README.md` — replaced stale `haversineMeters()` references with `calc.HaversineMeters()` following promotion in #219 (#217)

---

## [0.4.0] - 2026-03-01

### Added

#### `pkg/traffic` — traffic guide and updated example (#38)

- `docs/traffic-guide.md` — MVP guide covering all three aircraft kinds, the async
  create→acknowledge lifecycle, waypoint helpers with flag reference, fleet management
  API, manager integration, and known limitations (no ground routing yet)
- `examples/simconnect-traffic/main.go` — rewritten to use `pkg/traffic`: parked spawn,
  non-ATC spawn with pushback→taxi→takeoff waypoint chain, periodic fleet status log,
  and graceful `Fleet.RemoveAll` on shutdown
- Website sidebar now includes Traffic and Datasets navigation sections

---

## [0.3.13] - 2026-03-01

### Added

#### `pkg/traffic` — new AI traffic abstraction package (epic #27 / #36, #37)

| API | Description |
|-----|-------------|
| `NewFleet(client engine.Client) *Fleet` | Creates a thread-safe aircraft fleet bound to an engine client |
| `(*Fleet).RequestParked(opts ParkedOpts, reqID uint32) error` | Queues a parked ATC aircraft creation; resolves asynchronously via `Acknowledge` |
| `(*Fleet).RequestEnroute(opts EnrouteOpts, reqID uint32) error` | Queues an enroute ATC aircraft creation along a flight plan |
| `(*Fleet).RequestNonATC(opts NonATCOpts, reqID uint32) error` | Queues a non-ATC aircraft creation at an explicit position |
| `(*Fleet).Acknowledge(reqID, objectID uint32) (*Aircraft, bool)` | Promotes a pending creation to a tracked `Aircraft` handle; call from `ASSIGNED_OBJECT_ID` handler |
| `(*Fleet).Remove(objectID, reqID uint32) error` | Removes an aircraft from the simulation and the fleet |
| `(*Fleet).ReleaseControl(objectID, reqID uint32) error` | Releases simulator AI control; required before `SetWaypoints` |
| `(*Fleet).SetWaypoints(objectID, defID uint32, wps []SIMCONNECT_DATA_WAYPOINT) error` | Assigns a waypoint chain to a non-ATC aircraft |
| `(*Fleet).SetFlightPlan(objectID uint32, planPath string, reqID uint32) error` | Assigns a flight plan to an ATC aircraft |
| `(*Fleet).Get(objectID uint32) (*Aircraft, bool)` | Returns the tracked `Aircraft` for a given ObjectID |
| `(*Fleet).List() []*Aircraft` | Snapshot of all active aircraft |
| `(*Fleet).Len() int` | Number of active (acknowledged) aircraft |
| `(*Fleet).RemoveAll(reqIDBase uint32) error` | Removes all tracked aircraft |
| `(*Fleet).Clear()` | Resets fleet state without issuing removal calls (use on disconnect) |
| `(*Fleet).SetClient(client engine.Client)` | Swaps the engine client and clears stale state (call on reconnect) |
| `PushbackWaypoint(lat, lon, altFt, ktsSpeed float64)` | Waypoint with `ON_GROUND \| REVERSE \| SPEED_REQUESTED` flags |
| `TaxiWaypoint(lat, lon, altFt, ktsSpeed float64)` | Waypoint with `ON_GROUND \| SPEED_REQUESTED` flags |
| `LineupWaypoint(lat, lon, altFt float64)` | Runway threshold waypoint at 5 kts |
| `ClimbWaypoint(lat, lon, altAGL, ktsSpeed, throttlePct float64)` | Airborne waypoint with `SPEED_REQUESTED \| THROTTLE_REQUESTED \| COMPUTE_VERTICAL_SPEED \| ALTITUDE_IS_AGL` |
| `TakeoffClimb(rwyLat, rwyLon, hdgDeg float64) []SIMCONNECT_DATA_WAYPOINT` | Standard 3-WP climb chain from runway threshold (1.5 nm / 5 nm / 12 nm) |

#### `pkg/manager` — traffic delegation methods (#37)

| API | Description |
|-----|-------------|
| `Fleet() *traffic.Fleet` | Returns the manager's internal fleet; reset on each reconnect |
| `TrafficParked(opts traffic.ParkedOpts, reqID uint32) error` | Delegates to `Fleet().RequestParked` |
| `TrafficEnroute(opts traffic.EnrouteOpts, reqID uint32) error` | Delegates to `Fleet().RequestEnroute` |
| `TrafficNonATC(opts traffic.NonATCOpts, reqID uint32) error` | Delegates to `Fleet().RequestNonATC` |
| `TrafficRemove(objectID, reqID uint32) error` | Delegates to `Fleet().Remove` |
| `TrafficReleaseControl(objectID, reqID uint32) error` | Delegates to `Fleet().ReleaseControl` |
| `TrafficSetWaypoints(objectID, defID uint32, wps []SIMCONNECT_DATA_WAYPOINT) error` | Delegates to `Fleet().SetWaypoints` |
| `TrafficSetFlightPlan(objectID uint32, planPath string, reqID uint32) error` | Delegates to `Fleet().SetFlightPlan` |

---

## [0.3.12] - 2026-03-01

### Added

#### `pkg/datasets` — composition helpers (epic #26 / #32)

| API | Description |
|-----|-------------|
| `(DataSet).Clone() DataSet` | Deep copy of a dataset; returned value has an independent backing slice |
| `Merge(...DataSet) DataSet` | Combines multiple datasets; last definition wins on duplicate `Name` (position shifts to last occurrence) |
| `NewBuilder() *Builder` | Fluent builder for incremental dataset construction |
| `(*Builder).Add(def DataDefinition) *Builder` | Appends a pre-built definition |
| `(*Builder).AddField(name, unit string, dataType, epsilon) *Builder` | Convenience wrapper that constructs a `DataDefinition` inline |
| `(*Builder).Remove(name string) *Builder` | Removes first definition matching `Name` |
| `(*Builder).Build() DataSet` | Returns a new `DataSet`; backing slice is independent of the builder |
| `(*Builder).Len() int` | Number of pending definitions |
| `(*Builder).Reset() *Builder` | Clears the builder (severs backing array) |

#### `pkg/datasets` — global registry (epic #26 / #34)

| API | Description |
|-----|-------------|
| `Register(name, category string, constructor func() *DataSet)` | Registers a named dataset constructor; panics on empty name or category; silently overwrites duplicates |
| `Get(name string) (func() *DataSet, bool)` | Retrieves a constructor by name |
| `List() []string` | Sorted list of all registered names |
| `Categories() []string` | Sorted list of distinct categories |
| `ListByCategory(category string) []string` | Sorted names in a given category; nil if category unknown |

`pkg/datasets/traffic` now auto-registers `"traffic/aircraft"` (category `"traffic"`) via `init()` — import it blank (`_ "github.com/mrlm-net/simconnect/pkg/datasets/traffic"`) to activate.

#### Documentation (epic #26 / #35)

- New guide: `docs/dataset-composition.md` — covers `Clone`, `Merge`, `Builder`, and the global registry with runnable end-to-end snippet.
- `examples/using-datasets/main.go` rewritten to demonstrate blank-import auto-registration, `List`, `Categories`, `Get`, `Clone`, `Builder`, and `Merge`.

---

## [0.3.11] - 2026-02-28

### Fixed

- **Sponsorship links** — appended `?currency=EUR` to all Revolut URLs in `.github/FUNDING.yml`, `README.md`, and the marketing homepage.

---

## [0.3.10] - 2026-02-28

### Added

- **Sponsorship infrastructure** — `.github/FUNDING.yml` activates the GitHub Sponsor button (Revolut custom URL); `README.md` gains a `## Sponsoring` section listing what sponsorship covers; the marketing homepage gains a full-width CTA section. No Go code or API changes. Closes #172, #173, #174.

---

## [0.3.6] - 2026-02-22

### Fixed

- **`pkg/types/receiver.go`** — `SIMCONNECT_RECV_VOR_LIST` had a wrong struct layout copy-pasted from `SIMCONNECT_RECV_SYSTEM_STATE`. It now correctly embeds `SIMCONNECT_RECV_FACILITIES_LIST` with `RgData []SIMCONNECT_DATA_FACILITY_VOR`, matching the SDK and the pattern of `SIMCONNECT_RECV_NDB_LIST` / `SIMCONNECT_RECV_AIRPORT_LIST`. `AsVORList()` previously returned a misinterpreted pointer. Closes #189.
- **`pkg/datasets/facilities/ndb.go`** — `NewNDBFacilityDataset()` was missing `ICAO` and `REGION` fields; NDB identifiers were silently absent from dataset responses. Closes #190.
- **`pkg/datasets/facilities/vor.go`** — `NewVORFacilityDataset()` was missing `ICAO` and `REGION` fields. Closes #191.
- **`pkg/datasets/facilities/waypoint.go`** — `NewRouteFacilityDataset()` PREV block was missing `PREV_LATITUDE` and `PREV_LONGITUDE`; the NEXT block had both but PREV did not. Closes #192.

### Added

#### `pkg/convert`

| Function | File | Description |
|----------|------|-------------|
| `NMToStatuteMiles` | distance.go | NM → statute miles |
| `StatuteMilesToNM` | distance.go | Statute miles → NM |
| `KilometersToStatuteMiles` | distance.go | km → statute miles |
| `StatuteMilesToKilometers` | distance.go | Statute miles → km |
| `StatuteMilesToMeters` | distance.go | Statute miles → m |
| `MetersToStatuteMiles` | distance.go | m → statute miles |
| `KnotsToFeetPerSecond` | speed.go | knots → ft/s (SimConnect body-axis velocity unit) |
| `FeetPerSecondToKnots` | speed.go | ft/s → knots |
| `NormalizeAngle` | angle.go | Normalises angle to (-180, 180] |
| `AngleDifference` | angle.go | Shortest signed rotation from → to in (-180, 180] |

Closes #193, #194, #195.

#### `pkg/calc`

| Function | File | Description |
|----------|------|-------------|
| `AlongTrackMeters` | crosstrack.go | Signed along-track distance from A toward B for point D; positive = ahead, negative = behind |
| `HaversineKM` | haversine.go | Great-circle distance in kilometres |

Closes #196, #197.

---

## [0.3.5] - 2026-02-22

### Added

#### `pkg/calc`

New aviation math functions extending the cross-track and wind correction capabilities of the package.

| Function | Signature | Description |
|----------|-----------|-------------|
| `CrossTrackMeters` | `(latA, lonA, latB, lonB, latD, lonD float64) float64` | Great-circle cross-track distance in meters; positive values indicate the point is to the right of the track |
| `WindCorrectionAngle` | `(windDir, windSpeed, tas, course float64) float64` | Wind correction angle in degrees; returns 0 for near-zero true airspeed |
| `TrueToMagnetic` | `(trueHeading, magVar float64) float64` | Converts a true heading to magnetic; positive `magVar` is easterly; result is normalised to [0, 360) |
| `MagneticToTrue` | `(magneticHeading, magVar float64) float64` | Inverse of `TrueToMagnetic` |
| `CrosswindComponent` | `(windDir, windSpeed, runwayHeading float64) float64` | Signed crosswind component; wrapper over `HeadwindCrosswind` |
| `HeadwindComponent` | `(windDir, windSpeed, runwayHeading float64) float64` | Headwind (positive) / tailwind (negative) component; wrapper over `HeadwindCrosswind` |

Closes #133, #134, #135, #138.

#### `pkg/convert`

Three new conversion files covering temperature, pressure, and weight/volume domains.

**`temperature.go`**

| Function | Converts |
|----------|---------|
| `CelsiusToFahrenheit` | °C → °F |
| `FahrenheitToCelsius` | °F → °C |
| `CelsiusToKelvin` | °C → K |
| `KelvinToCelsius` | K → °C |
| `FahrenheitToKelvin` | °F → K |
| `KelvinToFahrenheit` | K → °F |

**`pressure.go`**

| Function | Converts |
|----------|---------|
| `InHgToMillibar` | inHg → mbar |
| `MillibarToInHg` | mbar → inHg |
| `InHgToHectopascal` | inHg → hPa |
| `HectopascalToInHg` | hPa → inHg |
| `InHgToPascal` | inHg → Pa |
| `PascalToInHg` | Pa → inHg |

**`weight.go`**

| Function | Converts |
|----------|---------|
| `PoundsToKilograms` | lb → kg |
| `KilogramsToPounds` | kg → lb |
| `USGallonsToLiters` | US gal → L |
| `LitersToUSGallons` | L → US gal |

Closes #136, #139, #140, #141, #142.

### Fixed

- Closed stale chore issues that were completed as part of v0.3.4: removal of `//go:build windows` build tags from `pkg/calc` (#132) and all existing `pkg/convert` files (#137). No code changes in this release for these items.

---

## [0.3.4] - 2026-02-22

### Added

#### `pkg/convert`

| File | Functions |
|------|-----------|
| `angle.go` _(new)_ | `DegreesToRadians`, `RadiansToDegrees`, `NormalizeHeading` |
| `distance` | `NMToKilometers`, `KilometersToNM`, `KilometersToMeters`, `MetersToKilometers` |
| `speed` | `KnotsToMetersPerSecond`, `MetersPerSecondToKnots`, `FeetPerMinuteToMetersPerSecond`, `MetersPerSecondToFeetPerMinute` |
| `altitude` | `FeetPerMinuteToFeetPerSecond`, `FeetPerSecondToFeetPerMinute` |
| `position` | Pole guard in `OffsetToLatLon` — prevents division singularity at ±90° latitude |

#### `pkg/calc`

| File | Functions |
|------|-----------|
| `haversine.go` | `HaversineNM` — great-circle distance in nautical miles |
| `bearing.go` | `BearingDegrees` — initial great-circle bearing in [0, 360) |
| `wind.go` _(new)_ | `HeadwindCrosswind` — decomposes wind into headwind/crosswind components relative to a runway heading |

### Fixed

#### `pkg/convert`

- **`IsICAOCode`** now correctly accepts `R` and `S` prefixes — Japan (`RJTT`, `RJAA`), Korea (`RKSI`), Philippines (`RPLL`), and South America (`SBGR`, `SCEL`, `SKBO`, `SEQM`) were previously rejected. Dead code and contradictory guard logic cleaned up.
- **Mach KPH constant corrected** — `KilometersPerHourToMach`/`MachToKilometersPerHour` now derive from `mach1Knots * 1.852`, restoring mathematical closure: `kts → mach → kph` now equals `kts → kph` exactly.

### Other Changes

- `//go:build windows` removed from `pkg/calc` and `pkg/convert` — both are pure math packages with no DLL dependency (closes #132, #137).
- `pkg/calc/main.go` restructured into per-topic files (`haversine.go`, `bearing.go`, `wind.go`).
- Test files split into per-file structure in both packages.

Closes #183, #184, #185, #187.

---

## [0.3.3] - 2026-02-21

### Fixed

- **fix(website):** Scope `overflow-hidden` per marketing section instead of the root layout to restore sticky table-of-contents on documentation pages (#175).

  The initial approach (`overflow-x-hidden` on the root `<div>`) still broke `position:sticky` on the docs sidebar. The correct fix moves overflow containment to each individual marketing section, keeping the root layout clean.

> Patch release — no API or library changes. Go import paths and SDK behaviour are unchanged.

---

## [0.3.2] - 2026-02-20

### Fixed

- **Corrected field offsets** for 40/41-byte airport entry strides in `pkg/types/facility.go` and facility examples — offsets are now `12/20/28` (same as 36-byte stride), not the incorrect `16/24/32` introduced in v0.3.1.
- MSFS 2024 uses `char Ident[9]` (not `char[6]`), so layout is `ident[9] + region[3] = 12 bytes` before doubles with no alignment padding needed. Extra bytes in 41-byte stride are trailing data after altitude, not prefix padding.
- Removed incorrect `airportWire8` struct from all facility examples.
- Added MSFS 2024 ident size documentation to `pkg/types/facility.go`.

Affected examples: `read-facilities`, `all-facilities`, `subscribe-facilities`, `locate-airport`. Closes #119.

---

## [0.3.1] - 2026-02-20

### Fixed

- **Fixed airport facility entry alignment** — SimConnect (MSFS 2024) reports 41-byte entries in `SIMCONNECT_RECV_AIRPORT_LIST` responses but the parsing code used hardcoded offsets for a 36-byte layout. This caused garbled ICAO codes and invalid coordinates for entries 2+ in multi-entry batches. Replaced hardcoded offsets with a runtime switch on `actualEntrySize` supporting 33/36/40/41-byte layouts using `unsafe.Offsetof` for correct field positions (#117).
- Added stride warning comment to `SIMCONNECT_DATA_FACILITY_AIRPORT` in `pkg/types/facility.go`.

Affected examples: `read-facilities`, `all-facilities`, `subscribe-facilities`, `locate-airport`. Closes #118.

---

## [0.3.0] - 2026-02-20

Initial v0.3 milestone release. See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.3.0) for full notes.

---

## [0.2.1] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.2.1).

---

## [0.2.0] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.2.0).

---

## [0.1.2] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.2).

---

## [0.1.1] - 2026-01-27

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.1).

---

## [0.1.0] - 2026-01-18

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.0).
