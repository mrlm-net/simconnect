# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.160.7] - 2026-11-01

### Fixed

- Rolling, a step in the ground or an older ground reading moved the aircraft 0.3 to 0.5 ft in one frame. The height now follows at up to 8 ft/s (#1373).

## [0.160.6] - 2026-11-01

### Fixed

- The simulator reports a wrong ground height for a single frame at some spots, and an aircraft placed on it hopped by 0.15 to 0.39 ft. Ground samples now pass through a median of three, and spikes stay out of the ground map (#1370).

## [0.160.5] - 2026-11-01

### Fixed

- Each airport's airway read after the first registered the facility definitions again, and the simulator answered 34 exceptions per airport. All reads now share one loader (#1369).

## [0.160.4] - 2026-11-01

### Fixed

- An arrival for a stand whose stop sits far back on its spot could lose its whole taxi route: it counted as vacated seconds after touchdown and drove from the runway straight to the stand at speed (live, LKTB 3M: a B738 at 137 kt). The route is now kept and joins the stand at the point furthest behind the stop, or the aircraft turns on the stand (#1366).

## [0.160.3] - 2026-10-31

### Fixed

- A tug's way in could pass under the parked aircraft where the vehicle road runs under it, and come back under it on the way home. Road points under the aircraft are dropped and the tug goes round it, in both directions (#1364).

## [0.160.2] - 2026-10-31

### Fixed

- A tug coming from the aircraft's side turned 180° and hopped 6 m in one frame as it connected at the nose. Its way in now makes that turn before it gets there (#1363).

## [0.160.1] - 2026-10-31

### Fixed

- `addons.AircraftPackage` found nothing from the whole path MSFS 2024 reports after a flight load, so a Fenix fell back to the A320 family profile. It now takes the path from `SimObjects` on, and the Fenix profile also matches titles with "Fenix" (#1361).

## [0.160.0] - 2026-10-31

### Added

- `Leg.OriginFix` and `Leg.Origin`: a procedure leg's recommended navaid, read from the facility data (#1359).

### Fixed

- CD, VD and FD legs ended their DME distance from where the leg began, cut off at 20 km. They now end where the course reaches that distance from the navaid, in `ProcedurePath` and the resolved `NavPoint`s. Data dumped earlier has no navaid and behaves as before (#1359).

## [0.159.2] - 2026-10-31

### Fixed

- The jump trace reported a steady turn on a long frame as a jump: its heading limit now grows with the frame. Each line also gives the simulator's own step, to tell a simulator hitch from a wrong placement (#1357).

## [0.159.1] - 2026-10-31

### Fixed

- The map exiting during a scene left the simulator's clock at the scene's time of day, and on any clean exit with our camera on left the simulator's view where the release put it. Both now go back (#1355).

## [0.159.0] - 2026-10-31

### Added

- `Scene.Local`: the time of day a scene plays at, put back when it ends (#1353).

## [0.158.0] - 2026-10-31

### Added

- Films of a replay on any ghost: `camera.OnObject`, a shot on an object only known as the replay plays; `flight.GhostFleet.Keep` flies a film's subjects whatever the budget and the distance; `GhostFleet.ObjectOf` gives an aircraft's object by its key (#1351).

## [0.157.0] - 2026-10-31

### Added

- Scene events of kind `radio`: each transmission with speaker, frequency, text, actual start and speaking time (#1349).
- `Scene.Lines`: scripted radio to a timetable, by a cast role or a named station, word for word and never moved, with traffic kept off the frequency around it. `traffic.Radio.Script`, `Transmission.Scripted` (#1349).
- `Scene.Mute`: the host hears only the scene's lines (#1349).

## [0.156.0] - 2026-10-31

### Added

- Film moments on any aircraft: `flare`, `touchdown`, `reversers`, `vacated`, `rotate`, `liftoff`, `gearup`, by `World.OnMoment` and `GET /api/camera/moments`. Ours say all of them; the player's and other traffic's are read off the scan (no flare, rotate or reversers) (#1347).
- Scene cues `ROLE:MOMENT`, also on `player` and any call sign outside the cast, and `ROLE:below:FEET` for an arrival's height (#1347).

## [0.155.1] - 2026-10-31

### Fixed

- Spotting director: the `ours` kind and the `Types` filter matched nothing, ours counted as `other`, and `subject` could be the object's ATC ID rather than its call sign. The scan it reads now carries them as `GET /api/traffic` does (#1345).

## [0.155.0] - 2026-10-31

### Added

- Spotting director: `World.StartSpotting`, `POST /api/camera/spotting`. The camera picks an aircraft landing, taking off or taxiing and a spot to see it from, holds it to the end of that, and moves on, across all traffic the World sees and the player. Filters by kind, type and event; `GET /api/camera` gives `event` (#1343).

## [0.154.2] - 2026-10-31

### Fixed

- The jump trace reported simulator time catching up after a late frame as a jump: motion steps on simulator time, the trace on the clock. It now also allows for the gap before (#1341). Trace only; nothing moves differently.

## [0.154.1] - 2026-10-31

### Fixed

- A face-out stand approached from behind: the arrival carried on to the lead-in ahead of the stand and looped round behind it, over ground nothing says is apron (LOWI E1). It now pulls straight in. When it must turn, the loop takes the side nearer mapped pavement (#1339).

## [0.154.0] - 2026-10-31

### Added

- Expedite on climb, descent and speed: `climb?level=&expedite=1`, `&through=` for "expedite through", and on approach `descend` and `speed` `{"expedite":true}`, `{"throughFt":…}`. A climb and a descent are flown at 3000 ft/min that far. `traffic.Expedited`, `TaxiController.ClimbExpedited`, `ArrivalController.DescendExpedited` (#1331).
- `takeoff?immediate=1` and `takeoff?nodelay=1`: the qualifier chosen by the player, said and hurried. `traffic.NoDelayed` (#1331).
- A landing clearance names the traffic on the runway: "traffic departing ahead", "traffic landing ahead". `traffic.LandingAhead` (#1331).
- `upto?taxiway=B`: a clearance limit by name, where the route next meets it. `traffic.HoldShortOf` (#1331).
- `lineup` offered while taxiing with the runway next and within 800 m (#1331). `TaxiController.RunwayNext`.

### Fixed

- A limit at a node part-way along a taxiway was said as holding short of the taxiway it is on; now the taxiway crossing there, or the marked point (#1331).

## [0.153.0] - 2026-10-30

### Added

- `vacate` on the landing roll: `Actions` offers it in `ArrivalRollout`, `VacateAt` plans the deceleration again from where the aircraft is, and `ErrExitTooClose` says why it cannot: "too fast for A4, 84 kt with 150 m to it", or "past A4". `ErrExitFixed` now means off the runway (#1330).
- `ErrExitTooTight`: `VacateAt` refuses an exit the aircraft cannot vacate by at any speed: its wings do not fit, the turn is too sharp, or no way on from it fits. `airport.Graph.ExitTight` gives the reason (#1330).
- `POST /api/control/{id}/vacate?side=left`: the first exit on that side the aircraft can make, for a tower not minding which. It is said with the exit it takes (#1330).

## [0.152.3] - 2026-10-30

### Fixed

- A departure was assigned a runway in use that publishes no SID (EDDF 25R, with 25C in use beside it), leaving it no procedure to be cleared on. Runways with a SID are now taken while one is in use. `ControlView.NoSID` marks a departure whose runway has none, so a host can tell it from one with no procedure picked yet (#1329).

## [0.152.2] - 2026-10-30

### Fixed

- An arrival on a staffed tower or on held gates landed without a landing clearance: the crew's decision at 0.6 NM skipped every aircraft that was the player's. It now goes around and says why ("going around, no landing clearance", `traffic.GoingAroundFor`), and in the ATC game it costs 50 points (#1334).
- A landing clearance given by the player stayed good for the approach after a go-around (#1334).

## [0.152.1] - 2026-10-30

### Fixed

- A take-off starting a few degrees off the runway heading snapped to it in one frame (live at LOWI: 5° at 1 kt). The aircraft now straightens out over the first 40 m of its roll.

## [0.152.0] - 2026-10-30

### Added

- `camera.Spot`: a camera standing in the world, fixed or on a rail following its subject, the lens keeping the aircraft at a set share of the frame, with an aim smoothed from samples as rare as one a second (`LockAim` for the object itself).
- Spots at any airport from its data: `World.Spots`, `GET /api/camera/spots`, threshold, touchdown, approach, holding and apron, changed or added per value in `DataDir/spots.json`. `World.PlaySpot` and `POST /api/camera/spot` play one on any aircraft or the player; `"spot"` on a scene's shot.

## [0.151.0] - 2026-10-30

### Added

- A host plays a scene it holds: `World.PlayScene(icao, Scene)` or `POST /api/camera/scene` with the scene in the body, validated first, and `World.StopScene()`. `World.OnSceneEvent` hears start, cue, beat, shot and end; `GET /api/camera` has the progress as `scene` (#1324).
- `camera.Stretch(shot, d)` plays a shot past `MaxShot`, up to `MaxStretch` (5 minutes), its move slowed to fit. `"long": true` on a `SceneShot` or `ShotRequest` makes `sec` exact. The 10 s cap stays for everything else (#1325).
- `worldtest.Loopback` reports the camera as the simulator does (acquired, released) and `CameraPose` gives the last pose, so scenes and shots can be tested offline (#1324).

### Fixed

- A scene whose last beat is not marked `end` stopped at once, without playing that beat's shots. It now plays them (#1324).
- A scene stopped during its last beat kept running until that beat's shots would have ended (#1324).

## [0.150.0] - 2026-10-30

### Added

- `ArrivalController.VacateAt(exit)` sends an arrival off the runway by an exit the controller names, instead of the one the engine chose for the rollout: the whole ground part is planned again from it, the stand unchanged (`ChangeStand`'s mirror image). Refused once the aircraft is down (`ErrExitFixed`) — the rollout is a braking profile already being flown to the old exit, and re-aiming it mid-roll would arrive at the wrong speed or miss the turn-off.
- The `vacate` action on an arrival, with the exit named by any node on it (`?node=N`), so a map click on the turn-off, the taxiway behind it or the runway centreline where it begins all mean the same exit. A click matching no exit is refused rather than snapped to the nearest: `bestExit` already weighs what an aircraft can make at its speed, and "the nearest exit to wherever you clicked" undoes that.
- `vacate` is in `ControlView.ActionParams` as needing `node`, beside `upto` (#1308): a host is told it takes a point rather than discovering it from a refusal.
- `VacateAtExit` says it: "CSA123, vacate left via L" (Doc 4444 12.3.4.20), the side alone when the turn-off has no name.
- `traffic.ErrExitTooClose`: `VacateAt` refuses an exit the aircraft cannot slow down for, as `bestExit` passes it; the last exit is always there to take.

### Changed

- An arrival is **ground's once it is off the runway**, not once it has stopped. `ArrivalPosition` kept the tower through `ArrivalVacating` — "off the runway, rolling clear to the vacate stop" — so the handover, the crew's call and the taxi clearance all queued behind a full stop. A staffed tower could not hand it on until then either, since `handoffDue` follows this.
- An arrival's `Actions` follow its phase, as a departure's now follow its crew (#1308): `land` and `goaround` in the air, `vacate` with them before the wheels are down, nothing on the landing roll, and the ground clearances from the moment it is off the runway. This offered `taxi` and `upto` to an aeroplane on final.

## [0.149.1] - 2026-10-30

### Fixed

- `World.Hold` could stop the whole World: it took the World's lock and then the manager's, while the schedule's tick takes them the other way round (#1318).
- An aircraft whose spawn was under way when `World.Hold` ran came up beside the replay's traffic and stayed. It now goes again, and its flight is done (#1318).
- A data race reading a real aircraft's details in the snapshot while it spawned (#1318).
- A scenario's `push` on a departure already cleared failed with "already cleared for that" since v0.146.0. It is now done, with nothing said (#1321).
- A refused clearance dropped the mark that the one in force had been said, so the approval was said a second time when the state changed (#1321).

## [0.149.0] - 2026-10-30

### Added

- `ControlView.ActionParams` names the offered actions that need a query parameter: `upto` needs `node`, `climb` needs `level` (#1308).
- `ControlView.SayingUntil`: a clearance on a busy frequency is taken at once and said when it clears; until this time it is queued, so a host can show that instead of what looks like a swallowed command (#1312). `Radio.AircraftClearAt` is the same for an aircraft.

### Fixed

- `Radio.Recent` and `GET /api/radio` are in order of when transmissions go out, not of when they were booked (#1312).
- A duplicate ATC call dropped within 2 s is now logged (#1312).
- `upto` without `?node=` answers 400 and says the limit is required, instead of 422 "clearance limit is not ahead on the taxi route" (#1308).

## [0.147.1] - 2026-10-30

### Fixed

- A circuit arrival reports its base wherever it turns in. `OnBase` went by the corner of its extended downwind, which is not where it turns once sent in early, so the call was missed (#1314).
- A circuit's downwind is no longer extended once the aircraft has started its turn in. It turned back and came onto the final the long way round, with no base at all (#1314).
- The conflict log no longer reports lost separation between VFR traffic not separated where it is, unless they nearly meet (within 1 NM and 500 ft) (#1314).
- A VFR departure leaving a field without a tower no longer calls "Departure" and gets "identified" (#1314).

## [0.147.0] - 2026-10-30

### Added

- `ControlView.ClearedEarly` lists the clearances a player gave ahead of the crew: not called for yet, or before its IFR clearance. They are still taken, as a controller can expedite; a host decides whether to warn or score them (#1308).

### Changed

- A departure's `Actions` on its stand follow the crew's calls instead of the taxi state: nothing until it has its IFR clearance and has called, then what it called for. "awaiting pushback" holds from the spawn, so `pushback` was on offer through delivery (#1308). A client that waits on `Actions` to give a clearance now waits for the crew's call.
- `ControlView.Request` is what the crew has called for, not what it will call for: empty while the call waits for delivery or boarding (#1308).

### Fixed

- After a late IFR clearance, an aircraft a player already cleared ahead of it is no longer told to contact ground (#1308).

## [0.146.0] - 2026-10-30

### Fixed

- A clearance already in force was offered, and said, again (#1269). The user, live: the identical taxi clearance transmitted three times over, read back each time.
  - `TaxiController.Cleared("taxi")` now answers true while an **unlimited** taxi clearance is in force. With a limit in force (`ClearUpTo`, hold position) it stays false — there the clearance lifts the limit, which is a new instruction, not a repeat.
  - `ClearUpTo` refuses the node that is already the limit in force with `ErrAlreadyCleared`. A different node is a new limit, nearer or further on, and always goes through.
  - `World`'s clearance action refuses any once-only clearance `Cleared` reports as given (a taxi clearance in other words, a changed route or runway, still goes out; the same words answer 204 without being said, #1275), rather than transmitting a second identical instruction with its own readback. The offered list dropped it already; that was advice, this is the gate (a host clearing by name, a stale strip and a double click all arrive the same way).
- A taxi clearance was offered to an aircraft standing on its stand (#1268). The user: "the sequence of commands … cannot give taxi when it hasn't called ground".
  - `taxi` and `upto` are gone from `TaxiAwaitingPushback` and `TaxiPushback`. The crew has asked for a push, a tug is attached, and it could not go if it were told to. They come on offer at `TaxiAwaitingTaxi`, where the tug is clear and the crew calls ready (`TaxiEvent.Request` "taxi").
  - A stand the aircraft taxis straight out of is unchanged: no push to ask for, so start-up and taxi are one call and one answer.
  - An aircraft cleared to push and start is now offered nothing at all, which is correct — ATC has had its say, and the next word on the frequency is the crew's.
- `takeoff` is offered at the runway, not on the stand (#1231). It was on offer from the push onwards, which is not a thing anyone says, and it taught an order of the job that does not exist. It stays at the holding point, lining up and lined up.
- The automatic ground answer to a push-and-start request is one `pushstart` clearance instead of `startup` then `pushback`. The pair are the same two calls, but a clearance already in force is now refused — and at a face-out stand approving the start-up approves the push, so the second would have been refused and the aeroplane would have waited for ever.

### Added

- `traffic.WithReportReady(t, what)` adds what the crew is to report next to a push approval: "CSA1, pushback and start up approved, report ready for taxi", read back "wilco". It composes with `WithFacing` either way round, and leaves a start-up approval and an empty report alone.
  - It is the other half of the gate above. Withholding the taxi clearance on the stand is right, and on its own leaves a strip that went quiet with no reason given; said, the next call stops being a guess. The `World` says it with a push approval whenever the taxi clearance is still to come, the player's and the automatic ground's alike.
- `traffic.ErrAlreadyCleared`: this exact clearance is the one in force.
- `world.ControlView.Request` (`request`): what the crew is asking for right now, in the engine's own words — "pushback", "start_up", "taxi", or "" when they are waiting on nobody (`TaxiEvent.Request`, #462).
  - It is what the offered list cannot say. `Actions` names what a controller MAY give, and a departure cleared to push and start is now offered nothing at all — correctly, because ATC has had its say. A strip with no buttons and no reason reads as a stuck game; this is the other half, whose move it is.
## [0.145.1] - 2026-10-30

### Fixed

- The World no longer gives a VFR arrival the wake turbulence caution when a player staffs the tower (#1310).

## [0.145.0] - 2026-10-30

### Added

- At a field without a tower, a VFR circuit now also reports the upwind after a touch-and-go and the crosswind ("left crosswind runway 24"), and reports its legs again on the next circuit, also after a go-around. The first circuit's final is announced before a touch-and-go too. `ArrivalController.OnBase` and `OnCrosswind` tell when a circuit arrival has turned onto those legs (#1307).
- At a towered field, the tower gives a VFR arrival "caution wake turbulence" with the traffic, when a heavier aircraft is ahead of it in the sequence or landed on its runway in the last 3 minutes, once per circuit (`WakeCaution`, `WakeCautionDue`). It is said after a sequence call that names the traffic, or else with the landing clearance. At a field without a tower nobody says it (#1307).

### Fixed

- The base report no longer goes missing when the downwind is extended; it follows the moved turn (#1307).
- With no weather known, an IFR arrival at a field without a tower now cancels IFR, as unknown weather counts as good for VFR flights too (#1307).

## [0.144.0] - 2026-10-30

### Added

- At a field without a tower, the World's traffic now also announces itself inbound ("10 miles south, inbound for landing") and reports the downwind and base with side and runway ("left base runway 24"). Approach releases IFR departures ("released for departure, report airborne"), and IFR arrivals cancel with approach in good visibility (visibility of 8 km or more, no ceiling below 3000 ft, not in cloud) before the advisory handoff. Both stay silent when a player works approach. Towered fields do not change (#1305).

## [0.143.0] - 2026-10-30

### Added

- Settings for the scheduled traffic's timing (#1303). `World.SetScheduleTuning` and `GET`/`POST /api/schedule/tuning` change the wave hours, the night threshold, the morning wave, the night-stop limit, the turnaround window, the planning horizon, the leads and late limits, overnight remote stands, self-announce and the cargo share. They are saved in `DataDir/schedule-tuning.json`, which wins per value over the defaults, and bad values are refused with every problem named. `traffic.ManagerTiming`, `ManagerOptions.Timing`, `TrafficManager.SetTiming`.

### Fixed

- At an airport whose hours do not begin on the UTC hour (+05:30), the morning wave was taken to begin 30 minutes late. The night is now looked at by the quarter hour. Tests now cover its edges, the clocks going back and other time zones.

## [0.142.0] - 2026-10-30

### Added

- Our traffic announces itself at fields without a tower (#1296). Before, a tower and ground named after the common frequency cleared it there. Now the crews say `SelfTaxiing`, `SelfDeparting`, `SelfPattern` (final) and `SelfClear` on the common frequency when the World decides those moves, and approach hands a flight over with "change to advisory frequency approved". No other tower, ground or delivery call is said. `traffic.RadioOptions.Uncontrolled`, `UncontrolledField`, `SelfAnnounceFor`, `AdvisoryFrequency`, `FieldCallName`. A player staffing tower, ground or delivery there keeps the usual dialogue. Towered airports do not change.

## [0.141.0] - 2026-10-29

### Added

- Cargo traffic (#1300). The schedule had no cargo airline, so the cargo stands (#833) stayed empty. It now has six, from their hubs: European Air Transport (DHL, Leipzig), AeroLogic, Lufthansa Cargo, UPS, FedEx and ASL Airlines Ireland, with telephonies from the airline-codes table. They make a few flights a day at Prague, on cargo stands first. A cargo airline flies freighters only (`FSLTL_B748F_UPS`); the others never one, as before (`ModelsFor`).

## [0.140.0] - 2026-10-29

### Added

- Aircraft parked overnight take remote stands (#1298). Night stops and arrivals staying the night take an airliner stand without a jetway first, then a GA or cargo one, and a jetway's stand last, which leaves the gates to the day's turnarounds. The jetways come from the simulator's jetway list: `traffic.StandRequirements.Overnight` and `Bridged`, `jetway.Helpers.Jetways`. Most airports type every airliner stand GATE, jetway or not (LKPR: all of them), so without the list nothing changes. With it, types boarding by their own airstairs (`NoJetway`) now also take an airliner stand without a jetway before the GA apron.

## [0.139.0] - 2026-10-29

### Added

- Fields without a tower (#1296), most of the simulator's airports. There pilots announce themselves on the common frequency and nobody clears them; `FrequencyFor(tower)` there gives that frequency, so clients ran a controlled dialogue.
  - `airport.Layout.Controlled()`: whether the field has a tower.
  - `SelfAnnounceFrequency()`: the CTAF, else the UNICOM, else the MULTICOM; not one named for the lights only (AYBK's "ACTIVATE LIGHT ONLY").
  - `ControlledFrequency(kind)`: a controller's frequency only. No tower, and ground or clearance only on the field's own frequency (KVUO: clearance by Portland), never the common frequency.
  - `Frequency.Service`: `ctaf`, `unicom` or `multicom`.
  - Self-announce calls to "<Field> Traffic" (`traffic.PosTraffic`, `IntentSelfAnnounce`): `SelfTaxiing`, `SelfDeparting` (a direction, or `PatternStay`), `SelfInbound`, `SelfPattern`, `SelfClear`.
  - For IFR there: `ReleasedForDeparture` ("released for departure, report airborne"), `CancelIFR` and `IFRCancelled`.

## [0.138.0] - 2026-10-29

### Added

- Aircraft parked overnight (#1294). In the night (from the schedule's waves: 23–04 local by default) the morning wave's departures stand on their stands, quiet, up to half the per-airport limit, and each calls delivery 10 minutes before its STD as the others do. An arrival landing late turns into a morning departure of its airline and type and stays on its stand through the night. Before, an arrival with no turn within 3 hours was removed 20 minutes after parking and departures appeared 10 minutes before their STD: live at LKPR after 23:00 three aircraft stood on the stands. `traffic.ManagerOptions.Night`, `MorningWave`, `NightStops`; `ManagedFlight.NightStop`.

## [0.137.0] - 2026-10-29

### Fixed

- The ATC game's flights go on under automatic control when the game is over. Before, they stayed the player's: live, the game was on for 7 s and its RYR232 asked for its push at B15 for an hour and a half, nobody to answer (#1292).

- Departures take the main entrance at the runway's end first (LKPR 24: A); the others side by side with it (Z) are only the overflow once its line is full, three heading for it (#1291). Since v0.111.0 the shorter line won, each departure heading for an entrance counting as 250 m more taxi: from the C stands Z is the shorter taxi, so with one departure heading for A the next ones all went to Z (live: TVS1517 and TVS322 via H1, H, Z with only BAW1334 for A). Before v0.111.0 every departure used A.

- A push no longer waits for a neighbour parked on its stand unless its wing would sweep the neighbour's fuselage (#1289). At tight stands the 3 m margin held it until the neighbour's own push, while the neighbour's tug and fuel truck gave way to the held push: neither ever moved (live, LKPR C26 and C27: DLH1560 and TVS1517 for over 40 minutes, the fuel truck removed for not leaving). Pushed, the wing passed 14.6 m clear of the neighbour's fuselage.

- MSFS 2024's water runway surfaces (water, pond, lake, river, waste water) read as `water`, not unknown (#1290), so seaplane runways are no longer taken for land runways.

### Added

- `World.LoadAirportsFile(path, kinds...)` holds only the airports of those kinds (#1290), and `world.AirportDataDir(root, ref)` gives an airport's data folder in simconnect-data (`<ICAO>/`, or `fields/<kind>/<ident>/`). `traffic.AirportRef.Kind` carries the list's `kind` (`towered`, `ifr`, `airfield`, `short`, `water`, `norunway`, `unknown`), with `traffic.AirportKind*` constants, and `SelfAnnounce` and `GroundLayout` its flags. Without kinds, everything is held, as before.

- `engine.WithDispatchEvent()` (#855 E6): the connection is opened with a Win32 event SimConnect sets when a message waits, and the dispatcher waits on it instead of polling. A message is taken as it comes, not after a backed-off sleep of up to 50 ms, and an idle connection costs a poll every 50 ms. Off by default until checked live. With the manager: `manager.WithEngineOptions(engine.WithDispatchEvent())`.

- Assist suggestions for arrivals (#1096): an alert for two arrivals on their STARs now suggests what the AI would say first to the one landing later: a stop of its descent above the other well out, else a speed for spacing ("number 2, for spacing reduce speed to 180 knots"). Before, an alert for arrivals came without a suggestion. Suggestions are worded in the airport's phraseology, as `World.Phrase` words a clearance.

## [0.136.0] - 2026-10-28

### Added

- Staffed stations in network play (#1096 step 4): each player staffs their own. `PUT /api/stations/staffed?as=<player>` (or `X-ATC-Position`), or `World.SetStaffedBy(player, list)`, replaces only that player's stations, kept with `StaffedStation.By`. Without `as` the list replaces everything, as before.
  - A player clears the aircraft on every station they staff, so a band-box works across positions: before, a client cleared only on its own position's frequency ("not on your frequency (tower)" for a ground aircraft of a tower player band-boxing ground).
  - On a station a player staffs, only that player clears it; the position's own client is refused there.
  - A single station staffed wins over its whole position staffed by another player.

## [0.135.0] - 2026-10-28

### Added

- Staffed station modes (#1096 step 3): `StaffedStation.Mode` is acted on. On a staffed station, a predicted conflict is the player's:
  - `assist` (the default): an alert with the AI's suggestion where it has one. `World.Alerts()` and `alerts` in `GET /api/separation` list them.
  - `silent`: nothing.
  - `safetynet`: an alert with a deadline. Still predicted when it passes, the AI resolves the conflict itself and marks it: log, `safetyNet` on the resolution, `Params["by"] = "safetynet"` on its calls.
  - The deadline is at least 30 s after the alert, 1 minute after the player's last instruction, and 2 minutes before the earliest loss of separation predicted.
  - New: `world.ModeAssist`, `ModeSilent`, `ModeSafetyNet` and `StaffedAlert`. `SetStaffed` keeps the mode normalised (`"safety-net"` → `"safetynet"`, unknown → `"assist"`).

### Fixed

- On a staffed station the conflict watch no longer instructs the player's aircraft (#1096): it stopped the descent of an arrival on a staffed approach and slowed it for spacing ("number 2, for spacing reduce speed to 220 knots"), gave staffed departures crossing restrictions, and gave traffic information on the player's frequency.

## [0.134.0] - 2026-10-28

### Added

- `flight.GhostFleet.Owns(objectID)` and `Objects()`: the objects a fleet flies, for a recorder to leave replayed traffic out (#1285).

### Fixed

- `GhostFleet.Handle` takes only the objects of requests the fleet made (#1285): any object in its ID range it was not waiting for was removed, another fleet's on the same connection included, and the range check took one ID past its range.
- A World linked as an actuator spawns no traffic of its own (#1284): its schedule (set, or replayed on a new connection), corridors and real-world traffic stay off while linked, and a scenario's cast is refused with `world.ErrActuator`; schedule settings made meanwhile apply once the link ends.
- `World.Hold(true)` keeps the traffic out whatever asks for it meanwhile (#1283): a schedule set while held (`SetSchedule`, or the settings replayed on a new connection) and real-world traffic switched on are remembered for when the hold ends instead of starting the schedule; corridors spawn nothing; a scenario's cast is refused with `world.ErrHeld`. The hold is the World's now, so a new connection (`RunOn` again) keeps it. Holding again removes whatever came in meanwhile; it returned at once before.

## [0.133.2] - 2026-10-28

### Fixed

- A fast arrival behind a slow one is spaced by the speeds both really fly (#1281): the landing prediction flies an arrival at its own speed (at most 180 kt) to the join point 8 NM out, then at the injected final's speeds, 15 kt above its approach speed slowing to it 1 NM out. Before, the last 10 NM were an average of the two, so a TBM9 18 NM out was predicted 1m20s early and the B738 behind it planned that much too close (live, OKBJZ and LOT229).
- Direct to the final keeps a point halfway along the direct leg, so a delay is still lost by speed or a dog-leg before the final; a downwind extended before no longer leaves the whole delay over. An arrival on its final not yet inside `FreezeNM` with a delay is now slowed to its final approach speed, as an established one is; it was told nothing. One far more short than that speed gains is sent around at once when the separation minimum would be lost, not 20 s later and that much closer (offline: a B738 sent direct 20 NM behind a TBM9 joined ahead of it, 2.7 NM apart, and landed first).

## [0.133.1] - 2026-10-28

### Fixed

- A vectored arrival flies its base straight and is turned to intercept within 1 NM of the turn, not 15–30 s after "for base" (#1279): the intercept heading, flown from where it is told, cut the base to about 0.5 NM and turned it in onto the final early, toward the traffic behind. A corner within 1.5 NM no longer counts as the intercept turn reached.

## [0.133.0] - 2026-10-28

### Added

- `airspace.NewStore(records, opts...)` and `airspace.NewFetchStore(fetch, opts...)`: a Store over records the caller has, such as the simulator's nearest search decoded into `[]Airspace`, or one asking a fetch function for each box not inside one asked for before (#1260). `Area`, `At`, `Containing`, `ClassAt`, `StorePreferNewerSet` and `StoreSkipBroken` work as over the tiles; a record listed twice is kept once.

## [0.132.0] - 2026-10-28

### Added

- A crew refuses a runway too short for its take-off (#1232): `ChangeRunway` to another runway whose full length is less than `RequiredTakeoffRun` returns `ErrRunwayTooShort`, and in the World the crew says so on the frequency ("CSA1, unable runway 12, request runway 24 for departure", `traffic.RunwayTooShort`, intent `unable_runway`) and keeps its runway; a host amending the clearance is told the error too. Departures the schedule creates are not checked at spawn, so traffic never stops on it.
- `TaxiRequest.ISADeviationC`: the temperature above ISA lengthens the take-off run in the entry and runway checks; the World fills it from the weather (it was always 0 before).

## [0.131.0] - 2026-10-28

### Added

- After a pushback into another ground station's sector (an apron, then ground), the crew is told "when ready for taxi contact <station> <frequency>" once the push is done, then asks that station for taxi (#1268). During the push it stays with the station that cleared it; before, it was handed over with a bare "contact" as it left the sector.

## [0.130.1] - 2026-10-28

### Fixed

- A taxi clearance the same as the one in force (same route, runway and limit) is no longer said again, nor read back (#1269); it is logged and answered 204. A changed one, a hold position and a taxi after it still go out.

### Added

- `TaxiController.TaxiInForce` and `ArrivalController.TaxiInForce`: the taxi clearance in force having a given limit (none: -1).

## [0.130.0] - 2026-10-28

### Added

- Reserved airlines WTD ("WANTED") and MCR ("MyCrew"), `traffic.ReservedAirlines`, occasionally scheduled from LKPR (about 1 in 75 flights there).
  - Their codes are in neither Wikipedia's list of airline codes nor the shipped telephony table.
  - They are kept over every table: the shipped telephony and airlines, the MyCrew API's set, a local override and a schedule config's airlines.
  - They fly their own traffic models only (`FSLTL_WANTED_*`, `FSLTL_MYCREW_*`), switched on or not, never a flyable livery; without them installed, their flights are left out.
- The Learjet 25 (LJ25): type, wake (L / RECAT F), performance and schedule limits; span, length, stall speed, cruise and ceiling published, the rest estimates.
- A319 schedule limits.
- `world.Options.ModelProviders`: a host's default provider switches; the switches set by `SetModelProviders` are kept in `DataDir/model-providers.json` over them.
- `traffic.TitleOn` and `ModelProviderItem.On` (a provider's default in the `traffic.modelProviders` table).

### Changed

- Our aircraft default to the game's own models: MSFS on; FSLTL, AIG, MyCrew and other makers' aircraft off; the ground vehicles' providers (GSX) still on.
  - The airport map and `worldtest` switch FSLTL on, so the tests fly the same models as before.
- A flyable title is used only when its provider has nothing else and is switched on: a maker's FSLTL-format traffic package is no longer counted as a flyable add-on's because the same maker ships flyable liveries.
  - Where nothing switched on fits, flyable titles are taken last.
- A telephony designator written in mixed case is said as written ("MyCrew", not "Mycrew").

## [0.129.2] - 2026-10-28

### Fixed

- A VFR circuit arrival fixed at its own time passed the arrival ahead of it, which could not lose the time (live, LKPR 24: OKYQK took number 2 from CSA1293 12 NM out, which was given 6 minutes and sent around for spacing).
  - Fixed arrivals are planned before the ones not fixed, so fixing it put it ahead whatever the order.
  - An arrival fixed by its controller now stays in the order behind one that was ahead of it, while that one cannot lose the time it would cost (as for a move up: up to 3 minutes, less near its final); the ones behind take the rest.
  - Offline LKPR beds (no circuits): unchanged, no go-arounds for spacing.

## [0.129.1] - 2026-10-27

### Fixed

- An extended downwind ended in a hook: the base flew out past its turn and back onto the final (the user, live, LKPR 24: CSA188, AFR1447).
  - The intercept leg onto the final began 3 NM out from where it meets the final, beyond the base turn.
  - Extended again, the downwind ran on from that hook point, 1.5 NM beside the final, not 5.
  - The base turn now moves out along the downwind to abeam the intercept leg's start: downwind, a square base, the 30° intercept, the final.
  - It joins the final where it did, so the spacing holds. Extending the downwind more adds a little more track than before (2–3 NM a time), which the sequencer measures.
  - Offline LKPR beds: no go-arounds for spacing, delays as before.

## [0.129.0] - 2026-10-27

### Added

- Airspace class from the real airspaces (#1257).
  - `world.Options.Airspaces` takes an `*airspace.Store`. Classes B–E come from the records holding the aircraft, the strictest where several overlap. Where none covers the aircraft it is in class G.
  - AGL limits are measured over the aircraft's ground, else over the nearest managed airport's elevation.
  - Without a store, the managed airports' zone and class E apply as before.
  - `airspace.Store` gains `ClassAt`, `Containing` and a cached `Shape`. It finds records through a 1° grid of the index and skips records without limits (`NoLimits`).
  - `StorePreferNewerSet` also matches same-named records within 0.1°: Prague CTA 2 is now from 1,000 ft AGL, not MSL, and 34,964 records are kept worldwide.
- VFR weather minima (#1258):
  - `traffic.VFRMinima` (SERA.5001 Table S5-1, with FL100 and reduced-visibility options).
  - `VMCMinima.Check`: visibility only; the cloud part is reported unknown.
  - `SpecialVFRMinima` and `CheckControlZone` (SERA.5005(b) / SERA.5010(c)). No ceiling is ever assumed.
  - The schedule's VFR flights now use `CheckControlZone`.
- Airspace route check (#1259).
  - `airspace.Store.Route` lists the airspaces a path enters, in order. Each entry has its entry point, distance along, limits, designation and frequency.
  - Each entry also says what it needs of a VFR flight: B, C or D a clearance, E none, restricted or prohibited avoid, MOA, warning or alert inform. `RouteNeeds` overrides this per type.
  - `airspace.Ahead` gives a straight look-ahead path.

### Fixed

- A VFR circuit arrival moved up ahead of a jet near its final made the jet go around (live, OKJRU past DLH792 after a TCAS RA: DLH792 given 4m20s at 10 NM, sent around).
  - A move up now costs the arrival behind it no more than it can still lose: `TacticalSwapMaxCost` far out, none at `FreezeNM`, in between by distance (`MoveUpRoomNM`).
  - A circuit arrival on its downwind is not frozen inside `FreezeNM`, because it can still extend its downwind (`ApproachAircraft.Extendable`, `ArrivalController.OnDownwind`).
- VFR traffic in class D was given traffic information only, even on a near collision course (live, OKFVQ and OKJRU: predicted 0.6 NM and 223 ft apart, then a TCAS RA). VFR traffic predicted within 1 NM and 500 ft is now kept apart like IFR traffic.
- A fast departure on another route went 2 minutes behind a slow one and closed on it after take-off (live, TVS737 behind OKDWM). `DepartureIntervalSpeeds` now adds the catch-up on any route when the follower climbs `DepartureFirstKts` or more faster.
- Departures jammed on the ground: gaps of 3–4 minutes between landings carried none (live, LKPR: 7 such gaps, 0 departures; CSA1911 lined up behind the landing A320, then told to hold position).
  - The next arrival's landing time counted its speed now all the way in. CSA1080 was 8 NM out at 180 kt, counted 3 minutes, and landed in 4.
  - It now slows to its approach speed for the last `FinalSlowNM` (4 NM) (`RunwayUser.LandsIn`, `RunwayUser.FinalKts`).
- "For spacing" was said with every sequence call (live, CSA1080 four times on one approach). The reason now goes once per approach, with the first number. Later speeds and numbers go without it (`SpacingTold`, `ParamSpacingTold`).
- An IFR go-around turned 3 NM past the threshold, 1 NM past the far end of LKPR 24, near the VFR circuit's crosswind. It now climbs straight ahead past the circuit's crosswind turn with `GoAroundCircuitClearNM` (1 NM) to spare, before it turns. On a long runway that is farther than `GoAroundClimbNm`.

Offline LKPR beds, morning: go-arounds 3 → 0, arrival delay p90 30m30s → 23m30s. Evening: 29 arrivals landed instead of 27, departures about the same, no go-arounds.

## [0.128.2] - 2026-10-27

### Fixed

- Approach clearance given on the downwind (the user, live, LKPR: OKZSU "fly heading 060, for spacing", then 31 s later "cleared visual approach runway 24", its turn in never said). Since 0.128.1, a base turn already within 45° of the final counted as the turn onto the final, and the clearance came `interceptLeadNM` (3 NM) before it, still on the downwind. Turning in off a downwind (more than a right angle), the clearance now comes 1.5 NM before the turn. On vectors there is no visual approach: the ILS clearance carries the heading to intercept.
- A departure at an intersection was told to line up behind one starting its roll from the threshold, then sat lined up while the next arrival came in (live, OKSZG at B behind CSA813: it waited 31 s for CSA813 to pass B and took 75 s to line up; CSA236 was sent around, "traffic on the runway"). The room before the arrival now counts the time the one rolling takes to pass this one's way in (`RollPastIn`, at `RollAccelMS2`).
- "Behind the departing Cirrus SR22, line up and wait runway 24, behind" to TVS737 at the threshold, with the Cirrus rolling from B (the user, live). Doc 4444 12.2.7: the aircraft in a condition must be the first to pass in front. One that entered down the runway from this one's way in, or has rolled past it, never does, so the line-up is plain: "line up and wait" (`RunwayClearances.LineUpAhead`).
- After a go-around, an IFR arrival on the map showed the go-around's turning points (UPWIND, CROSSWIND, DOWNWIND, BASE) as fixes (live, DLH792). It is on vectors: only its fixes are shown. A VFR circuit arrival still shows its circuit. `circuitPoint` is exported as `CircuitPointName`.

## [0.128.1] - 2026-10-27

### Fixed

- Vectored arrivals had no heading to intercept with their approach clearance (the user, live, LKPR: "turn left heading 330, for base", then "cleared ILS approach" with no heading).
  - The base no longer meets the final square. The last leg onto the final is an intercept 30° off the final course, 3 NM long, meeting the final 1 NM outside the old turn. That is within Doc 4444 8.9.3.6 (45° or less) and FAA 7110.65 5-9-2.
  - The clearance comes as the arrival turns onto that leg, in one call with its position (Doc 4444 8.9.3.3, 12.4.2.2; FAA 7110.65 5-9-4 example order; CAP 413, the note before 6.26 and 6.26): "CSA1, 11 miles from touchdown, turn left heading 270 to intercept, cleared ILS approach runway 24, QNH 1013, report established". The crew reads back the heading and the clearance, not the position (`ParamFromTouchdown`, `ApproachClearance.FromTouchdownNM`).
  - New exported constants: `InterceptMaxDeg`, `InterceptAngleDeg`, `InterceptLegNM`, `InterceptBeyondNM`.
- "Fly heading 060, for spacing", then "turn left heading 330, for base" 17 s later (live, WZZ1042). A downwind extended by less than `MinDownwindVector` (30 s) of flight is not extended any more; speed takes the time.
- Repeated speed reductions within minutes (live, WZZ1042: 230, 210 and 180 kt). An arrival told a slower speed is not told another for 3 minutes, nor given its normal speed back sooner, nor slowed again sooner after that. Only one a minute or more short of its leader is slowed at once. In the offline evening bed, "resume normal speed" then a new cut within 4 minutes went from 12 to 0.

## [0.128.0] - 2026-10-27

### Added

- `pkg/airspace` reads the simulator's airspace outlines from the simconnect-data dump (`airspaces/`, 56,419 records worldwide). No simulator is needed, and it builds for WASM. See [Airspaces](docs/airspace.md).
  - `Airspace` is the raw record, field for field: `BoundaryType`, `AltType`, `VectorType`, `DataSet()`, `Floor`/`Ceiling` with AGL limits over a given ground elevation, and `Broken()`.
  - `Store` (`Open`, `Area`, `At`) picks the 10° tiles by the index's boxes and decodes each tile once. Boxes wider than 90° of longitude (the airspaces reaching the antimeridian) match every longitude.
  - Two cleanup options, off by default: `StoreSkipBroken` and `StorePreferNewerSet`. The second drops a record that a newer data set repeats with the same type and box.
  - `Shape` flattens the outlines: arcs every 2°, circles into 72 sides, longitudes running on past ±180. `Inside` and `Contains` (floor to ceiling) test a point.
  - Offline tests run on the real tiles: LKPR is inside Ruzyne CTR (class D, GND–1,066.8 m MSL), once per data set, or once with the newer set preferred. Also tested: a 926 m restricted circle, the antimeridian boxes, and every Czech outline against its box.
  - All 56,419 records build an outline except the 1,890 broken ones.

## [0.127.1] - 2026-10-27

### Fixed

- Departures pushed with "the fuel truck did not leave: removed" and "the tug did not arrive: pushing without it" (the user, live, LKPR evening: OKUBK, KLM999, CSA1317, QTR1005 and others).
  - A fuel truck told to leave while still driving in drove on to the wing. The push waited 2 minutes for it and then removed it. It now turns round (a U-turn away from the aircraft) and goes home. Off the aircraft it is clear at once, and near it once it is round.
  - A fuel truck was sent with `FuelMinService` (3 min) left before it had to be off the wing, whatever its drive. Live, OKKQT's truck on the far S apron was sent 5.5 minutes before then, for an 8-minute drive, and turned straight back. It now comes only if `FuelMinService` is left once it is there: its way in at `FuelTruckKts` and a quarter again (`SimObjectFuelTruck.DriveIn`). Fewer futile trips leave more of the 6 trucks free.
  - A tug held on its way in by aircraft crossing it (KLM999 at C18 waited 20 minutes for the departures queuing behind the stand) was given up as lost. `SimObjectTug.ArriveWithin` now adds the time it has been held.

## [0.127.0] - 2026-10-26

### Changed

- Arrivals were nearly all late (the user, live, LKPR: the board's estimates 3 min apart, arrival after arrival). Flown alone, an arrival lands 15–20.5 min after appearing at its STAR entry, as the schedule assumes. In traffic it took about 30. The approach sequence put a departure gap of about 10 NM in front of nearly every arrival, which held the runway to about 14 arrivals an hour. At 10 an hour, every pair the timetable brought together then queued 5–9 deep on the approach.
  - The tower now lets a departure go with the next arrival 2 NM out once the departure is airborne (`RunwayControllerOptions.LandingClearanceNM`, default 2 instead of 3).
  - The sequencer's departure gap follows (`DepartureGapArrivalNM` 4.7 instead of 6.3). Between two A320s the gap is now 8.4 NM. The rest of it is the line-up behind the landing aircraft (`LineUpBehindReady`, 95 s).
  - Offline, LKPR, 2.5 h at density 3: arrivals 8.75–14.5 min late on average instead of 16.5–19.5 in the evening, 12.5 instead of 23.8 in the morning peak. Departures were 20 instead of 26 min late in the morning, unchanged in the evening. No go-around and no runway alert in any run.
- An arrival slowed for spacing was given its normal speed back and slowed again a minute later (the user, live: RYR1647 at 210 kt, "resume normal speed", 90 s later "reduce speed to 180 knots"; 37 of that evening's 62 resumes were taken back within 4 minutes). Within 35 NM of the threshold it now keeps the speed, and farther out it resumes only with 90 s to spare (45 before). Offline, 5–8 flip-flops a run instead of 24–33. The arrivals were also less late: 7.4 min on average in the evening.

## [0.126.4] - 2026-10-26

### Fixed

- At LFPG the first departure took 2 min 8 s to start, and the whole World waited for it (#1233, Vector: WASM, simless). As an airport loads, the World works out every stand's taxi-in from the runways in use in the background (`StandAllocator.WarmTaxiIn`). LFPG has 372 stands and two runways. A WASM host runs Go on one thread with no preemption, so the warm-up held that thread to its end, and the first spawn and every tick waited behind it. The warm-up and the stand push planner now yield after each stand. Under WASM (Node), with LFPG warming, a 10 ms tick waited at most 112 ms (27 s before).
- Route searches worked out the runway crossings of each taxiway edge again in every search, sampling it against every runway. The graph now keeps them per edge (`Graph.crossings`). LFPG's warm-up takes 33 s of CPU instead of 55 s.

## [0.126.3] - 2026-10-26

### Fixed

- `ControlView.OnGround` did not mean "on the ground" for an injected departure (#1230, #1243, Vector).
  - On its stand it read false until the aircraft first moved. It is now true from the spawn, in every state before the take-off roll.
  - At lift-off the view kept the last on-the-ground frame until the next progress event, up to a second later, so a line-up behind a departure already airborne could score a runway incursion. Lift-off is now emitted at once.
- Overflights flew the shortest path along MSFS's airways and came out lopsided (the user, live: overflights north and west of LKPR only, none from the Vienna side). Doha–Amsterdam, for example, went round by Poland. The generator picks and times each overflight on its great circle, and today's 221 entered the area from all sides, the south-east most. An overflight now crosses the area on that great circle, appearing as far along it as it should be by now. Upper airspace is free route: Prague FIR since February 2021 (EUROCONTROL), flown direct rather than along airways. The airway route is kept only where the great circle gives no crossing.
- Airport map: an aircraft flown by MSFS AI had its whole route drawn from where it appeared, so the line ran back to that point and out again, two lines along one airway (the user: WZZ1467). `/api/traffic` now gives only the route still ahead of the aircraft.

## [0.126.2] - 2026-10-26

### Fixed

- After the simulator connection dropped and came back, no overflights came for two hours (the user, live: almost all aircraft gone; MSFS removes a client's AI when its pipe breaks). The schedule was set again 42 ms before the area was known, and the overflight hours asked meanwhile were marked as asked with none in them. `ManagerOptions.OverflightsReady` now holds them until the area is known; the World sets it to the picture having its centre.
- Airport map: the schedule's status badge held the whole note ("approaching — landing sequence", a cancellation's reason) and grew to a paragraph (the user). The badge holds the status alone, on one line; the note is muted text beside it.
- A push could leave the aircraft facing up a dead-end stub, its taxi-out driving to the stub's end, turning round there and coming back past where the push ended (#1227, Vector at LFPO: stand 35 facing up a 37 m stub, stand 11 up a 17 m one, 178° off the way out). The planner refused a taxi-out only when it turned back at once. Now a push whose taxi-out drives back over the push's own edge loses to any other (`pushTurnBackPenalty`), and where every push does, the push at the junction takes over, which refuses such a taxi-out too. Of 3,736 stand and runway pushes at 12 airports, the 18 that did this (LFPO 11, 17, 35; KJFK B1, F3, F7, 244, 260; EGLL 1026) no longer do, and 15 of them now end within 7° of the way out. All other pushes are unchanged. LFPO 11 still ends facing its stand, then turns 144° onto the lane. Its real push swings the tail into the stub, which needs a tighter pivot than `PushbackMinArcMeters`.

## [0.126.1] - 2026-10-26

### Fixed

- The schedule posted again (as the airport map does on a restart) dropped every overflight still to come, and their hours were never asked again (the user, live: almost no overflights, at most 7 aircraft zoomed out; 2 spawned in 1 h 40 min at 30/h). `TrafficManager.SetAirports` now keeps the overflights; it drops only scheduled flights of airports no longer managed.

## [0.126.0] - 2026-10-26

### Added

- Push-and-start is suggested at a stand, not only accepted. `ControlView.Actions` now offers `pushstart` beside `pushback`, which is what a controller says at a stand more often than the two separately. It was implemented and accepted but suggested in no phase, so a host had to promote it by hand to put it on a strip.
- `TaxiController.Cleared(action)` reports whether a clearance has already been given, named as the control API names it.

### Fixed

- A clearance already given went on being offered. After "push and start approved" the view still offered `startup`, and still offered `pushback` as the aircraft's next clearance while it stood at its stand waiting for the push it had been cleared for (the user, at Orly and at Esenboga; measured at LKPR, SWR237 offered `startup` from the push through `awaiting taxi`, while TVS316 given the two clearances separately behaved). `departureActions` answered from the taxi state alone, and the state cannot tell the two apart — "awaiting pushback" looks identical whether the push has been cleared or not. It now asks the controller, and withholds only the once-only clearances: pushback, push-and-start, start-up, line-up and take-off. Taxi, a progressive limit, hold position and a runway crossing all mean something when said again and are never withheld, and aborting a take-off puts the line-up and take-off back on offer by itself. The clearances themselves were always correct.

## [0.125.2] - 2026-10-26

### Fixed

- A departure parked on a tight stand held up the taxilane behind it for good (the user, live: WZZ1469 on LKPR C27, its A321 tail 13 m from H1, stopped AFR1002 for 18 min while its own pushback waited for AFR1002; it moved once WZZ1469 was removed). An aircraft parked on its stand, not yet pushing, no longer stops taxiing traffic (`GroundPicture.ReportParked`); its pushback still waits for traffic behind the stand.
- A VFR arrival told "extend downwind, I'll call your base" had its base called by position alone, the traffic it followed not past yet (the user, live: OKCJV turned in ahead of the Citation OKDIT it followed; OKDIT, never cleared to land, went around). The tower now calls the base once the traffic ahead is on the final inside its base turn by its spacing; till then it flies on downwind a mile at a time (`ArrivalController.HoldDownwind`, to `HoldDownwindMaxNM` 8).


## [0.125.1] - 2026-10-26

### Fixed

- A take-off ahead of an arrival could leave the runway with the arrival well inside 3 NM, its landing clearance late (live LKPR, OKJTX cleared for take-off while still lining up, CSA1312 close behind). A departure or line-up now goes only when the runway is free with the arrival still `LandingClearanceNM` (3) out, counting the take-off clearance's own ~15 s on the frequency (`takeoffSaidIn`). The line-up is timed from where the departure is: `TaxiController.LineUpLeft` (its path from the holding point, or what is left of it while lining up; OKJTX from B took 66–83 s, counted at 30), the fixed allowance (`lineUpGoSlack` 20 s) only when that is unknown. An arrival lined up on the extended centreline up to 15 NM out is measured straight in, not along its route. The sequencer's departure gaps are sized to match (`DepartureGapArrivalNM` 6.3, `LineUpBehindReady` 95 s, `DoubleGapExtraNM` 5.5, the extra counted for the second departure; `RushArrivalWithin` 5 min). Offline LKPR: no landing clearance under 3 NM after a departure, departures within the old range.
- A widebody's runway exit was planned as an A320's: the user saw a 777 head for LKPR D, braking at its limit. One that touches down faster than an airliner and brakes at 2.0 m/s² or less (`exitHeavyDecel`) is planned at its own touchdown speed: a B77W takes E on 24 and A on 06. Narrowbodies (A320 at 130 kt, B738 at 140) keep the airliner's plan.
- An arrival given spacing on a dog-leg (`AbsorbWithin`) was turned off its downwind when extending it was the real answer (the user: UAE364 should fly heading 060, then turn base and intercept). The downwind is now extended for spacing too when its turn comes before the meeting point (`downwindMeetSlackNM` 1): "fly heading 060, for spacing", then "turn … for base".
- Approach started vectoring an arrival off its STAR without saying what for (the user). The first vector now carries the approach: "vectoring for ILS approach runway 24, fly heading 060, for spacing" (`traffic.Vectoring`; SKYbrary, Radar Vectoring for Approach, from Doc 4444).
- A landing aircraft slowed to taxi speed with its nose just off the runway and held it until its tail was clear (offline LKPR, an A320 at 12 kt on D, the arrival behind cleared at 1.0 NM). It keeps its exit speed until it is clear as the tower counts it, 60 m on, then slows: clear 53 s after touchdown instead of 57.
- "Behind the departing Boeing 737, line up and wait" was said with the 737 about to lift off (the user, live). With the one ahead at 80 kt or more on its roll, or off the runway, the line-up is a plain "line up and wait".


## [0.125.0] - 2026-10-25

### Added

- VFR circuit traffic absorbs its delay by cost: the downwind extends up to 5 NM (`CircuitWideExtendNM`, per aircraft with `SetCircuitExtendMax`) when no arrival follows within 9 NM; else the VFR goes ahead of an IFR leader not yet established 15 NM or more out, which loses the time by speed and path; else it keeps its time and those behind take the delay. Never an orbit, another circuit or a go-around.

### Fixed

- Arrivals stopped for up to 19 minutes after a spawn failed: an arrival that could not appear (no model of its type) kept its landing slot, its estimate sliding on with the clock, and every arrival behind it was delayed (offline LKPR, a DA42 with no model). A failed arrival holds no slot until it appears, and the schedule leaves out the types the simulator has no model of (said once per type).
- A take-off clearance on an empty runway waited behind the tower's answers to check-ins ("report ready for departure", "continue approach"), which shared or outranked its class and were older (offline LKPR, TVS1430 cleared 29 s late). Those answers now go after the runway's calls (`prioCheckInReply`).
- "Line up and wait behind the departing …" was followed at once by "hold position": the line-up behind one rolling was given without its own line-up time, which the recheck counts (offline LKPR, WZZ1040). The line-up time now counts, overlapping the interval.
- Two departures lined up at once: one told to line up behind the landing traffic did not count as next on the runway, and another holding point got the same clearance 0.4 s later (offline LKPR, CSA693 and THY675); and one cleared for take-off still turning onto the runway at an intersection counted as "the departing" for a line-up behind it (OKYQT, a DA-62 lining up at B for 80 s). Both now go one at a time (`RunwayUser.LiningUp`).
- A tug backing off after a push drove into taxiing traffic: its 15 m back-off had no traffic check (offline LKPR, the tug of CSA311 5 m from TVS698). It now gives way like the other vehicles.
- A tug waiting for an aircraft crossing its road crept on 3 m each look until it stood 9 m from it (offline LKPR, the tug of WZZ1639 and CSA1955): a vehicle within `VehicleCommitMeters` of the aircraft's path was let on. A vehicle now waits short of the path, and one already in it drives out the way it is going. `VehicleCommitMeters` is deprecated and unused.
- The runway in use flipped 24↔12 four times in a minute after a start with the crosswind at the margined limit: during the first minutes the runway in use is kept while it is still the choice without the margin.
- Overflight levels: route levels were the plan's altitudes rounded, with no direction rule (KLM1433 westbound climbed into DLH1506 eastbound at FL390). Every en-route point now sits on the semicircular level for its track; and two aircraft holding their levels within ±300 ft, 1000 ft or more apart with flat profiles ahead, are no longer predicted to lose separation.
- The landing order flipped: an arrival missing from one look (a handover, a frame with no position) left the sequence and came back last. A missing arrival keeps its place for `SequencerOptions.GoneGrace` (2 min); within the swap margin, arrivals numbered alike keep their order by key.
- An arrival back after a look missing was put behind the one that took its number for that look, at their merge fix, and its first-come key was reset; the two then swapped by turns (offline LKPR, TVS993 behind TVS1137 after a reroute, TVS1137 went around twice). Numbered alike, the one with the earlier key is the one that was ahead.
- Spacing go-arounds after compression on the final: the go-around compared with the unit's 5 NM spacing, not the separation minimum (offline LKPR, THY1537 sent around 4.4 NM behind with 3 NM the minimum); it now uses `SequenceEntry.SeparationNM`. A departure gap no longer goes to an arrival within 25 NM of its final that cannot lose the time (`PlanAbsorption`); the next one takes it.
- The spacing speed's cost counted the whole way in; it now counts up to the final (10 NM), where every arrival flies its approach speed. Spacing state is dropped when an arrival leaves or rejoins the sequence.

- Service vehicles drove through aircraft standing still on a taxiway: they gave way only to aircraft with a path ahead, so one stopped behind traffic or at a hold-short counted as parked (live, a fuel truck through CSA429 stopped on H1). An aircraft taxiing (its intent reported), pushing or pushed and waiting to taxi now counts by its body; one parked on its stand still does not. The way a pushed aircraft will taxi once cleared is not given way to, only its body.
- Ground told an aircraft to "give way to the Boeing 737 passing left to right" for one pushed onto the same taxiway ahead of it (live, CSA429 behind TVS1169, both via H1 and H). When the other's way runs along its own the same direction for 100 m or more (`GroundPicture.SameWay`), it is told to follow it ("follow the …") and gives way to it as a leader.

## [0.124.3] - 2026-10-25

### Fixed

- A spacing speed was given back ("resume normal speed") as soon as the arrival, at that speed, landed 45 s later than its spacing asks, so at its normal speed it was short again and slowed again: 190 kt, normal, 180 kt, normal, the delay never lost, and it reached the final 4 NM behind a slower one and was sent around (offline peak, RYR1616). The speed's own cost over the way to go now counts, and it is kept until the spare covers that too. Offline peak: no go-arounds (was 1), "resume normal speed" 8 times (was 23).

## [0.124.2] - 2026-10-25

### Fixed

- A circuit aircraft on its second circuit (after a go-around or a touch-and-go) was not established on its final until the 0.5 NM takeover, so its landing clearance came late again. `CircuitFinal` now holds off only until it is round to its base.
- A refused tower line-up, take-off or line-up-behind is no longer kept as given: the departure is not left `LineUpCleared` at its holding point with every other departure waiting behind it.
- `go generate` writes `remote_gen.go` without a trailing blank line (gofmt).

## [0.124.1] - 2026-10-25

### Fixed

- A departure lined up waited for VFR circuit traffic still joining downwind (live, DLH555 lined up for 6 minutes for OKGKF, two aircraft on the runway, OKGKF sent around). The take-off conflict check no longer counts circuit traffic: the tower gives traffic information, and the circuit aircraft's landing is the runway rules'.
- A circuit aircraft sent around from the injected final (or climbing out of a touch-and-go) was taken over by the approach again within seconds and landed onto the aircraft on the runway (live, OKGKF 11 s after "go around", OKDCK). It now flies its circuit round to the final first.
- Circuit traffic counts as established on its final once it is turned final within 3 NM, not only at the 0.5 NM takeover, so the landing order and landing clearance no longer wait for the last half mile.
- Two departures lined up at once from two holding points (live, TVS1608 at A and RYR257 at Z): one told to line up but still holding short (`RunwayUser.LineUpCleared`) goes next, and no other departure is lined up past it. For the arrivals it is still holding short.
- Answers to crews' calls (check-ins with the tower and departure, the weather, runway changes) go on the frequency's agenda in their priority class instead of on a timer ahead of more urgent calls (live, a "report ready for departure" said before a circuit call that had waited 53 s).
- `CircuitMaxExtendNM` is 3 NM (was 5): further out the base crosses under the ILS final.

## [0.124.0] - 2026-10-25

### Added

- Low visibility procedures hold departures at the ILS hold-shorts: `RunwayEntry.ILSHoldShort` and `RunwayExit.ILSHoldShort` (the CAT II/III line beyond the runway hold-short on the same taxiway, -1 none), `RouteOptions.LowVisibility` (the route to an entry ends there; `RouteToRunway` takes ILS holds first), `RunwayEntry.HeldAt`. The airport map sets it at a departure's spawn from the weather (visibility below 550 m or ceiling below 200 ft); the line-up, backtrack and entry checks know the ILS line.
- Stand areas (`traffic.standAreas`, a `pkg/dict` table with a local override per area, `StandAreaFor`, `StandAreas`, `Schengen`, `StandRequirements.Other`): which flights take which stands where the scenery has no airline codes, by airline and/or the Schengen side of the flight's other airport. Shipped: LKPR A and B non-Schengen, C Schengen (prg.aero). The map passes each flight's other airport.

### Fixed

- A circuit aircraft sent around on its base or final flew on down that final, was taken over by the injected approach and landed onto the aircraft lined up (live, OKQAK, OKDCK). It now goes straight over the runway at circuit height, and the approach takes it over only once it is round on its base again.
- The sequence no longer sends circuit traffic round another circuit (live, OKQAK told "extend downwind", then "make another circuit" 20 s later): the downwind is extended up to `CircuitMaxExtendNM` (now 5 NM, was 2) with "I'll call your base", also when closing on the one ahead; another circuit and orbits are for the player's commands and emergencies.
- "Line up and wait" from a holding point needs the arrival to leave `LineUpWaitTime` (85 s, lining up to a stop) instead of the 60 s of line-up and go (live, OKWMV at B lined up 100 s after its clearance, AFR765 went around).
- "Behind the departing …, line up and wait" only behind one cleared for take-off, never behind one lined up waiting for an arrival (live, RYR1416 lined up behind OKWMV: two on the runway).
- A go-around from the tower is flown on "go around", not after the readback (live, 12–16 s more down the final).

## [0.123.0] - 2026-10-25

### Added

- `addons.Install` gains `Version` and `Dir`, read with the simulator closed: Steam's build id and game folder from `appmanifest_<appid>.acf` in the library `libraryfolders.vdf` names, or the Store package's version and `PackageRootFolder` from the per-user package repository in the registry. `addons.SDK()` returns the MSFS SDK's folder (`MSFS2024_SDK`, else `MSFS_SDK`) and version (`version.txt`). Files and the registry only, with no dependency (the registry through `advapi32`).

## [0.122.1] - 2026-10-25

### Fixed

- A departure gap on final includes the speed difference, as the normal spacing does. A follower faster than its leader closes the gap all the way down, so the gap is now set up wider by the same compression (`CompressionNMPer30Kts`). Behind a slow leader it is still there at the threshold (live, LKPR: CSA814 set up 6.3 NM behind a Citation at 110 kt had closed to 4.6 NM at 7 NM to go). Behind a Citation an A320's gap is now 7.2 NM, was 6.2.
- A go-around for spacing is the last resort, only when the follower would land closer than the minimum behind its leader. That is measured from where the two are: their distances to go and speeds, at the leader's touchdown. It is no longer called for a departure gap or a buffer lost (CSA814 would have landed 3.8 NM behind, the minimum 3, and was sent around for the 6 NM of the gap).
- "Reduce to final approach speed" is no longer said to an arrival already going around (CSA814 was told it 15 s into its go-around, the frequency busy).

## [0.122.0] - 2026-10-25

### Added

- Route variety: `nav.FlightPlanRequest.Variant` picks among the near-equal ways across the airways, not always the shortest. Up to 3 entry fixes, exit fixes and airway routes are considered, each no more than 3% longer than the best choice. `AirwayGraph.Routes(from, to, k, extra)` gives the alternative routes: each is found by searching again with the segments of the ones before made costlier. The traffic engine passes a hash of the call sign for arrivals, departures and overflights, so a flight planned again flies the same way, and one flow spreads over parallel airways (live, LKPR: 36 of 80 overflights on one chain, L617 T710 T707 N869, north-west of Prague; offline, the south-east/north-west flow had 57% of its plans on N869–T707, now at most a third on any pair of airways). Variant 0 is the shortest, as before.

## [0.121.2] - 2026-10-25

### Fixed

- The landing sequence predicts a VFR arrival with its own lead: it appears `VFRLead` (8 min) before its STA to join the circuit, not `ArrivalLead` (25 min). Waiting to appear, a light aircraft was predicted to land `ArrivalLead` from now on every tick, and every arrival behind it in the sequence slid on with it (live, LKPR morning peak: the queue headed by light aircraft, airliners estimated 20-30 minutes late). The spawn delay it advises uses the same lead.

## [0.121.1] - 2026-10-25

### Fixed

- Schedule waves follow the airport's clock, not the sun: airline timetables run on civil time, summer time included. On solar time from the longitude Prague's morning and evening peaks ran an hour late all summer, Madrid's two hours, London's one. `traffic.LocalHour` and `AirportZone` give an airport's hour and zone (by ICAO prefix, the contiguous US by longitude, Arizona without summer time); a region they do not know keeps solar time. Scheduled, business and overflight waves use it. The zone data is embedded (`time/tzdata`).

## [0.121.0] - 2026-10-25

### Changed

- The facility enums are written to JSON by name instead of number, for a readable data model: the constant without its type prefix (`"GATE_HEAVY"`, `"GATE_B"`, `"HOLD_SHORT"`, `"PAINTED_LINE"`, `"LEFT"`, `"ILS"`, `"AT_OR_ABOVE"`; a leg type is its ARINC 424 code, `"TF"`, and `"NONE"` for none). Parking type and name, runway surface and designator, taxi designator, frequency, approach, helipad, taxi point and path types, leg types and altitude descriptors in `pkg/types`, and `nav.WaypointType` and `nav.AirwayType` (`"NAMED"`, `"JET"`). Reading takes the name or the number, so airport caches, layout and procedures dumps written before load as they are; a value without a name is written as its number. **Wire change:** a consumer comparing these fields with numbers must read the names (the airport map's pages turn them back into numbers as they load the airport). `types.MarshalEnumJSON` and `UnmarshalEnumJSON` do it for other enums.

## [0.120.0] - 2026-10-24

### Added

- Overflights are configurable on `POST /api/schedule` (and `ScheduleSettings`): `overflightsPerHour`, the peak hour at density 1 and scaled by the density (default 10, was 6: 30 an hour at density 3), and `maxOverflights` at once (default 20, was 10; `TrafficManager.SetMaxOverflights`). `GET /api/schedule` shows both. Overflights no longer count against `maxAircraft`: a sky full of them took the airports' room.

### Fixed

- Flights to and from airports beyond the airways known around the airport flew direct from the SID's last fix, or direct to the STAR's first: every LKPR departure to Paris, Madrid and Rome went one line south-west after DOBEN, and every arrival from the west came straight to LOMKI. `nav.Plan` now flies the airways to their edge toward the other airport (the fix nearest it that the airways reach within `EnrouteMaxStretch`, stubs at the crawl's edge passed by) and goes direct only beyond. Overflights between two such airports join the airways where they cross: LKPR → LFPG is now "DOBE4A DOBEN T136 NIRGO … T892 BETEX", LEMD → LKPR "DCT HOC Y112 MOMUK … P733 LOMKI LOMK8T".

## [0.119.0] - 2026-10-24

### Added

- The timetable is planned within the runway's capacity, as slot coordination does (`CoordinateSlots`, `SlotCapacity`): in order of their time, each flight takes the first 10-minute bin with room (3 arrivals, 3 departures, 5 movements by default), its STD and STA moved on together, up to an hour; VFR, real and replayed flights keep their times. The World plans each hour of its timetable so (live, LKPR: 8 arrivals due in 15 minutes, the last landing 20 minutes after its STA).

### Fixed

- The landing flow opened gaps for every departure taxiing, even ten minutes from the runway: it counts those in the runway queue now (`ManagedFlight.AtRunway`). In an offline 90-minute LKPR peak, arrivals land a median 3 minutes after their timetable time (was 14).

## [0.118.5] - 2026-10-24

### Fixed

- Arrivals came one at a time: one landing in 15 minutes at the 18:00 peak. The landing flow predicted an airborne arrival's landing from `Since`, and the World calls `Progress` for every mile it comes closer, which moves `Since` on, so the predicted landing slid along with the clock and every arrival behind it was pushed on and not spawned. The prediction now counts from when its status began. In an offline 90-minute LKPR peak, with `Progress` reported as the World reports it: 22 of 24 arrivals instead of 7.
- The departures' gap in the landing flow was opened again in front of every arrival, so arrivals came 6 minutes apart. The gaps now follow the queue: one ahead for every three departures waiting, the gaps already ahead counted (one goes between each two arrivals, two in a gap).
- A departure lined up and waited out its interval behind the last departure without counting that wait against the next arrival: once free to go, the arrival was too close and it stayed on the runway (live, LKPR: AUA511 lined up 28 s behind a Phenom; KLM1683 went around). It lines up only if it can still go before the arrival once the interval has run.
- A take-off clearance granted to a departure lined up was withdrawn while the call waited for a busy frequency, as the arrival came inside 3 NM; the departure sat on the runway and the arrival went around. Granted, it stands: down to 2 NM for one lined up, and 15 s inside its time rule for either, so a decision a second later does not withdraw it (live, LKPR: TVS850 "line up and take off" withdrawn with the arrival 2m14s out).
- Radar told an IFR arrival back from a go-around to "resume own navigation direct BASE": a go-around circuit's own turning points (UPWIND, DOWNWIND, BASE, FINAL) are no fixes. It is given the heading for base instead (live, LKPR: KLM1683).
- Overflights stopped after four: `MaxOverflights` defaults to 10 (was 4), enough for the 15 to 18 an hour the timetable has.

### Reverted

- The scheduling changes of 0.117.2 to 0.118.3 (runway capacity cap, fair share of the aircraft cap and its look-ahead, arrival maximum delay, landing flow worked out afresh, landing slots by STA). They chased the symptoms of the two bugs above and the doubled timetable fixed in 0.118.4, and broke the traffic mix live. Kept: overflights seeded per hour, the power-out without a loop, and the timetable asked again only for a new airport.

## [0.118.4] - 2026-10-24

### Fixed

- Posting the schedule again with the same airports doubled the timetable. Every `SetAirports` asked the Source again for the hours ahead, and the World's Source keeps call signs unique, so each ask brought other flights for the same hours on top of those already there (live at LKPR, the schedule posted repeatedly: 580 arrivals and 330 departures due in two hours, 45 arrivals at one STA, cancelled by the hundred). The hours asked are now kept per airport, and only a newly managed airport is asked again.

## [0.118.3] - 2026-10-24

### Fixed

- Arrivals still came about one in ten minutes against ten departures. The landing flow ordered arrivals by their predicted landing. An arrival already late has the prediction "now plus the flight", which slides on with the clock, so every arrival due later went ahead of it. It never came, and was cancelled an hour after its STA (live at LKPR: STA 12:00 estimated 15:06). Arrivals still to come now take, in STA order, the first slot at or after when they could land, first come first served by the timetable. Those in the air keep theirs, and the departures' gap stays reserved.
- A light aircraft leaving a self-manoeuvring stand made a full circle. The way out always turned it round, as if its route began behind the stand, and it then came back round to a junction just ahead (live at LKPR S17: facing 265°, its junction 15 m ahead at 298°, OKVBB). With the route's first junction within 100° of the nose, it now taxis straight on in one gentle turn.

## [0.118.2] - 2026-10-24

### Fixed

- The field's own carrier leads its own field (#1193). Presence on the stands was a flag — "named on any stand" — and it earned the full home-carrier eightfold, so at a hub most of the built-in carriers claimed it at once and it cancelled out. Measured over 480 flights at EDDM: easyJet 11%, KLM 10%, Lufthansa 9% — easyJet outflying Lufthansa at its own base — and Lufthansa Cityline, on 86 of the 180 stands, nowhere in the top twelve. How much of the parking an airline holds now decides, as a fraction of the biggest holder, applied once to the built-in carriers and the stands' own alike; `Bases` keeps the full multiplier as the explicit "this is its base" signal. A stand shared by a pool counts as a share of one stand: Munich's stand 0 names 23 airlines and stand 100 names 28, a common pool rather than a gate each, while stand 102 names Lufthansa and Cityline alone. EDDM is now Lufthansa 9% and Cityline 5% with the next carrier at 2%; LKPR (TVS 30%, CSA 23%) and EGLL (BAW 34%) are unchanged, having already been right — few foreign airlines hold stands at a smaller field, which is why this only bit at a hub.

## [0.118.1] - 2026-10-24

### Fixed

- No stub radio line after a clearance (#1164), which 0.118.0 announced and did not do: the guard went on `case "handoff"` instead of `case "clearance"`, where it read correctly and changed nothing. With no clearance due, `phraseView` went on falling through to the catch-all at the end of its switch, which builds a transmission out of the action's NAME — `{"position":"ground","callsign":"UAE168","intent":"clearance","text":"UAE168, clearance"}`, neither the words nor the position. Measured on the 0.118.0 tree: 122 of them in a two-minute run at LKPR.
- `GET /api/control/{id}/{action}/phrase` answers an empty `ClearancePhrase` when the action has no phrase right now. It worded whatever it was given, and wording an empty transmission rendered the empty call sign and intent into the phraseology's own punctuation — `", "` — so a sentence nobody would say became a fragment nobody would say, and a host logging the phrase still showed a line.

## [0.118.0] - 2026-10-24

### Added

- The schedule flies the airlines an airport's stands name, which the built-in list lacks (#1172): Munich's stands name 105 carriers, Lufthansa Cityline on 86 of them, against 18 in the list. A home carrier is weighted to dominate its base, and a stand's fleet is filtered by the span the stand takes.

### Fixed

- Still almost no arrivals, about one in 20 minutes, against 38 departures in 75 minutes. The landing flow's delay ratcheted:
  - Each pass, a waiting arrival's ETA started from its own previous delay, not its STA.
  - The delay could only move later, never earlier.

  So an arrival delayed once fell behind the others each pass. Live at LKPR, STA 09:55 was estimated at 16:17, and such arrivals were cancelled an hour after their STA. The flow's delay is now worked out afresh from the STA each pass: it moves earlier as well as later, and goes when no longer given.
- No overflights for hours. `traffic.Overflights` seeded its random draws the same way on every call, and the manager asks for one hour at a time, so every hour drew the same flights. The manager dropped them as repeats of the first hour's call signs: three overflights at the start, then none for two hours. Each hour now has its own seed.

## [0.117.3] - 2026-10-23

### Fixed

- Still no arrivals after v0.117.2. The fair share of `MaxAircraft` counted only flights due at that moment. Departures board on their stands well before the arrivals of the same hour are due, so they filled the cap before any arrival was due. It now counts the other kind's flights due within 30 minutes.
- An arrival the landing flow kept delaying was chased for hours: its estimate slid later each time, so it was never "too late". Live at LKPR, 145 arrivals were queued 2–3 minutes apart, estimated as late as 03:45, and none spawned. A scheduled arrival not in the air an hour after its STA (`traffic.ArrivalMaxDelay`) is now cancelled.
- Arrivals came about five an hour. Every spawn attempt took the 3-minute arrival spacing, even one that failed with nothing appearing: an enroute spawn falling back to the STAR entry (common in MSFS 2024), or a type with no model installed (a BE58) retried three times. A failed attempt no longer uses up the spacing, and "no model of a <type>" now cancels the flight at once (`ErrSpawnImpossible`) instead of retrying.
- "landing flow" and "landing flow, gap for N departures" (N moving) counted as different reasons, so each change logged again. The count no longer makes a new reason.

## [0.117.2] - 2026-10-23

### Fixed

- No arrival at all in the morning peak at a high density. Live at LKPR, at density 3: 41 of ours, all departures, with 12 waiting to push, 10 taxiing and 16 still counted while climbing away. Two causes:
  - **Unreal traffic:** the timetable made 72 movements in the peak hour, about double what one runway in use takes. A focus airport's movements are now capped at `ScheduleOptions.CapacityPerHour` (`traffic.DefaultCapacityPerHour`, 40) whatever the density.
  - **The aircraft cap was first come, first served:** departures, on their stands early and counted while climbing away, took all of `MaxAircraft`. Now neither arrivals nor departures take more than 60% of it while the other kind has flights due.
- "arrival delayed — landing flow" was logged 40,000 times in an hour and a half, every scheduled arrival once a minute. A delay is now announced again only for another reason, or when it grows by 5 minutes.

## [0.117.1] - 2026-10-23

### Fixed

- A placeholder weather reading while the simulator loaded held the wrong runway for 40 minutes. Live at LKPR, the first reading was 090°/10 kt and 15 °C, putting 06 in use; the real 140°/6 kt came a minute later and favoured 24, the preferred runway. The 30-minute wind memory kept the placeholder. Now, after each connection to the simulator (`nav.RunwaySelector.Resettle`), the choice follows the weather for `nav.RunwaySettle` (3 minutes) and remembers none of it. A runway given with `Seed`, or already in use at the reconnect, is kept through the settle while within its limits. A zero-value selector settles only after `Resettle`; hosts with their own selectors call it after each connection.
- `hotkeys`: SimConnect refused every key mapping with exception 1. Each action's client event is now declared (`MapClientEventToSimEvent(event, "")`) before its input is mapped, as the SDK's input sample does; checked live in MSFS 2024. `hotkeys.Client` gains `MapClientEventToSimEvent`, which the engine and the manager already have.
- A jetway dock refused (the gates still filling, no helper the simulator gives the gate) was retried every second without a trace. The reason is now logged once per gate: "jetway LKPR B14: no dock for DEMO014 yet: …".

## [0.117.0] - 2026-10-23

### Added

- Jetway helpers with a smaller aircraft's door (`jetway.HelperDoorTitle`, `PackageOptions.DoorOf`). At a gate bigger than the others (LKPR B14, C22, C24), a narrowbody docks with a wider helper, so the simulator keeps that helper on this gate and not on a smaller gate it would take a jetway from. That wider helper's jetway docked to its own door, higher and further back: at B14, a 747's door above and beside a B738's. The variant is as wide as the 747 but has the narrowbody's door, so the jetway meets it.
  - Used for the aircraft's own door, else an A320's, when the simulator lists the variant.
  - `examples/jetway-helpers -door-of A320,B738` (the default) writes those two doors on each widebody helper (wing span over 150 ft). Write the package with MSFS closed; it loads at the next start.

## [0.116.0] - 2026-10-23

### Added

- Staffed departure (#1096 step 2): a departure handed to a staffed departure climbs to the level of its clearance and levels there, not on to its cruise. About 30 s level, its crew asks "request climb to flight level 360", again every 3 minutes. The view offers `climb`: `POST /api/control/{id}/climb?level=24000` clears it on ("climb to flight level 240").
- Staffed approach, no further clearance (#1096 step 2): an arrival cleared its STAR but not its approach asks about 3 minutes before the STAR's last fix, its clearance limit: "Airfrans 1288, approaching ERASU, request approach clearance". It then holds there at its level, right turns on the course it came in by ([FAA AIP ENR 1.5](https://www.faa.gov/air_traffic/publications/atpubs/aip_html/part2_enr_section_1.5.html)). The view offers `approach`: the player's "cleared ILS approach runway 24" takes it out of the hold onto the approach. Back to the AI, the hold is left as well.
- On a staffed station, the crew goes around without a landing clearance from the player, as it does without one from the AI. Only "cleared to land" permits a landing ([SKYbrary, Landing without ATC Clearance](https://skybrary.aero/articles/landing-without-atc-clearance)). The player's `land` is that clearance.
- `traffic.RequestClimb`, `IntentRequestClimb`, `traffic.RequestApproachClearance`, `IntentRequestApproach`.

### Fixed

- A climb cleared after the hand-over (`TaxiController.ClimbTo`) reaches the new level at 2,000 ft/min, not by the next waypoint tens of miles on, where it climbed at a few hundred feet a minute.
- A push not yet started waits for a taxiing aircraft stopped a moment whose route comes through its corridor within 120 m. That aircraft goes first rather than stopping again for the push. Live at LKPR: TVS439, just cleared to taxi, gave way to TVS512 pushing across its way, and EZY489 waited 1.5 minutes for TVS439's push.
- Ground no longer tells an aircraft to give way to an arrival turning into its stand ahead ("give way to the Airbus A220 passing left to right" for CSA1065 turning in to C18).
- "pushback holding for … behind the stand" was logged several times a second while the object in the way changed between an aircraft and its tug; it is now logged once per aircraft.
- The head-on look at two routes (v0.115.1) runs once per pair of routes, not every 30 s, and its log line is written once.

## [0.115.1] - 2026-10-23

### Fixed

- Two aircraft taxiing head-on along one taxiway could meet in the middle, neither able to give way. Live at LKPR, TVS681 came in along F to C24 while CSA398 went out along F from C28. Three general rules, any airport:
  - **Whole routes:** who is coming the other way is now judged on both whole routes, not the next 400 m. The aircraft not yet on the shared stretch holds at its last junction before it; CSA398 waits on H1 short of 894. The check runs once a second on a grid, which costs less than the old check ten times a second.
  - **Plan up front:** ground re-routes one of the two round the shared stretch, the one further from it first, when there is a reasonable way round. The last 250 m to its own stand or runway is exempt, since there is no way round there.
  - **Short detours only:** a re-planned taxi route is at most 50% and 500 m longer and crosses no runway the old one did not; otherwise the aircraft keeps its route and holds. This applies to re-plans round a pushback too.
- Arrivals can now be re-planned on the ground with a runway crossing still ahead; the crossings are planned afresh with the new route.

## [0.115.0] - 2026-10-23

### Added

- Staffed approach (#1096): an arrival's first call to a staffed approach goes unanswered, and its STAR clearance waits for the player's action `clearance`, as delivery's does. The crew flies its flight plan meanwhile.
- Handovers out of a staffed station (#1096): leaving a staffed station, the crew stays on the player's frequency. The view offers the action `handoff`, and the player's "contact Ruzyne Tower 134.56" transfers it. `World.Phrase(id, "handoff")` gives the wording. A station with `AutoHandoff` still hands off by itself.

### Changed

- Hosts staffing stations without `AutoHandoff` must now hand their aircraft over themselves; until v0.114 the AI did it. Set `AutoHandoff: true` to keep the old behaviour.

### Fixed

- The player's `clearance` was preceded on the radio by a stray "Airfrans 1288, clearance".

## [0.114.1] - 2026-10-23

### Fixed

- `worldtest`: the user aircraft was parked on the airport's reference point, which at LKPR lies on runway 06/24. The tower held every departure ("Player on the runway") and sent every arrival around. It is now parked on the smallest stand.

## [0.114.0] - 2026-10-23

### Added

- An amended departure clearance (#1164). `POST /api/control/{id}/clearance?runway=06&sid=…&level=4000`, or `World.GiveClearance(id, ClearanceAmend)`, gives a staffed delivery's clearance:
  - off another runway, re-planned with the SID named, or else the SID off that runway from the same fix;
  - with an initial climb lower than the SID's, which tower then keeps.

  The clearance is said as applied, for example "DOBE 7D departure, runway 06, climb via SID to 4000 feet". A SID not off the runway is refused, and the clearance keeps waiting. Without parameters, the clearance is given as filed.
- A level on the approach API (#1164): `POST /api/approach/{icao}/{callsign}/descend {"ft":5000}`, or `World.ApproachDescend`. It is said against the transition altitude ("descend to flight level 080") and is only a descent: a level at or above the aircraft is refused.

### Changed

- A runway with no published procedure now reads "runway 02 has no published procedure", not `no procedure "" for runway 02`.

## [0.113.1] - 2026-10-23

### Fixed

- Data races found by `-race` in a host's offline tests, which make one World per subtest:
  - The World's console output was a package variable, replaced by each new World while an earlier one's goroutines still wrote to it. The swap is now atomic.
  - The slow-frame log line read an aircraft's state without its lock.
  - The schedule's replay hook was set when the HTTP API was first built, without the World's lock, while the connection read it under that lock.

  A test now runs Worlds one after another, each with its own output.

## [0.113.0] - 2026-10-23

### Added

- Cruising levels: `nav.LevelRule`, `LevelRuleFor`, and `CruiseLevelRule`, with the rule taken from the departure country. They follow Wikipedia's "Flight level":
  - below FL290: odd thousands eastbound, even westbound;
  - in RVSM, FL290–FL410: FL290, 310 eastbound and FL300, 320 westbound;
  - above FL410, 4,000 ft apart: FL410, 450, 490 eastbound and FL430, 470, 510 westbound;
  - without RVSM, from FL290: FL290, 330, 370 eastbound and FL310, 350, 390 westbound;
  - north/south in Italy, France, Portugal and Spain, with southbound odd; New Zealand southbound even;
  - VFR the IFR levels plus 500 ft below FL200.

  Before, levels above FL410 followed the RVSM parity (FL430 eastbound).
- Performance for 27 more types: airliners (A319, A21N, B739, BCS3, E195, A332, A333, A359, B763, B744, B788), turboprops (AT72) and the business types of the schedule (Phenom, Citation, Praetor, PC-24, PC-12, TBM, King Air). A King Air was planned as a generic jet at FL360.
- `World.Now()`, `World.PlayerPlace()` and `World.Vehicles()` read the clock, the player's landing place and the ground vehicles without building every aircraft's view, as `Snapshot` does under the lock the traffic's tick needs.
- `flight.Recorder.SamplesSince(objectID, from)`: only the samples recorded since `from`, for a save that appends, instead of copying the whole flight under the lock `Handle` needs.

### Changed

- `nav.Performance.MaxFL` is the highest level a type is normally planned at, not its certified ceiling. An A320 now goes to FL370, a 737-800 to FL380, and the long-haul twins to FL370–FL390. A 737 had been planned at FL400.
- Jetways:
  - **Unfit gates.** A gate whose helpers the sim puts on other gates more often than it docks them (3 times or more) gets no jetway, and is tried again once every 2 hours (`GateUnfitMin`, `GateRetest`). Every such miss takes a docked gate's jetway. Over three days at LKPR, stand 11 docked 8 times and missed 15, stand 19 docked 5 and missed 9, and every other gate missed 2 at most. The counts are kept in `jetway-gates.json` (`HelperOptions.OnGateResult`, `Helpers.GateRecord`).
  - **Refills back off.** A fill whose placeholders land on docked gates twice stops, and waits twice as long each time, from 2 minutes up to 30 (`FillStrays`, `StrayWaitMax`). Live, the sim gave each refill a docked gate again, one gate lost every 2 minutes. Removing the newcomer at once was also tried live, and the jetway did not come back to the gate's own helper.
- `airport.Cache.Put` of the same layout, or an equal one, keeps the built graph instead of throwing it away (a host refreshing its layouts every 5 minutes rebuilt them each time).

### Fixed

- `pkg/traffic/world` crashed under `-race` ("checkptr: pointer arithmetic result points to invalid allocation"): two enumeration readers kept a pointer as a `uintptr`. Both now use `unsafe.Add`, and a test reads a multi-entry enumeration.

## [0.112.0] - 2026-10-23

### Added

- Staffed delivery: a departure calls for its en-route clearance and the AI does not answer; the action `clearance` gives it, with `World.Phrase(id, "clearance")` for its wording. Delivery unstaffed, the World gives those still waiting (#1156).
- `ControlView.AltFt`: our aircraft's altitude as the simulator reports it, for a scope's label without `/api/traffic` (#1152).
- `POST /api/game` with `"intervalSec": 0` scores the traffic the host makes and spawns none; without it a flight comes about every 3 minutes, as before (#1152).
- `traffic.FitsStands`, `traffic.FlightsThatFit`.

### Fixed

- `/api/control` named ground as an aircraft's position while delivery still had it (from its clearance request to its clearance), and `?as=ground` was refused by the very position the view showed. The view keeps delivery until it hands over (#1155).
- Schedule: light aircraft are drawn only from the types the field has a stand for. At LKKB, stands of 5 m radius, only a PA-28 parks, and every other VFR flight was refused at its spawn, retried and cancelled. A club's aircraft the field cannot park stay at home, and a field with no stand for any flies no light traffic. Every flight, IFR included, whose type has no stand at its airport is left out of the schedule and logged once, instead of being refused at its spawn: the airliners into LKKB (#1152).

## [0.111.2] - 2026-10-22

### Fixed

- Jetways: a gate that refuses a helper type (the simulator puts that helper on another gate, taking its jetway from our aircraft there) is docked from then on with a type it takes: the last helper docked there, or else the placeholders' type, since one sits there. Live at LKPR over three days, stand 11 never took BCS3 in five tries or A20N in two, 19 never took BCS3 or A20N in three each, and 20 never took B738 in four but took A332 twelve times; each miss stole another gate's jetway. What each gate taught is saved in `jetway-gates.json` in the World's data folder and given back at the next start (`jetway.HelperOptions.OnLearn`, `Helpers.Learn`).
- Pushback tugs: vehicles call ground 250 m before a taxiway (was 80 m), about 30 s at road speed, so the answer comes before they arrive and they drive on without stopping. A tug's time to reach the nose allows 45 s more for each call on its way (`VehicleGateAllowance`). Live, OKJTE's tug waited up to 27 s at each of four taxiways and missed its push.

## [0.111.1] - 2026-10-22

### Changed

- Docs: the traffic pages match the engine. Holds are only for emergencies, weather and closures, with no 360 for spacing; a crossing restriction comes before a level-off; aircraft on the ground sit at their exact static pitch.
- Docs: new pages Traffic Overview (a reading path through the traffic docs), Emergencies & Scenarios, Jetways, Failures, Jetway and Tools (airport map via `go run -C cmd/airport-map .`, scene-cli, traffic-actuator and traffic-director). Traffic Decisions is split into Ground Decisions and Runway & Air Decisions. ATC Game is rewritten around staffed stations. Traffic World gains the runway in use, stand rules, ATIS and offline loading.
- Docs: registry counts (150 entries in six categories), complete request and event ID ranges (failures at 0x7F00 from v0.111.0), the missing dictionary tables, `WithStallAfter`/`WithStallPaused`.
- Website: /getting-started renders the markdown guide; the /docs index uses the sidebar's section labels and order; new Tools section.

## [0.111.0] - 2026-10-22

### Added

- Camera, composition first: shots are slow glides between frames picked from stills (`sceneCompositions`: every move at three distances and three moments, swept at LKPR and reviewed; the close details were all too tight). A glide moves at most 1.2 m/s, and a dolly slides camera and aim along the aircraft at about 0.5 m/s, from side frames only. No frame from behind the tail. At a stand the camera never goes ahead of the nose or to the jetway's side; elsewhere it is low only over the taxiways and aprons. A dolly with one end hidden starts there and comes out. Scene shots take `from`/`to` (`"to": "dolly"`); the scene filler and the auto camera play the glides in turn, none twice before all have played.
- `POST /api/camera/shot` plays one move, glide or held frame on an aircraft (`back` and `fov` to tune it, `at` to hold it still), then gives the camera back; `GET /api/camera/moves`, `GET /api/camera/glides`.
- `POST /api/camera/capture`: the simulator's own camera, read without taking it (`CameraGet`), placed in the world, from the aircraft filmed (offsets as a scene shot's, with the point aimed at) and on the airport (nearest stand and taxiway, along and beside the runway).
- `cmd/scene-cli`: spawn the aircraft to film and a car to frame against, capture compositions with a picture of the simulator's window, play shots, glides and scenes, switch the vehicle radio.
- Scenes: a cast member appears `after` a cue (`"after": "dep:pushback"`), so the arrivals keep in step with a departure however long its turnaround takes; the showcase does. A beat `listen`s to a frequency given (`"132.89"`, a station not ours). The `say custom` action takes a crew's line (`"from": "pilot"`), the answer as written (`"reply"`, `"none"`) and a frequency of its own; `emergency` with `"say": "none"` gives priority, squawk 7700 and the rescue without the stock calls.
- `airport.Graph.EndEntrances`: the entrances side by side at a runway's end, each with its holding point (LKPR 24: A and Z; EDDM 08L: A1, A2, A3); intersections further in are not. `airport.RouteOptions.HoldShortQueue`: a full-length departure takes the entrance with the shorter line, still "runway 24" on the radio.
- Vehicle radio off: `World.SetVehicleRadio(false)` (`POST /api/vehicles/radio`) keeps the ground vehicles quiet; a runway crossing still waits for the tower.

### Changed

- Departure queue: counted apart on the taxiways (up to 6, `DefaultMaxDepartureQueue`) and at each runway (in its line within 400 m, holding short, lining up or rolling: 3 per entrance at its end, at least 4, `DefaultMaxRunwayQueue`, `ManagerOptions.RunwayQueue`). Live at LKPR departures were held on their stands behind four already at the holding point, and one rolling at 160 kt still counted as taxiing.
- `airport.FullLengthMeters` 300 m (was 150): measured on 262 runway ends at 40 large airports, the nearest entry is within 186 m of the threshold at nine in ten, 265 m at 95 in 100 (LKPR 24 at 201 m, EDDL 23L at 269 m).
- Ground vehicles ask for a taxiway or a runway crossing when within 80 m of it (`VehicleAskAheadM`), not as they set off.
- `failures.DefaultEventBase` 0x7F00 (was 0x7B00, `systems.DefaultControlBase`'s: their client event IDs collided).

### Fixed

- The stand fit is no longer logged for a departure docked as it spawns, before it has a position ("5228941.69 m ahead of the parking point").
- CHANGELOG: the stale `[Unreleased]` block shipped in 0.18.12 and is filed there.

## [0.110.0] - 2026-10-22

### Added

- Scene `4-showcase` ("SDK showcase"), about 11 minutes at LKPR among the scheduled traffic:
  - DEMO007 in the WANTED Midnight livery boards at jetway gate B13, is refuelled and pushed back while a Lufthansa lands;
  - as it taxis, a Ryanair declares MAYDAY with an engine failure and DEMO007 is told to expect a delay;
  - DEMO007 holds short while the Ryanair lands with the fire trucks following it in, the runway held;
  - the runway is released and DEMO007 departs.

  Played offline end to end (worldtest), all 16 beats.
- Scenes do more than film:
  - `Scene.Jetways` and `Scene.Schedule` (`{density, maxAircraft}`, switched on if off) set the airport up;
  - `SceneCast.Stand` (any free one if taken), `SceneCast.Fuel`, and `SceneCast.Emergency` (a kind: declared once it flies, with its MAYDAY or PAN PAN, priority and rescue vehicles);
  - `SceneBeat.Actions` run World actions on the cast as a scenario's steps do (`World.Act`), and `SceneBeat.DelaySec` starts a beat after its cue;
  - `Scene.EndActions` run however the scene ends.
- Scene cues `ROLE:rescue` (rescue vehicles sent for it) and `ROLE:rescue:with` (one of them within 80 m of it).
- `World.HoldRunway(icao, runway, held)` and the scenario action `runway` with `kind: "hold"`: a runway held a while gets no clearances, its arrivals on short final go around, but it stays the runway in use (no runway change), as for the rescue vehicles after an emergency landing.
- `TaxiEvent.PushbackHeldBy`: the aircraft a push waits for. The log names it ("pushback holding for RYR1405 behind the stand").
- `worldtest`: the Loopback now behaves more like the simulator:
  - it has an add-on camera (`CameraSets`), so the World's camera and scenes run offline;
  - it keeps the lights client events set (`Object.Lights`, read back by `LIGHT …`);
  - it tells simulated objects (`Object.Vehicle`: tugs, trucks) from aircraft in by-type scans.

### Fixed

- Tower: a closed or held runway blocks line-up, take-off and landing clearances. The check gave the runway's name ("06/24") where an end is matched, so it matched nothing (a take-off cleared on a runway held for the rescue vehicles); a closure was only kept by the runway change.
- Offline airport dumps write the ILS of the airports an answer changed, not every airport known so far on each answer (O(n²) over a long dump; another session's airport dump).

## [0.109.5] - 2026-10-21

### Fixed

- Take-off (#1148): a model that refuses the main gears' positions is held gear down (`Injector.HoldGearDown`) by the handle and nose gear only. The first writes of each aircraft are watched for an exception; one switches that aircraft and its model. Live, the FSLTL Phenom (E55P) raised SimConnect exception 20 on every frame of its take-off roll until its gear came up (about 30 a second).

## [0.109.4] - 2026-10-21

### Fixed

- Tower (#1145): a departure is told to line up behind one of ours about to go only when that one is the only one of ours lined up or lining up. Live, LKPR: OKFXC lined up at B, CSA958 lining up from the threshold, and OKFZG at the same holding point was told to line up too; it was then cleared for take-off with CSA958 on the runway, CSA958's take-off cancelled, and OECHR later went around over CSA958.
- Jetways (#1146): a gate whose dock failed `JetwayGateFails` (3) times in a row, whatever the aircraft, rests `JetwayGateRest` (1 h): its aircraft get no jetway meanwhile. Live, LKPR B15's helper went to another gate on nearly every dock, taking a docked gate's jetway each time.

## [0.109.3] - 2026-10-21

### Fixed

- With no simulator (`SimlessAI`), an arrival on a STAR starts where it was created, at the STAR's first fix (#1143). SimlessAI never saw that pose: released, it started the aircraft at its first waypoint, the STAR's next fix (MyCrew app, offline: APRA2S appeared 6 NM from LKPR at 10,000 ft, at PR521 instead of APRAQ 27 NM out). `SimlessAI` notes each `AICreateNonATCAircraft`/`EX1` pose and takes it when the object ID comes (`SimlessAI.Handle`, which the World's run loop calls with every message). MSFS was never affected.

## [0.109.2] - 2026-10-21

### Fixed

- Jetways (#1141): a dock that times out keeps its helper, which the simulator put on the gate, as the gate's placeholder (the retract plan, released `RetractWait` later). It no longer removes the helper and refills the gate: the refill's placeholders went to docked gates, which the simulator counts free (live, LKPR 18 timed out, then 2, 15 and 17 lost their jetways). An undock whose kept helper's jetway is not seen home in time leaves the gate held, not refilled.
- `ScenarioCast.Procedure`'s doc says what it does for an arrival: it appears at its STAR's first fix (false: on final, cleared for the approach at once).

## [0.109.1] - 2026-10-21

### Fixed

- Jetways after an undock (#1139): the docked helper stays on its gate as the gate's placeholder (the retract plan sends its jetway home, its claim on the gate kept; released `RetractWait` later) instead of being removed and a new placeholder made. The new placeholder could be put on a docked gate, which the simulator counts free, taking its jetway from our aircraft there (live, LKPR stands 6, 8 and 18).

## [0.109.0] - 2026-10-21

### Added

- `World.PlaceUser(icao, stand)` and `POST /api/user/place` (`{icao, stand}`): put the user aircraft on a stand, a scenario's start when the player chose to be moved (MyCrew app, #1137). It stands where an aircraft parks there (`traffic.StandPoint`), facing the stand's heading, on the ground at the field elevation, stopped (INITPOSITION on the user object). It is refused, each with its own error (`ErrUserAirborne`, `ErrUserMoving` over 1 kt, `ErrUserEngines`, `ErrStandOccupied`, `ErrUserUnknown`), while the aircraft flies, moves or runs an engine, and when one of ours holds the stand or an aircraft is on it or on an overlapping stand in the way. The stand scan then finds the user there, so none of ours is sent to it. The API answers a refusal with 409.
- `Aircraft.EnginesRunning` (any `GENERAL ENG COMBUSTION:1–4`) and `Aircraft.ObjectID` on the user aircraft; `worldtest.Object.EnginesRunning`.

## [0.108.9] - 2026-10-21

### Fixed

- Jetways: one helper on its way at a time until it has docked, not only until it has an object: a helper made 24 ms before another gate's dock was seen was given that gate (live, LKPR 11 made as 4 docked took 4).

## [0.108.8] - 2026-10-21

### Fixed

- Jetways at bigger gates: the simulator gives a ParkedATC the best-fitting gate it counts free, a docked one among them, so an A320 helper for LKPR C24 (28 m) or B14 (35 m) went to a docked 22 m gate and took its jetway (live, A1, C30, C23). At a gate bigger than others of its airport, an aircraft whose helper would fit a smaller one docks with a helper too wide for the smaller gates and fitting this one (C24: B763, B14/C22: B788), placed door to door. Wingspans from the installed models (`ModelDoor.WingSpanFt`), else the profiles. The B14 A320 override is gone.
- Intersection taxi requests say the runway: "OKCVT, request taxi, runway 06, intersection E" (`RequestTaxiIntersectionFor`; the user: the intersection alone does not say which runway).
- No "give way to the … ahead" for an aircraft that stops behind traffic standing on its way (pushed, waiting to taxi): it stops anyway (the user, live: TVS1693 behind BAW106).

## [0.108.7] - 2026-10-21

### Fixed

- Jetways leaving docked aircraft: the simulator counts a gate docked moments ago as free and gives it the next helper, whose jetway then leaves our aircraft there; dropping that gate's helper left it free, and every later helper went to it (live, LKPR: C30 docked 30 s before C24's helper was put on it; A1 took the helpers of B14, C26 and B16 one after another). No helper is made while another gate docked within `jetway.DockSettle` (2 min), and a gate the simulator gives placeholders, left with nothing on it, is filled again before any other dock. Live: B16, C19, B11, C26 and A3 docked one after another, none taken. A gate given to the wrong helper logs the jetway list (stand, status, attached object) with ours.
- Gate departures' wait for the jetways is capped 6 min from when the jetways were switched on, not from the traffic's start (live, the cap had run out before they were on: TVS1779 on C22 spawned unwaited).

## [0.108.6] - 2026-10-21

### Fixed

- Jetways: a fill whose placeholders no jetway took (all `EmptyFillTries`) is tried again every 2 minutes (`jetway.RefillEvery`) instead of never, so the gates fill once the simulator gives them, with no toggle or restart (live, LKPR one evening: no gate taken for hours; the next morning all 19 at once).

## [0.108.5] - 2026-10-20

### Fixed

- A departure on a jetway gate waits for the jetways (`ErrSpawnBlocked`, tried again, no attempt lost) until its airport's gates are filled with placeholders, at most 6 min after the start: placed earlier it stood where no helper's door met it (live, QTR605 at B14 spawned a minute before the gates were filled, 18 m off its jetway). Ramp stands and arrivals are not held; with the jetways off or no helpers installed nothing waits.

## [0.108.4] - 2026-10-20

### Fixed

- Jetways at LKPR B14 for widebodies: the simulator parks no helper of a big type there (live, UAE892's 777 helper put on C22; earlier a 787's 308 m away). Stands listed in `gateHelperTypes` dock any aircraft with the helper of the type given (B14: A320), ours standing with its door where that helper's is.

## [0.108.3] - 2026-10-20

### Fixed

- Rescue trucks going home jumped 5 m in a frame: their way home started at their reference point, which the mover takes as the nose, so they went back a wheelbase at once (MyCrew app offline, worldtest). It now starts at the nose. `worldtest` `TestPlayerRescueSmooth`: drive out, follow, stand by and home with no frame over 3 m.

## [0.108.2] - 2026-10-20

### Fixed

- Jetways at stands with a stand rule along the line (LKPR B14/C22, narrowbodies 9 m back): with the jetways on, our aircraft stop where the simulator parks at a jetway gate; the jetway reached no further (live, a B738 on C22 9 m back, the jetway docked short of its door; the user: the jetway wins). The rule still applies with the jetways off. `jetway.Helpers.HasJetway`.
- A jetway gate no placeholder took (our aircraft already on it as they filled) docks when it is the only free jetway gate: the simulator puts the next ParkedATC there (live, LKPR B15: WZZ1023 never docked while the helpers of stands 11 and 17 were given B15 and failed).

## [0.108.1] - 2026-10-20

### Fixed

- Jetways after a simulator start: the placeholders taking the gates were tried 5 times (about a minute) while none was taken, and then never again; a map started while the simulator was still loading had no jetway all evening (live, "gates filled (0)"). Now tried for about 5 minutes (`jetway.EmptyFillTries`, 25).
- With no simulator (SimlessAI), the airports around the user aircraft from the offline list (`LoadAirportsFile`, within 100 NM) stand in for the simulator's facility list: the picture had no airport for any aircraft, so an emergency's rescue was never sent (MyCrew app, the player's emergency offline). `worldtest.Start` loads `<DataDir>/airports.json`; `TestPlayerRescue`.

## [0.108.0] - 2026-10-20

### Added

- One clock for the traffic and the World (`traffic.TimeSource`, `SetTimeSource`; `traffic.Now`, `Since`, `Until`, `After`, `NewTicker`, `AfterFunc`, `Sleep`): every read of the time and every timer of the controllers, the injector and the World's loop goes through it, the airport and nav loaders' timeouts too (`airport.SetClock`, `nav.SetClock`, set with it). The wall clock by default: nothing changes live. Logging, the camera, real-world feeds and the network stay on the wall clock.
- `traffic.ManualClock`: time that moves only when `Advance` is called, its timers and tickers fired in time order.
- `worldtest`: `Options.Manual` runs the World on a manual clock; `Advance(d)` and `Until(max, cond)` step it frame by frame, each frame taken by the World (`World.Queued`) before the next: about 100 times faster than the simulator (LKPR, 10 frames a second: 2½ simulated minutes in 1.6 s).

## [0.107.0] - 2026-10-20

### Added

- `pkg/traffic/world/worldtest`: a World run with no simulator, for tests (MyCrew app's scenarios end to end). `Loopback` is an `engine.Client` standing in for MSFS: AICreate* answered with object IDs, positions written kept, every data request (by SimVar name and unit, each frame or second) answered from them, the model lists given (`DefaultAircraftTitles`, `SetTitles`). `Start` runs a World on it with `SimlessAI` and an airport from offline data (`simconnect-data`), records the radio (`Heard`, `Said`), finds our aircraft (`Aircraft`) and scripts the user aircraft (`SetUser`). Wall-clock time for now: `Advance` waits.

## [0.106.0] - 2026-10-20

### Added

- Jetways meet our aircraft at stands where a stand rule puts it off the simulator's place along the line (LKPR B14/C22: narrowbodies 9 m back): the helper, parked by the simulator at its own place, is a variant with its door that far along the fuselage (`jetway.HelperAheadTitle`, `PackageOptions.AheadM`; live, RYR341 on C22 had the jetway at its nose, 9 m ahead of the door). The world asks for it when installed; `jetway-helpers` writes the stand rules' offsets by default (`-ahead rules`). Reinstall the helper package and restart MSFS to load the variants.

## [0.105.0] - 2026-10-20

### Added

- Ground radio: an aircraft giving way short of another taxiway is sometimes told "hold short of C" instead of "give way to …" (half the time, `holdShortShare`), read back "Holding short of C" (Doc 4444 12.3.4.8), and "continue taxi" once the traffic has passed. `TaxiEvent.GiveWayHoldShort`, `ArrivalEvent.GiveWayHoldShort`, `HoldShort`, `IntentHoldShort`.
- Our aircraft on the ground are placed with their ground speed (the INITPOSITION airspeed), as vehicles are, so their wheels can turn.

### Fixed

- Aircraft on the ground no longer float: placed a degree nose-down of their rest pitch (on extended struts, `MovingPitchDeg` 1°, #676) a frozen aircraft rests on its nose wheel alone, the main gear in the air (live, GEAR IS ON GROUND: B738 at +1° 9.72 ft nose only; A320 at +0.05° to +0.2° and -0.1° nose only; at the static pitch on all three). Standing, taxiing and rolling they are now placed at exactly their static pitch (`MovingPitchDeg` 0); the struts extend as the nose lifts at rotation.

## [0.104.3] - 2026-10-20

### Fixed

- Parked arrivals stand on all their wheels: frozen in their last taxi pose (a degree off the rest pitch, on extended struts) only the nose wheel touched and the main gear hung in the air (live, B738s at 9.72 ft). Arrived on the stand they are placed once at their static pitch and CG height (`Injector.PlaceRest`), where the simulator settles them on all three (8.40 ft, measured live).

## [0.104.2] - 2026-10-20

### Fixed

- Departures keep the level they were cleared to: their climb route went on only 60 km past the SID, after which MSFS AI climbed to a level of its own (live, TVS1828 cleared to FL220 and DSXHZ held at FL100 met at FL234, the conflict watch turning them apart 18 min out; others at FL356). The route now goes on 450 km along the last track (`ProcedureContinueMeters`), the 30 min a departure is ours.
- A departure's climb held clear of traffic is cleared on for as long as it flies: the re-check stopped once its controller finished after the hand-over, so it was never cleared higher (live, DSXHZ).

## [0.104.1] - 2026-10-20

### Fixed

- Airport map deadlock: a departure asking for taxi from its stand ("stand B1, request taxi") locked its own mutex twice and froze the whole world (live, 75 min with no traffic moving). `onStand` now runs under the lock its caller holds.

## [0.104.0] - 2026-10-20

### Fixed

- Rescue vehicles (live, CSA1461):
  - They followed 6 s before touchdown, the flare under 50 ft counting as down. Now they start from the wheels on the ground (under 3 ft).
  - Taxiing was taken for stopped, because MSFS reports 0 kt for AI on the ground. The aircraft's speed now comes from its positions.
  - The trucks show on the airport map (R markers with their way, `ControlView.Rescue`).
- Tower: a departure with a conditional line-up behind one departing is no longer told "hold position" right after it. When the line-up was released, the room before the next arrival was counted as if the one ahead were landing, waiting out its landing roll. It now uses the room the line-up was given with: its own line-up, take-off and the margin (`RunwayController.LineUpRoom`; live, KLM1616).
- Tower: a departure waiting out its departure interval is told to line up and wait only within 45 s of its take-off (`LineUpWaitMax`). Before that it holds short instead of blocking the runway lined up (live, EZY562 lined up 3 minutes behind a TBM 900 on the same SID). Conditional line-ups behind one departing are unchanged.
- Tower: with one of ours already lined up behind the one rolling, another waiting at an intersection further down is no longer lined up too (live, OKDIS lined up ahead of CSA443 at the threshold and jumped it).
- Ground: "follow the …" only for traffic within 300 m ahead on the route, in sight, not 800 m (live, TVS1373 told to follow a far-away Ryanair).
- Pushback: once approved, the schedule's queue hold no longer holds a departure on its stand again (live, WZZ1251 approved, held 3 s later and waiting 10 minutes with its tug at the nose). Waiting for a neighbour's push is unchanged.

### Added

- A crew still on its stand asks "stand B1, request taxi", as in the ICAO Phraseology Reference Guide (`RequestTaxiFrom`).

## [0.103.0] - 2026-10-20

### Added

- `POST /api/emergency` (`{tail, kind, level, on}`): one of our arrivals declares an emergency of a kind, as a scenario's "emergency" step does; `on: false` cancels it.
- Airport map: Emergency (engine failure, MAYDAY) and Medical (PAN PAN) buttons in an arrival's panel, to try the rescue vehicles and the sequence around them. Pressing Emergency again cancels it.

## [0.102.0] - 2026-10-20

### Added

- Rescue vehicles (the emergency add-on, part 1): an aircraft with priority coming in to an airport of ours gets its fire and rescue.
  - As it comes within 20 NM, two fire trucks drive from the nearest vehicle depot along the service roads to standby points beside its landing runway, 60 m off the edge on the side of its exits, and wait there. GSX's fire trucks are used first, else MSFS's own.
  - Once the aircraft is down they follow it, 80 m behind (40 m more for the second), up to 50 kt and turning no faster than 30°/s. A minute after it has stopped they drive back and are removed. If priority is taken away before the landing, they go back at once.
  - A medical gets MSFS's ambulance instead, following the aircraft in to its stand and staying 5 minutes.
  - Library: `traffic.SimObjectRescue` (`Standby`, `Follow`, `Home`, `Update`, `Handle`, `Remove`, `Done`) and `traffic.RescueStandby(layout, runwayEnd, n)`.
  - World: `World.PriorityFor(tail, kind)` gives priority for an emergency of a kind (`""` is the player). A cast aircraft's declared emergency sets its kind itself.

## [0.101.0] - 2026-10-20

### Added

- Staffed stations, step 1 (#1096): `World.SetStaffed` / `Staffed`, `GET` and `PUT /api/stations/staffed` (`[]StaffedStation{ICAO, Position | Station, Mode, AutoHandoff}`). A whole position or single stations, several making a band-box.
  - An aircraft on a staffed station is the player's (manual, as its own toggle makes it): the AI gives it no clearances, sequence speeds or vectors, or conflict resolutions. Its crew still calls on its own, and it goes back to the AI from its current state when it leaves the station or the station is unstaffed.
  - An aircraft's own manual toggle wins. Modes, flying without a further clearance, and handovers come in the next steps.

- Rotations: `Flight.Rotation` names the airframe flying a flight (a registration, an app's aircraft id). The legs of one rotation are one aircraft: it lands, turns around at its stand and departs as the rotation's next leg from that airport, whatever the airline, type or turn window. The app gives them through `World.AddFlights` / `POST /api/flights`; the map's schedule pairs as before.
- A take-off whose climb would meet an arrival on its way in gets a restriction first: "after departure climb to altitude 4000 feet" (ICAO Phraseology Reference Guide, ALL CLEAR), the lowest of the field plus 3000/4000/5000 ft that keeps it clear. It is held on the ground only when none does. `TaxiController.LimitClimb` caps its climb until departure clears it higher; `traffic.AfterDepartureClimb`.
- `pkg/failures` (the emergency add-on, part 3): fails and restores the user aircraft's systems by generic names (`engine-failure` per engine, `electrical`, `hydraulic`, `vacuum`, `pitot`, `static`, `brakes`, `brake-left`, `brake-right`) through the simulator's failure key events (MSFS 2024 Aircraft Miscellaneous Events). `Default()` is the simulator's own; a model's override (`Profile.With`, `LoadProfile`) maps or takes away kinds. `Supported()` says what an aircraft can fail. An engine fire, pressurisation and a fuel leak have no failure event, so they stay unsupported unless an override maps them. `Apply`, `Clear`, `ClearAll`, `Active`, `Reset` + `Reapply` after a reconnect.
- Emergency details:
  - The crew's answer follows the ICAO Phraseology Reference Guide's example: `traffic.EmergencyDetails` "CSA123, 156 persons on board, endurance 2 hours 10 minutes, request immediate landing runway 24", unknown parts left out. Intentions: `IntentionsLand`, `IntentionsReturn`, `IntentionsDivert`.
  - ATC's question: `traffic.EmergencyQuestions` "report persons on board, fuel endurance and intentions". The guide lists the items, not the question, so this wording is the project's.
  - A cast aircraft's emergency asks and answers with its own plausible values.
- Emergency cancel:
  - `traffic.CancelEmergency` ("Praha Approach, CSA123, cancel MAYDAY") and `AcknowledgeCancelEmergency` ("CSA123, roger, MAYDAY cancelled, squawk 4521"); the project's wording, since the ICAO guide quotes none.
  - Scenario `emergency` with On false cancels a cast aircraft's emergency (its code back, priority off).

### Changed

- The conflict watch resolves a climbing departure of ours with "cross (fix) at or below (level)" on its SID first (Doc 4444 12.3.2.4 a). It climbs on past the fix with no second call; a level-off only when no fix fits (the user; live, OKETU levelled at 4000 ft for OOILE).
- Runway crossings keep the flow:
  - An aircraft taxiing up to a runway it crosses is in that runway's queue from 250 m before its holding point, cleared across before it stops when nothing is in the way.
  - Two crossings with nobody landing or taking off go together.
  - The runway's own traffic comes first: a departure that can go now goes before a crossing (the user).
- Intersection departures:
  - One holding at an intersection lines up and waits behind a departure from further down once that one is cleared for take-off, or behind one rolling once it is 400 to 800 m past its way in (by call sign, a controller's judgement; the user).
  - A conditional "behind the landing X, line up and wait" at an intersection goes once X has rolled past it, not once X is off the runway (live, TVS480 lined up 79 s after touchdown).
- Tower:
  - A departure told to hold position for want of time before an arrival is not cleared again until that arrival is down and rolled out (live, DCDHS "hold position", then "cleared for take-off" 3 s later).
  - A take-off cleared before an arrival to the other end came close in is cancelled before its roll (live, RYR1626).
- A stand hold released stands 2 min, time to push (live, TVS622 held again 3–20 s after each release).
- Speeds and vectors for spacing go before routine calls on the frequency (`prioSpacing`; live, a speed a minute late).
- Conflicts predicted only at the look-ahead's very edge are left for the next look (live, FTMKL/DLH620, OKUTI/BAW1182, QTR413/OKFLN "in 5m0s").
- Goodbyes follow the time of day: "good evening" in the evening, not "good day" next to a "good evening".
- "Arrival delayed" is logged once a minute for the same reason, not every second.
- `GET /api/ground-titles`: every ground vehicle title the simulator offers.
- An arrival slowed for spacing gets "resume normal speed" once the sequence no longer needs it (the user: the least speed only with a reason).
- The emergency acknowledgement says "roger the MAYDAY" (ICAO Phraseology Reference Guide). A fuel emergency says "fuel shortage": "fuel emergency" and "fuel priority" are not recognised terms.

### Fixed

- An arrival being created (no position yet) stays in the sequence at its route's first point, not out of it for a look (live, BAW451).

## [0.100.0] - 2026-10-19

### Added

- Emergencies (the emergency add-on, part 2):
  - `World.Priority(tail, on)` gives an aircraft priority. It lands number one at its own time, the other arrivals are fitted around it (`ApproachSequencer.SetPriority` / `SetPriorities`), and departures wait while it is within 15 NM. `tail` "" is the user aircraft, sequenced at the nearest airport when the host has not cleared it to land.
  - A cast aircraft's scenario action `emergency` (Kind engine-failure, medical, fuel, pressurisation, fire, hydraulic; Level chosen by kind unless given) squawks 7700, declares MAYDAY or PAN PAN and is acknowledged.
  - Phrases: `traffic.DeclareEmergency`, `AcknowledgeEmergency`, `EmergencyLevel`.
- Scenario say `custom`: a scenario's own words to a cast aircraft, on its unit's frequency (Args["station"]: delivery, ground, tower, departure, approach, center; "" the unit it is with), said as written and acknowledged "Roger, CSA4KM". The cast's {role} names are filled with call signs, then `ScenarioHooks.Fill` fills the host's values. `traffic.CustomCall`, `traffic.Roger`.
- `traffic.ExpectDelay(pos, cs, minutes)`: "CSA123, expect 10 minutes delay" ("expect delay" with 0), read back. The scenario say action takes `expectDelay` (Args["minutes"]).
- Offline weather: `World.SetWeather(*nav.Weather)` sets the weather the World uses (wind, QNH, the ATIS, the runway in use) over the simulator's; nil gives it back. `LoadAirportDir` reads an airport's `weather.json` (`WeatherFile`) as it.
- `nav.ATISWithLimits`: the ATIS reads its runway limits at every update. `nav.RunwayLimits.ChangeNow`: a supervisor's runway choice is changed to at the next Ready moment, not after the wind's ten minutes.

### Fixed

- The runway in use no longer flip-flops after a runway is made the one in use or closed. The ATIS chose by the limits it was made with and turned the traffic's runway back every few minutes (live, LKPR: 06 and 24 by turns six times in 26 minutes).
- A runway change's changeover deadline is kept until the changeover is over. A look that counted no departure for a moment set a new, later one, and the first arrival's delay crept up (live, OKBWJ number 1, 16 s to 4m15).
- The climb check flies a departure on its SID before its hand-over to MSFS AI too, not on no route (live, DLH620 cleared to FL220 through CSA253 at 10000 ft, 6 s before its hand-over).
- Merge planning leaves pairs the conflict watch is resolving, and meetings closer than 12 NM along the way. It no longer gives a vector the same second as the conflict watch (live, CSA253 "fly heading 355", then "resume own navigation" a second later).
- Sequencing:
  - Predictions are smoothed over 30 s; a jump of 2 min or more is taken at once (live, BAW1182's delay 45s, 80s, 47s, 16s within a minute).
  - A departure gap stays with the arrival it was given to (live, OKBWJ's gap 10.5 and 6 NM by turns).
  - An arrival handed from en route to its controller stays in the sequence.
- A spacing vector is flown: a dog-leg whose apex is the first corner of the re-planned route is judged from where the route was given. Before, it counted as passed at once, the aircraft being nearer the fix after it, and "resume own navigation direct PR742" followed a second after the vector (live, OKUTI and CSA253: the planned spacing never flown, caught by the conflict watch).
- The approach clearance with its intercept heading is given only while the arrival flies toward its turn onto the final. Before, a corner counted passed too early brought "turn left heading 060 to intercept" while the aircraft still flew its route back to PR742, heading 146, and the arrivals behind it were cascaded (live, LKPR: CSA253).
- Merge planning no longer steers arrivals around departures climbing out (added in 0.99.0): its dog-leg to pass behind one turned the arrival into the climb (live, LKPR: OOILE "fly heading 285" into OKETU cleared to FL240). The take-off and climb checks time the departure instead.
- An aircraft on a conflict resolution's heading or level is predicted as it flies, not along its route, by the climb, en route and merge checks (live, DLH620 on "heading 080, due traffic" predicted on its SID, FTMKL cleared to FL240 into it).
- The stuck timers count from progress: a controller's state change, an arrival closer to the field than ever, a hold or its release. A flight held by ATC is never removed, and one in the runway queue has twice the time (live, LKPR: FZEEP removed holding short with a line-up clearance, CSA253 on short final cleared to land, TVS622 held on its stand). `TrafficManager.Progress`.
- At a runway change only departures holding short or later, or taxiing within 500 m of their runway, finish on the old one; the others go to the new runway at once (live, FZEEP held at 24 with no slot, sent round ten minutes later; RYR1626 off 24 into the 06 arrivals nine minutes after the change).
- The same ATC call to one aircraft within 2 s is said once (live, EZY1029 "contact Ruzyne Tower" and TVS252 "when vacated contact" twice in a millisecond).
- The intercept heading said is flown: from where the aircraft is, on that heading to the final's centreline, then the final (`ArrivalController.InterceptOnHeading`); a heading it cannot fly there is not said (the user: told 060, it follows 060).
- Speed before a vector in trail: an arrival spacing behind another, or within 30 NM of the end of its STAR, may be slowed to the approach speed (180 kt) the arrivals ahead fly from the IAF, not only to 210 kt and then a dog-leg (the user, live: CSA495 at 210 kt behind TVS252 at 180 got a dog-leg).
- After a runway change, the new STAR's first point is flown to only while it is still the first point: a dog-leg put before it, or a direct past it, lets the aircraft pass it (live, THY869's route drawn back to it for good).

## [0.99.0] - 2026-10-19

### Added

- Scenarios: `World.StartScenario(sc, hooks)` plays a scripted situation around the traffic (the app's Situation Creator). A cast of our aircraft, each pinned to a role, flies on top of the schedule. Steps each wait side by side for their cue, then a delay in the World's clock (a pause stops it), then run their actions once. Cues: `start`, `t:SEC`, `after:ID`, `ROLE:STATE`, `ROLE:airborne`, `ROLE:heard`, `ROLE:lights:X`, `ROLE:height:FT`, `ROLE:distance:NM` / `distance:>NM`. Actions: spawn, push, goAround, hold on/off, runway close/open/use, say (cancelTakeoff, holdPosition, goAround), camera (the scene moves). Anything else goes to the host's `ScenarioHooks` (Cue, Action, Event), called outside the World's locks. `ScenarioRun.Stop/Status`. For a host that runs the steps itself: `CastSpawn`, `CastCue`, `Act`.
- `World.UseRunway(icao, end)` and `POST /api/runway-use`: make a runway end the runway in use whatever the wind ("" gives it back to the wind). The change goes as any runway change.
- Arrivals planned apart where their ways in meet. Each arrival's way (STAR, approach, any dog-leg or longer downwind) is flown 12 min ahead against those landing before it. Where they would meet, the later one loses the time before the meeting point by speed and one vector. A shared named fix is not needed (live, LKPR: TVS1113 on a downwind extended for a runway change into TVS122 on LOMK 8S, caught by the conflict watch at 4m40s).
- En route traffic planned clear before it appears: its way across at its cruise level is flown 20 min ahead against everyone's planned ways. Where it would meet one, it gets another level of its direction (2000 ft lower, then higher), or waits (live, DLH1050 and THY1880 created in the same second at FL400 on converging tracks).

### Changed

- Runway change as a planned changeover: the old direction's departures get a deadline set once at the change (2½ min each), no longer slid on every look. One still at its holding point after it is re-cleared to the new runway. The first landing on the new direction comes 5 min after the last has gone, far enough out to be no traffic for its climb.
- Touchdowns spread ±60 m (before ±10 m: live, every one 480–494 m past the threshold). The exit is the first one reachable slowing down as planned, from where an injected landing really touches down, not one further down nearer the stand. The rollout is one even deceleration from the touchdown to the exit speed at the exit, at least 1 m/s², at most braking hard. Before, it braked hard to 80 kt and rolled on to the exit.
- Headings are said in 5° steps: "fly heading 060", never "061".
- A landing aircraft on a runway with an exit on the other side near its own is told which way and where: "vacate left via F, when vacated contact Ruzyne Ground 121.91", read back (`WhenVacatedVia`, `Graph.ExitsBothSides`).
- Departure gaps in the arrivals: the departures waiting at a runway count at their most for 2 min. A gap no longer comes and goes as departures near the runway come and go (live, TVS555 behind ENT891 at 6.4 and 10.5 NM by turns every few seconds, its delay 1 s and 1m46s).
- A runway change's new STAR is joined no farther out than the arrival is: never sent back up it against the arrivals coming in (live, TVS555, number 2 east of the field, sent back to LOMKI head-on with OKJAF on LOMK 8S). One that joins part-way (out of position) is sequenced behind the arrivals already on the new runway's flow within 40 NM, each behind the one before: vectored in behind, not turned across the stream.
- Departure radar clears a climb only clear of everyone on their way: the other departures and the en route traffic too, not only the arrivals (a week of logs: 67 departure pairs cleared up into each other, CSA208 and CSA657 at FL234).
- The merge planning flies an arrival's way against the departures climbing out as well: it passes behind them (a week of logs: 94 arrival–departure conflicts after a vector or an arrival's delay).

### Fixed

- `SimlessAI`: a departure handed back to the AI after its injected climb (unfrozen) is flown. Before, every departure stood where its injected climb left it.
- `DepartureWaypoints`: a SID that turns back keeps its fixes. A fix behind is left out only when it lies on the track just flown, and a direct-to-fix leg never is. Before, LKPR 24 ARTU 6M, EGLL 09L CPT 4K and others lost every fix and flew the runway heading for 60 km. Also hits live MSFS and the take-off check.
- `PlanArrivalProcedure`: a descent never climbs. Each point is no higher than the one before, so an at-or-below holds on from its fix (EDDF 07C via EMPAX: SPESA planned at 28031 ft after ADNIS at or below 10000).
- The tower's "when vacated contact ground" is said once (live, OKECF and TVS122 told twice in the same millisecond).

## [0.98.1] - 2026-10-19

### Fixed

- Take-off clearance: a departure is cleared only with its way out clear of the arrivals' ways in. It is predicted from lift-off along its SID, climbing at the rate it can make, against every arrival flying its STAR and approach as planned. Before, only the arrival's time to the threshold was counted, so a SID across the arrival flow, or a long procedure, met them in the air (live, LKPR: ENT1744 off 06 on ARTU 5E into TVS1113 on the 24 approach, then held at 3700 ft under the arrivals). It waits with "<arrival> on its way in". `TaxiController.DepartureRoute()`.
- Vacating: an arrival stops just past the holding point behind its exit, not 280 m on down the taxiway at the next junction. It is cleared to taxi from there, without the exit off the runway: "via L, D" (live, OKZYC off 06 at L for N73 heard "via D, L, D"). A short first taxiway, where the aircraft stands, is not said (`Route.SpokenTaxiways`). `ArrivalPlan.TaxiRoute()`.

## [0.98.0] - 2026-10-19

### Added

- `traffic.SimlessAI` and `world.Options.SimlessAI`: with no simulator behind the client (Vector, the ATC game, on a loopback client), what MSFS AI would fly is flown in the core. An aircraft released to the AI with a waypoint chain follows it: at each point's speed (1.5 kt/s changes), reaching each point's altitude by the point (at most 3000 fpm), turning at a standard rate with the turn begun early, its position written each step for the client to report back. Arrivals fly their published STAR and approach transition to the join point, where the injected approach takes them to touchdown as after MSFS AI; an aircraft frozen or placed by its controller is no longer flown. Off unless asked for: with MSFS nothing changes.
- The worldwide airport list offline: `Options.AirportDumpDir` writes it once as `airports.json` (every airport the simulator knows, in ICAO order) and `World.LoadAirportsFile(path)` serves it without a simulator, for diversions and en-route origins. `airport-dump -airports` writes it.

## [0.97.0] - 2026-10-19

### Added

- Offline airport data: `Options.AirportDumpDir` writes everything the simulator gives the World about each airport it fetches, one directory per airport: `<ICAO>/layout.json` (the raw facility records, as `LoadFile` reads), `procedures.json` (`airport.Procedures`, as `LoadProceduresFile` reads), `airways.json` (`nav.AirwayGraph`) and `ils.json` (the ILS by runway end). `World.LoadAirportDir(dir)` serves an airport from such a directory without a simulator (each file optional): Vector, the ATC game, runs the World in WebAssembly on a loopback client.
- `examples/airport-dump`: writes that data for the airports named (`airport-dump -out dir LKPR EGLL …`), connected to the simulator; airways and ILS where the airport has them. The dumps live in their own repository (mrlm-net/simconnect-data).

## [0.96.0] - 2026-10-19

### Added

- `World.LoadProceduresFile(path)`: an airport's SIDs, STARs and approaches from a procedures dump (`airport.Procedures` as JSON, written by `spike-procedures -loader -dump`), `LoadFile`'s counterpart, so the World runs without a simulator (Vector, the ATC game, on a loopback client in WebAssembly). Procedures loaded so are not asked of the simulator.

## [0.95.8] - 2026-10-19

### Fixed

- Holding: never for sequencing or a conflict, only on a trigger (an emergency, the weather, a closure); a delay is absorbed by speed and vectors, a second dog-leg only after the first was flown and 2 min or more are still to lose (`SecondDogLegFrom`).
- Release planning: an arrival enters its STAR only when, flown along it at its levels and speed, it keeps separated from our arrivals already on their way (20 min ahead); else it waits for a later release (live, GSEXC and OKFLS merging at FL100, vectored apart).
- Departures: a climb above the arrivals is cleared only clear of them, flown at the rate the aircraft climbs (at least 1500 fpm), not at the cleared level by its next point, with a mile to spare; else "cross (fix) at or below (level)" (Doc 4444 12.3.2.4 a), else the climb is held and cleared once clear. An arrival level now counts as staying level. A climb resumed after a conflict is checked against every arrival. The conflict watch predicts departures the same way (live, KLM216 cleared to FL240 into SWR811 at FL100; AUA939 into TVS706 stopped at FL095; TVS608 cleared on into TVS706). Departures on the same route within 8 min get levels in trail, 2000 ft apart.
- Runway in use: the wind of the last 30 min, its direction too, must keep a runway within its limits before it is changed back to; a preferred runway (LKPR 24, then 06) is kept while within its limits; a change is forced only 2 kt or more beyond them, else made at a gap within 5 min. Near calm the preferred runway is gone back to after a closure (live, LKPR stayed on 12 after 06 reopened: each puff under 3 kt started the wait again, and 06 and 24 took turns as the choice).
- Runway change: departures taxiing, holding short, lined up or rolling finish on the old runway (not one closed); arrivals established within 15 NM land there; the others join the new runway's STAR at its entry ahead, else at its IAF, flown from that first point (live, AUA133 cleared for the ILS at once and taken for landing in a minute, its whole STAR to fly). The new runway's first arrival is planned after the old runway's departures with a 3 min margin. No approach clearance for a runway closed or no longer in use (live, AUA133 "cleared ILS approach runway 06" 5 s after 06 closed).
- Tower: opposite ends of one runway are never used against each other (an arrival within 12 NM or rolling, a departure lined up, rolling or airborne); no take-off under a go-around climbing out. Crossing runways: no take-off or landing while one of ours uses a crossing runway and has not passed the crossing, an arrival within 3 NM holding take-offs (live, TVS833 rolled on 30 and CSA522 was cleared for 24 at LKPR).
- Vectors: on vectors the approach clearance with the heading to intercept comes 3 NM before the turn onto the final course, however far out (live, OKKSC turned onto a 26 NM final unsaid); a spacing heading already said and flown is not said again.
- Ground: an aircraft does not hold for an oncoming one stopped behind it, in its way (live, CSA839 and TVS1120 on J, 19 min until TVS1120 was removed); pushbacks wait while 4 departures are already taxiing out ("hold on the stand, expect a few minutes delay"); a stale taxi re-plan of an aircraft moved since is dropped.
- Arrivals below 115 kt on touchdown are planned to an earlier exit at their own speed (a PC-24 at LKPR: 06 L, 24 C).
- TCAS, separation and conflicts: an aircraft on the ground or under 50 ft is not traffic (live, UAE530 TA with an aircraft at the gate); two aircraft both level 800 ft or more apart count as separated.
- En route: levels from FL100 up are whole flight levels (live, TVS1260 at 39300 ft, 700 ft under FL400 traffic); the conflict watch does not say the same instruction twice.

## [0.95.7] - 2026-10-18

### Fixed

- Go-around: a route re-planned on the way up (a delay absorbed) keeps the go-around's level instead of capping it where the aircraft is (live, TVS354 levelled at 1400 ft at LKPR and stayed under the stream; `climbCap`).
- Go-around: a new approach after it gets its own dog-leg again; the one-dog-leg-an-approach count of v0.95.5 was kept over the go-around (live, TVS354, vectored for a conflict before, only ever got longer downwinds and turned base ahead of the arrival it followed).
- Go-around: approach says nothing to a go-around handed over until it has answered "radar contact" (live, "number 5, extend downwind" before tower's "contact Ruzyne Radar"); a tower go-around still waiting for the frequency is dropped once the aircraft has landed (live, WZZ181 told to go around 52 s after touchdown).
- Go-around level: `airport.Limits.GoAroundFt`, the airport's own where the facility data's missed approach climbs lower; LKPR 5000 ft (the data: 4000 ft). Said by the tower and flown.
- Phraseology: an IFR downwind extended is flown on radar vectors, "fly heading 064, for spacing" leaving the STAR, then "turn right heading 151, for base" and the intercept with the approach clearance (Doc 4444 12.4.1.3); the sequence call says no "extend downwind, expect vectors", the aerodrome circuit's instruction (12.3.4). `ParamExtendDownwind` is gone.
- Runway change: an arrival past its STAR's entry fix joins the new runway's STAR at its first point ahead, not flown back to the entry (live, TVS356 turned back to VLM after the change to 06, against THY940 coming in on the same STAR).
- Overflights: none appears near another of ours asked for in the last 30 s, before the simulator reports it (live, THY436 and BAW1868 created in the same instant at FL400, 1 NM apart, TCAS RA).
- Conflicts: a change that is not working is looked at again: once flown a minute, a conflict of it still predicted within 2 min (or separation lost) gets another, instead of waiting out the 5 min look-ahead (live, EZY1384 turned for OKIXH, 3.4 NM and 790 ft from it, nothing more done for 1m47s).

## [0.95.6] - 2026-10-18

### Fixed

- Pushback: a push no longer ends with the fuselage on a service road past the end of a taxilane, where the planner takes the apron to run on (`laneEnds`). LKPR C17 for 24 swung the tail past the end of JB onto the road round the stands; it now pushes onto J facing north-west. Only roads up to 12 m wide count (`pushRoadMaxWidthMeters`; wider vehicle paths are aprons, LOWW C42), and a road crossing a taxiway does not. Of 245 stands at the 10 test airports only LROP 115 changes besides (its tail stood 5 m onto a road; now onto J); LKPR B16 keeps its pose, 5 m shorter.
- Pushback: saved standard pushes are planned again (`standardPlanVersion` 2).

## [0.95.5] - 2026-10-18

### Fixed

- Smoothness: a departure handed to MSFS AI gets its climb waypoints before it is released, so it no longer jumps at the handover.
- Smoothness: our aircraft step by the sim's own clock (`SIMULATION TIME` from each monitor frame), not the wall clock: no judder on the take-off roll or the final approach.
- Engine: the dispatcher polls every 1 ms while messages flow (250 ms window) instead of backing off up to 50 ms, so per-frame data no longer arrives two frames at once.
- Smoothness: a taxiing aircraft giving way is never pulled back along its path (3–23 m snap-backs with 60–94° swings).
- Smoothness: an aircraft turning at 0 kt is placed every frame, not every 30th.
- Smoothness: vehicles no longer jump. The follow-me car, bus and stairs leave from their front axle (3–6 m steps back before), stairs back out with a quarter turn instead of flipping 180°, and a vehicle eases into its spot (8 m) and onto a path (10 m) instead of snapping in one frame.
- Smoothness: under a standing aircraft a new ground height from the scenery is eased in at 1 ft/s, not taken in one frame.
- Radio: a vector queued behind a busy frequency is dropped when a newer instruction was said meanwhile (live, AFR539 heard a stale vector after its return to the path).
- Radio: one exchange at a time per aircraft: the tower check-in comes after the ground readback (live, EZY693).
- Arrival spacing: one dog-leg per approach, and none with an out-leg shorter than 30 s (`MinDogLegOut`; live, OKBZM turned through 100° several times).
- Radio: the number of departures ahead is said only with 3 or more ahead (`QueueSaidFrom`), and then about half the time.
- Diagnostics: a jump trace and per-minute frame stats on the console; glitch-watch records sim time, altitude and ground height.

## [0.95.4] - 2026-10-17

### Fixed

- Smoothness: no more height pops at rotation, touchdown and lift-off. MSFS draws compressed struts only at exactly an aircraft's rest pitch and extended ones (0.5–0.8 ft higher, in one frame) at any other; probed live (examples/spike-strut). Our aircraft now always sit 1° (`MovingPitchDeg`) off their rest pitch, struts always extended, so the look never switches.
- Smoothness: the ground under an aircraft is taken from a map of the samples the sim gave (about 1 m cells), at the position being placed, not the last frame's: the bump at 76 kt on LKPR 24 is gone.
- Smoothness: a departure taken over on its stand is placed once at its stop, on the stand's heading, before it moves: no 3–11 m jump at the first push frame.
- Smoothness: every moving aircraft is moved every frame, wherever it is (level of detail by distance stepped a climb-out watched from the tower); a taxiing aircraft's route search around occupied places runs off the connection's goroutine (80–90 ms frames before).
- Arrival spacing: two arrivals in trail on converging STARs are spaced as a radar controller would: the later one slowed and, if needed, given one simple vector before the merge (`ArrivalController.AbsorbWithin`), no level change, no orbit, no downwind extension; and no shortcut for one whose route joins that of an aircraft ahead of it in the sequence (live, OKCMB sent direct PR517 behind CSA1305: 3 NM at the same level).
- Stands: a per-stand stop correction, the way a GSX airport profile does it (`traffic.standRules`, `StandRuleFor`; the map's `/api/stands/rules` and a local `stand-rules.json` over the shipped ones). Shipped: LKPR B14 and C22 (35 m circles), narrowbodies stop 9 m further back (live, an A320 stood 9 m past its stop marks).
- Airport map: the map no longer freezes after hours of traffic. Go's mutex/block profiling deadlocked the runtime (found in a full minidump); it is now only on with `-pprof-locks`.
- Airport map: after the sim drops the connection the schedule is posted again to the new one; traffic stayed off for an hour before.

## [0.95.3] - 2026-10-13

### Fixed

- Traffic stutter when a departure's ground vehicles set off. Each new vehicle path was checked against every taxiway segment every 2 m, and the vehicle's first radio call was made inside the aircraft's frame: 110–140 ms frames, live at LKPR, whenever a tug or fuel truck got a way or asked ATC.
  - `Layout.TaxiwayAt` now uses a grid index per layout: 19 ms → 0.05 ms for a 435 m path.
  - A vehicle's request is queued, and its radio call goes out from the World's pending queue.
- The first arrival after an airport loads no longer computes stand taxi-ins on the connection's goroutine (206 ms). While `WarmTaxiIn` runs, the ranking uses the taxi-ins already known.

### Added

- SimConnect calls over 30 ms (`SlowCall`) are reported on stderr, naming the call.
- Benchmarks `BenchmarkVehicleRoute` and `BenchmarkVehicleGates` (docs/traffic-smoothness.md).

## [0.95.2] - 2026-10-13

### Fixed

- Traffic: no more vertical pops of 0.4 to 0.5 ft in a single frame at lift-off, as a landing rolls out, or holding on a taxiway. A change in the height an aircraft rests at blends in at `CGBlendFtPerSecond` (1 ft/s). Such a change comes from the rest it learns once settled, or from switching between the runway's and the apron's rest heights.
- glitch-watch: AI on the ground reports a ground velocity of 0 while it rolls, so every frame of a landing roll read as a position jump. The speed is now taken from the last frames when that is more.

## [0.95.1] - 2026-10-13

### Fixed

- The whole map froze (no traffic moved, the map lost its aircraft; live, LKPR at 19:55 and 01:05). A departure's frame asked whether its tug waits for the jetway (`TaxiRequest.TugWait`) and took the aircraft's lock. The aircraft's update held that lock and called into the controller, so the two waited for each other for good. The tug's wait is now drawn at the spawn and read without a lock.

## [0.95.0] - 2026-10-13

### Added

- Traffic smoothness (docs/traffic-smoothness.md): benchmarks of the work behind each simulator frame (`pkg/traffic/smoothness_bench_test.go`, world `BenchmarkAssignStandArrival`), `scripts/bench-smoothness.sh` to run them, and `examples/smoothness-report` to draw the charts from them and the recorded history. The World logs any step of its connection loop over 40 ms, naming the step. The airport map's `-pprof` also turns on mutex and block profiling.
- Landing: reverse thrust from main-gear touchdown, the throttle in the reverse range (`ReverseThrottlePct`), back to idle below `ReverseIdleKts`. Live: N1 44 %, reversers 43 % open.
- Jetways: helper variants with the door moved sideways (`jetway.HelperVariantTitle`, `PackageOptions.SideSteps`/`SideStepM`, `Helpers.DockAs`). At a gate where the simulator parks off the line, our aircraft stands on the line and the variant's door makes up for the offset. They need the helper package rewritten (examples/jetway-helpers) and MSFS restarted.
- `StandAllocator.TaxiIn` and `WarmTaxiIn`: each stand's taxi-in from a runway end, worked out once and kept. The World warms them in the background as an airport loads.
- The World logs each docked aircraft's distance from its stand line, flagging anything over 1 m.

### Fixed

- Traffic stutter: every moving aircraft jumped about once every 5 s (live, LKPR):
  - The per-frame traffic checks (blocking, head-on) cost 3.7 ms per aircraft, with frames of over 100 ms; now 0.13 ms. Another aircraft's way counts for 400 m, the checks work in flat metres, and the path is sampled once, with pooled buffers.
  - An arrival's stand pick searched about 200 routes on the connection's goroutine, taking 197 ms; now 0.2 ms.
  - A pushback plan created 450 MB of garbage; now about 150–220 MB. Curves are built into slices grown once, the route search's scratch is pooled, and one plan runs at a time.
- A departure cancelled while its pushback was being planned in the background crashed the map (send on a closed channel).
- Jetways: a helper whose jetway was still moving to it is sent home with the retract plan before it is removed. Removing it outright left that jetway stuck halfway for good (live, LKPR: every later dock there timed out).
- Jetways: the retract plan is written once per airport and never taken under the backend's lock: a lock-order risk.

## [0.94.0] - 2026-10-12

### Added

- Camera: `WorldPath`, a designed shot keyed in the world on a replay's clock (`WorldKey{T, Lat, Lon, AltM, HeadingDeg, PitchDeg, FovDeg}`). It looks at the replayed aircraft (its live object, or its recorded track), a fixed point, or where each key aims (`LookAt`, `LookAircraft`/`LookPoint`/`LookFree`). `Validate` checks the structure, and `Long` warns past `MaxShot` without cutting the shot. `Cue` puts it on a `Timeline` for `Director.Follow`.
- Camera: `KeysOf` turns a preset move into editable world keys at a replay time. `FovFromFocalMM` and `FocalMMFromFov` convert focal length to field of view.

## [0.93.0] - 2026-10-12

### Added

- Jetways (#572): our aircraft park at a gate the way the simulator parks there, so the jetway's hood meets the door. The nose gear sits a measured inset inside the parking circle, and the doors are matched door to door with the helper (`addons.ModelDoor.NoseGearFt`, read from `[CONTACT_POINTS]`). Each gate's inset and its sideways offset from the parking line (live, LKPR: 5.2 to 6.1 m and up to ±0.6 m, per gate) are measured on the placeholder or helper the simulator parks there (`jetway.HelperOptions.OnPlaceholder`). Live, LKPR: helpers within 1 cm of our aircraft on every gate measured.
- `TaxiRequest.StandSide`, `ArrivalRequest.StandSide`, `ArrivalOptions.StandSide` and `StandPointSide`: the stand stop moved sideways off the parking line.
- `jetway.Helpers.HelperFor`: the helper title used for a model.
- `RunwayUser.AlongM`: how far down the runway a lined-up aircraft stands.

### Fixed

- Runway: two of ours lined up at once (one at the threshold, one at an intersection further down) each waited for the other. Now each is considered, and the one further down goes first (live, LKPR: TVS1663 and PHVTA).
- Airport map: aircraft vanished from the map while jetways were docked. A traffic scan that ended on a jetway helper was never handed on.
- Jetways: a gate is retried at most 2 times per model, so retries no longer steal other gates. The door fit is logged against the aircraft on the stand, not one that has left it.
- Airport map: after a restart the page reloads only once the server is connected and has the airport, so it no longer comes up without aircraft.

## [0.92.2] - 2026-10-12

### Fixed

- Jetways that reported docked but never moved. Live at LKPR, the 737s at A4, C22 and C30 showed docked in the jetway data while their jetways stood at home: their helpers had been moved after the sim placed them. Helpers now stay where the sim parks them, and the door offset is only logged. `HelperOptions.MoveHelpers`, off by default, moves them anyway.

## [0.92.1] - 2026-10-12

### Fixed

- A departure stuck short of its holding point, live at LKPR. HBGOS gave way for good at B to OKHAQ's line-up path after OKHAQ had taken off: an aircraft's last reported ground path stayed in the picture while its position ran on. Now a path that no longer starts near its aircraft (80 m) is ignored, for giving way and for oncoming traffic.
- The ground mover stands still once it has braked onto its stop, instead of creeping at a thousandth of a knot. Creeping, it was never "stopped": it neither reached its hold nor logged why it stood there.
- The airport map serves its page with `Cache-Control: no-cache`. A browser kept the last build's script after a map restart (the aircraft icons gone until a hard refresh).

## [0.92.0] - 2026-10-12

### Added

- Boarding at a jetway, live at LKPR:
  - A departure at a gate whose jetway can dock asks for its push only once the jetway has docked and boarded `JetwayBoarding` (3 min). It waits for a jetway still to come, up to `JetwayWaitMax` (3 min), including while the gates are still being filled. Live, departures due at once pushed before their jetway came.
  - About half the departures (`TugAfterJetwayShare`) have their tug set off only once the jetway is home; the others' tug hooks up during boarding. New: `TaxiRequest.TugWait`.
  - `jetway.Helpers.Filling`.
- The airport map's page reloads itself when the map server restarts. The server announces a boot id on `/api/events`. Live, the aircraft icons were gone after a restart until a hard refresh.

### Changed

- The ground stop holds pushes from 8 aircraft taxiing (`DefaultMaxTaxiing`, was 4). With 4, departures waited on their stands at LKPR while the taxiways were far from full.

### Fixed

- Scheduled departures take the gates whose jetway can dock first too. Their stand was chosen without that preference, and departures at C27 and C31 got no jetway.
- An aircraft is told to give way at most once in 30 s, whoever to. Live, CSA1146 was told six times in 30 s as the traffic to give way to changed.

## [0.91.1] - 2026-10-12

### Fixed

- Ground's give-way call describes an aircraft pushing back as "pushing back", and one pushed and waiting to taxi as "about to taxi". It used to read their heading, which in a push points away from where they go. Live at LKPR, TVS770 was told "give way to the Boeing 737 passing right to left" for TVS1309 pushing onto J, which then came at it head-on. `GroundPicture.Pushing` and `Waiting` are new.

## [0.91.0] - 2026-10-11

### Added

- `World.Phrase(id, action)` (`GET /api/control/{id}/{action}/phrase`): what a clearance would say, without giving it. It returns the controller's transmission, worded as the engine words it (runway, taxi route, wind, line-up-behind traffic, FAA wording at US airports), and the crew's readback. For an ATC screen's "you say". `Radio.Worded` is the same for any transmission.
- `ControlView.Sequence`: an arrival's place in its landing sequence (number, leader, spacing, minimum).
- `pkg/camera`, for a replay shot designer (guide: [Shots on a replay](docs/camera.md#shots-on-a-replay)):
  - `Moves()`, the drone moves listed with name, doc and kind;
  - `Placed` and `WorldPoses`, a shot's eye and target in the world from the aircraft's recorded pose, for a map preview;
  - `Timeline`, `Cue` and `Director.Follow(timeline, clock)`, shots played on a replay's own clock, through pauses, seeks and rate changes (`ErrFilmEnded` at the end).
- `flight.Events(track)`: a recorded flight's milestones for a replay timeline. They run from engines start, pushback and taxi, through the take-off roll, rotate, lift-off and gear up, to touchdown, vacate and park, each found from the samples alone. Guide: [Milestones](docs/flight.md#milestones).

## [0.90.1] - 2026-10-11

### Fixed

- Head-on on one taxiway, live at LKPR. TVS770, told to give way to TVS1309, turned into J and met it nose to nose; then the two passed through each other.
  - A pushed aircraft waiting for its taxi clearance now counts as oncoming along its planned way.
  - Every give-way stop is at the last junction before the conflict, clear of it, never turned into the stretch the other comes along. Short of that point the stop always holds.
- Types with their own airstairs (CRJ, ERJ, ATR, Dash 8 and others) get a stand without a jetway first. Gates were tried first, so they only ever got a gate (live, every CRJ7 at LKPR).

## [0.90.0] - 2026-10-11

### Added

- Jetway door fit. The jetway docks to the invisible helper's door, not ours, so each helper is now put by our aircraft at our model's jetway door minus the helper's, read from both models' `interactive_point.0`. Live at LKPR, the sim's own place was 1–4 m off: the A321 and 737 hoods stopped forward of the door, and an A320neo helper on our aircraft docked 3 m aft. The jetway data's `ExitDoorRelativePos` confirmed the helper's config door.
  - `HelperOptions.AircraftAt`, `HelperOptions.Offset`; every placement is logged.
  - Overrides by type: `jetway.DoorOffset`, the dict table `jetway.doorOffsets`, `GET /api/jetways/offsets`, `POST /api/jetways/offset` (kept in `DataDir/jetway-door-offsets.json`).

### Fixed

- Jetways:
  - A gate whose jetway another helper or a placeholder takes is cleared and docked again a minute later. Before, it stayed without a jetway.
  - Switching jetways off and on fills the gates again. Before, it gave up at once after a fill that found none.

## [0.89.2] - 2026-10-11

### Fixed

- Website: docs pages use the full width between the sidebar and the table of contents, with no 52rem or 65ch cap on the main content. It looked poor on laptop screens.

## [0.89.1] - 2026-10-11

### Documentation

- Jetways:
  - Why a workaround: MSFS 2024 has no way to dock a jetway to a NonATC aircraft. It links the developers' request, [Jetway control for non-default ATC controlled AI aircraft](https://devsupport.flightsimulator.com/t/jetway-control-for-non-default-atc-controlled-ai-aircraft/17529).
  - What other programs see:
    - the invisible placeholders and helpers are real ParkedATC aircraft, which traffic maps, radar and other SimConnect add-ons list (title "MyCrew Jetway Helper", ATC ID JP…/JW…);
    - a placeholder carries a flight plan to the nearest airport;
    - the gates they hold count as taken for MSFS ATC and its AI traffic;
    - hosts should leave them out with `jetway.IsHelper`.

  Guide: [Jetways](docs/traffic-world.md#jetways-572).

## [0.89.0] - 2026-10-11

### Added

- Jetways: with jetways on, the stand allocator gives airliners the gates whose jetway can dock first, before their airline's own gates without one (`StandRequirements.Prefer`, `jetway.Helpers.Dockable`). A departure at a docked jetway boards 3 minutes before it asks for its push (`world.JetwayBoarding`). Guide: [Jetways](docs/traffic-world.md#jetways-572).
- `jetway.PackageOptions.WingSpanFt` (`-span` in `examples/jetway-helpers`): every helper's wing span set (measured: a small span does not change which gates the simulator gives, and no jetway docks to it).
- `TrafficManager.SetAtRunway`, `ManagedFlight.AtRunway`: a departure in the runway queue.

### Fixed

- Jetways, live at LKPR:
  - A jetway that went home for the push is not docked again; a gate whose aircraft has gone sends its jetway home. Live, the jetway docked again onto the empty gate as the aircraft pushed.
  - The push waits until the jetway is home, at least 25 s after it left. The jetway data said home after 16 s, while it was still moving.
  - Only gates a placeholder holds are docked (`jetway.ErrNoGate`). The simulator puts no ParkedATC next to an occupied gate, so the helper went to another gate or a ramp.
  - Docks wait until the gates are filled (`jetway.ErrFilling`), and one helper at a time. Two at once took each other's gates.
  - The helper is created 3 s after its gate's placeholder went (`HelperDelay`).
  - A helper no jetway takes is removed after 12 s, and a dock that timed out removes its helper. Either way the gate is filled again and the dock tried a minute later.
  - A fill that found no gate is retried. After a simulator start, the first placeholder went unclaimed and the gates were taken for full.
- Ground stop:
  - Departures in the runway queue (holding short, lining up) no longer count as taxiing. Live, there was a ground stop with the taxiways empty.
  - A held departure is told to expect a delay instead of being cleared to push and then sitting there.

## [0.88.0] - 2026-10-11

### Added

- `pkg/systems`: `State.Class` (`airliner`, `ga`, `helicopter`; the profile's, else from the sim's ENGINE TYPE and maximum weight), `TakeoffComputer` (`MCDU`, `FMC`), `ReducedThrust` (`FLEX`, `SEL TEMP`) and `ChecklistFamily`, from profile keys `class`, `takeoffComputer`, `reducedThrust`, `checklistFamily`; the A320 family and 737 bases set them.

## [0.87.0] - 2026-10-11

### Added

- Jetways for our traffic (#572), off by default: `pkg/jetway`. MSFS 2024 docks a jetway only to an aircraft in its ATC parking table, and our traffic is NonATC; an invisible ParkedATC helper of the aircraft's type, created under it on the gate, has the jetway dock (the jetway research session, live at EDDM 2026-10-10). `jetway.Service` (`Available`, `Prepare`, `Release`, `Dock`, `Undock`, `Status`): the engine never depends on it, and a direct simulator API, once there is one, is another backend behind the same calls. `Helpers` is today's: our airports' jetway gates held by placeholders (each made, its claimed jetway retracted by a flight plan, released `RetractWait` after: the sim then gives the next ParkedATC the one free gate), the helper put on a gate once its aircraft is parked, removed before the push (ground waits for the jetway home), the placeholder back. `WritePackage` writes the helper models from the installed traffic models (their own cfgs, the door they dock to; our FLTSIMs, gates only, no LODs), examples/jetway-helpers writes them into Community; `IsHelper`, and the World leaves helpers out of its traffic. World: `SetJetways`, `Jetways` (per gate: reserved, docking, docked, undocking, failed; unavailable with the reason), `GET`/`POST /api/jetways`; types with their own airstairs never get one. `engine.Message.AsJetwayData`. Not yet verified live: the sim accepting the generated package, and the flow at several airports.

## [0.86.1] - 2026-10-11

### Fixed

- Procedure paths: a course-to-fix leg the current heading crosses before its fix is joined where it crosses, with the turn onto it (`courseIntercept`; a turn of 135° at most, room for it on both sides), not by a Dubins path to the fix: LKKB ILS 24 via SULOV from D140J onto 237° short of CF24, drawn with a loop (MyCrew). `SmoothPath` leaves the corners of arcs already sampled (under 6°) as they are and drops points closer than a meter: rounded again they zig-zagged and stepped back, the loops at LKKB and EDDM AKIN1N.
- Flight plans: airports closer than `ShortHopNM` (40 NM) may be flown without a SID and a STAR, from the runway onto the approach, when that is shorter and the player chose none: LKPR to LKKB was planned 120 NM out on VENO4M and back on GOLO4W, now 37 NM onto the ILS 24 at D140I.
- Flown routes (`FlownRouteFor`, what the app and the map draw): approach and missed legs turn at the approach radius (1.8 km, was 2.5 km), and without a SID the climb-out turns round toward the arrival when no route fix follows (a short hop drew a hairpin back over the runway). With the two fixes above, LKPR to LKKB ILS 24 via SULOV is drawn with every turn a curve (no corner over 5°, no loop), with or without VENO4M and GOLO4W (`TestFlownLKKBNoLoops`, on the app's cached layouts and procedures).

## [0.86.0] - 2026-10-11

### Added

- Stairs at the model's own door (#1026): `addons.DoorsByTitle` reads the doors of every installed model by the titles our traffic spawns (a livery with its base model's, `base_container`), `addons.DoorsByType` by ICAO type as the fallback, from each model's `flight_model.cfg` (the frontmost main door on the left, and `interactive_point.0` for jetways) with its wing span; the World loads them in the background on Windows (about 0.7 s over 3,400 titles; or a host gives them, `World.SetModelDoors`), and stairs stand at that door (`SimObjectStairs.Door`, `DoorAheadMeters`, `StairsSpotAt`). A type without one keeps the wheelbase estimate. The user saw stairs miss the door on every type: against the installed FSLTL models the estimate was about 2 m too far back on the A320 family, 0.5–1.2 m too far forward on the 737, A220 and E190, and 5 m back on the A330 and 777. A type not installed (B38M, CRJ9) flies as an installed model, whose door is the one used; of the titles spawned on a live evening at EDDM all but one stock bizjet (no stairs anyway) resolved.
- Types boarding by their own airstairs (the user: CRJ, Citation): a new table `traffic.noJetway` (`NoJetway`; CRJ, ERJ, ATR, Dash 8 and other airliner turboprops, business jets) gets no stairs vehicle, prefers stands without a jetway (`StandRequirements.NoJetway`: its airline's, then any, a gate only when none other is free), and gets a GPU when parked at a gate.
- Model providers (MyCrew, the user): where our traffic's models come from, each switched on or off. A title's provider is its package's (`addons.TitleSources`: every installed SimObject title with its package and whether it is flyable, read on Windows with the doors), named by `traffic.modelProviders` (FSLTL, AIG, GSX, MyCrew, MSFS; else the package's creator); a title the simulator lists that no package on disk has is MSFS's own (packed). Flyable add-ons (titles with `isUserSelectable = 1`: FlyByWire, Fenix, any study-level aircraft) start off, the rest on. A provider off is passed over for aircraft (`ModelsFor`) and ground vehicles (`UsableModels`); when nothing of a provider on is there, any is taken and logged once (`OnProviderFallback`). `traffic.ModelProvider`, `ProviderOn`, `SetProviders`, `PreferProviders`; `World.ModelProviders` (per kind: aircraft, tug, fuel, stairs, gpu, bus, followme, with title counts), `World.SetModelProviders`, `GET`/`POST /api/models/providers`.

## [0.85.2] - 2026-10-11

### Fixed

- Line-up: a departure holds lined up, or rolls on, only once its body faces down the runway (within 0.5°), found by a dry run of its mover along the line-up path: the body swings in behind the nose wheel over a few wheelbases after the path is straight. Live at EDDM (glitch-watch), KLM1310 and ENT768 lined up 3.5° off 26L and the heading snapped as the roll began; every EDDM end was 3–4.5° off.
- Procedure paths: an RF arc ends exactly on its fix, its radius going over from the start fix's to the end fix's. Sampled at the start's radius, then the end fix appended, it made a short radial jog that the corner smoothing drew as a loop: EDDM AKIN1N from 26R turned ~330° round at DM044 (MyCrew, on the map and the charts).

## [0.85.1] - 2026-10-10

### Fixed

- Model rules: a rule whose title ends in `*` matches every title beginning with it (the exact title wins, then the longest prefix), and a shipped rule never uses any `MyCrew *` title as traffic. Live at EDDM, AFR342 was given the invisible "MyCrew Jetway Helper A320 AFR" and showed as a ghost.
- examples/glitch-watch: `-near meters` watches every aircraft and ground vehicle (tugs, GPUs, stairs) within that distance of the user aircraft, named by title, from the sim alone: for a host whose API needs a login (MyCrew).

## [0.85.0] - 2026-10-10

### Added

- Model rules (#1024): simulator models our traffic never uses, a new table `traffic.modelRules` (`ModelRuleItem`: title, kind, never, reason; `ModelNever`, `ModelRules`, `UsableModels`). A model never used is passed over for the next choice: aircraft titles (`ModelsFor`), the next bigger tug (`TugTitleFor`), the next stairs, GPU, bus or follow-me car, and scene models. Shipped: the small Mototok robot tug (the user: it does not show attached; light aircraft get the towbarless Mototok) and MSFS's "Car Ground Power Unit" van. The World keeps local rules in `DataDir/model-rules.json` over them per model (`"never": false` takes a shipped one back): `World.SetModelRule`, `World.ModelRules`, `GET`/`POST /api/models/rules`; the airport map's aircraft card lists its model and its vehicles' with "Never use".

## [0.84.0] - 2026-10-10

### Added

- Intersection departures by the runway left, class and queue (#1030). Ground offers an intersection once 3 (was 2) of our departures queue for the full length; which one follows the aircraft's class (`traffic.IntersectionClass`: light, turboprop, regional, jet, heavy) and its rule in the new `traffic.intersectionClasses` table (runway left needed: jet 2,500 m, regional and turboprop 1,800 m, light 800 m; widebodies the full length only). The first place along the runway after the full length comes first (at least 300 m in, the taxiways either side of one place together), deeper only while the runway left is enough and only where that place has no queue of its own; crews asking for one get the first their class may take. An airport may list its own entries per runway and class (`traffic.intersectionAirports`, none shipped). Some crews decline: "Unable intersection, request full length" and ground re-clears them (`traffic.DeclineChance`, by class, airline and the day's weight, drawn from the callsign). Where the airport table sets `sayRemaining`, the taxi clearance says "2800 metres available". Live at EDDM, AFR1910 (A320) got 26L at B10 with 2,250 m left and KLM1597 (E190) B10 with 2 queuing; a jet now gets B12 (2,808 m), and only after a queue of 3. `ParamRemaining`, `ParamNoReadback`, `IntentUnableIntersection`, `UnableIntersection`.
- World: `TitleFor(icaoType, callsign)`, the installed title the World would fly a type as (in the callsign's airline livery where installed), and `FleetFallback()`, a `flight.FleetOptions.Fallback` from it: recorded aircraft whose title is not installed here are created as the local model of their type (#1027).

## [0.83.3] - 2026-10-10

### Fixed

- Pushback: the push ends on the first taxiway of the route from the stand, the one its lead-in joins; a pose on another lane costs `pushOffRoutePenalty` (150 m), and a pose short of the start of its line is dropped where its nose lies at an angle on another named taxiway. Live at EDDM 214, LOT1300 for 26R was pushed facing 40°, angled across W2 down the D2sss apron line (its taxi-out across the remote apron looked cheaper); now onto W2 facing north or south for every runway.

## [0.83.2] - 2026-10-10

### Fixed

- GPU: it stays at the nose until the APU is on, `APUStartBeforeTug` (1 min) before the tug comes, after the stairs have gone; before, it left with the stairs, 2 min before the tug (the user: it went long before the push). A push cleared earlier still sends it off first (#1025).

## [0.83.1] - 2026-10-10

### Fixed

- Radio: ground vehicles are called by their own kind: "Follow-me 2", "Stairs 1", "GPU 3", "Bus 1" (each numbered on its own). Before, everything but a fuel truck was a "Tug" (live at LOWI: the follow-me car leading LOT913 was "Tug 2") (#1023).
- Follow-me car: it leads the aircraft to the stand's lead-in junction, then drives straight on along the taxilane up to `FollowMeAheadMeters` (40 m, on the taxi graph, never onto a stand or runway) before heading home; it no longer peels off 120 m short of the stand or drives down the lead-in line. Where no taxilane goes on it steps aside 10 m, not 30 m across the grass (live at LOWI). `SimObjectFollowMe.Graph` and `SetStandJoin`; the World sets both (#1023).

## [0.83.0] - 2026-10-10

### Added

- Traffic: backtracking. Where no taxiway reaches the take-off threshold (LOWI 08 and 26) and no intersection was given, a departure enters by the branch turning toward the threshold, backtracks along the runway, turns round within its width and lines up at the threshold for the full length; the tower says "enter runway 08 and backtrack, line up and wait" and never clears it for take-off before it has lined up. `TaxiController.Backtracks`, `Backtracked`, `ParamBacktrack`. LOWI layout capture in the testdata.
- World: ground vehicles on and off by kind (tugs, fuel trucks, stairs, GPUs, buses, follow-me cars): `SetGroundVehicles`, `GroundVehicles`, `Options.GroundVehicles`, `GET`/`POST /api/vehicles`; a kind off is never created and nobody waits for one, those at work finish.
- Nav: the player's choices in a plan (`FlightPlanRequest` and `world.PlanRequest`): SID, STAR and approach with transitions, cruise level, and an ICAO item 15 route flown over the known airways; what does not fit the runways or is not known is chosen as without it or flown direct, said in `FlightPlan.Notes`.

## [0.82.0] - 2026-10-10

### Added

- World: `Seed`, "real start": one real-world snapshot placed as flights of ours when a flight loads; parked aircraft depart after a turnaround, inbound ones arrive, outbound and overflying ones cross the area; nearest first within the aircraft budget; taxiing ones not placed yet. `ControlView.Seeded`, `Sighting.Seeded` (`POST /api/seed`).

## [0.81.0] - 2026-10-10

### Added

- Pilot: `PhaseGround` ("ground"), reported on the ground below 30 kt while the engine waits for the take-off or has handed back (the stand, taxi, a rejected take-off, the end of the landing roll); "takeoff" from the roll past 30 kt. Checklists due by phase no longer come due at the gate.

## [0.80.0] - 2026-10-10

### Added

- World: `Hold`, the live traffic held for a replay: the schedule stopped and every aircraft of ours removed; letting go gives the schedule back as it was (`POST /api/hold`, #1014).

## [0.79.0] - 2026-10-10

### Added

- Flight: `GhostFleet`, a Scene replayed in sync with a `Player` (pause, seek, rate): each aircraft created as NonATC when its time comes (its title, else the fallback), flown by the Injector, removed at its track's end, out of the budget (nearest first) or on Stop; a seek creates and removes as due (#1013).

## [0.78.0] - 2026-10-10

### Added

- Flight: `Scene`, the aircraft around a flight with their identity (callsign, title, livery, type, source) and own Tracks on the player's clock; `At`, `Span`, `Find`; versioned JSON lines in one file (#1011).
- Flight: `SceneRecorder`, on the player's Recorder: `Update` with the aircraft near the player records each at about 1 Hz from when it comes until it leaves, at most `Max`; `Snapshot`, `Stop` (#1012).

## [0.77.0] - 2026-10-10

### Added

- Checklist: `ImportMSFS`, an aircraft's readable MSFS checklist XML (FlyByWire's A32NX) into a `Set`: pages as lists due at their step's stage or phase, checkpoints as items with their words, simple tests of known SimVars as checks; verify only (#1002). Windows-1252 files read.
- Airport map: on simconnect v0.76.0 and voice-goio v0.16.0.

## [0.76.0] - 2026-10-10

### Added

- Systems: the captain's audio control panel, `acpVhf1Volume`…`acpPaVolume` (0…1) and `acpVhf1Receive`…`acpPaReceive` for VHF 1–3, INT, CAB and PA; none by default (full volume), the Fenix's `A_ASP_*_VOLUME` and `S_ASP_*_REC_LATCH` L:vars.

## [0.75.0] - 2026-10-10

### Added

- SimConnect input groups through the binding, engine and manager: `MapInputEventToClientEvent`, `SetInputGroupPriority`, `SetInputGroupState`, `RemoveInputEvent`, `ClearInputGroup` (#1006).
- `pkg/hotkeys` (#1006): keys and joystick buttons the player presses in the sim bound to an add-on's actions by name ("Ctrl+Shift+C", "joystick:0:button:5"), rebound from a settings file, mapped again on reconnect; IDs from 0xB100.

## [0.74.2] - 2026-10-10

### Fixed

- Airport: `FlownRouteFor` takes a SID or STAR given for another runway (filed before the runway changed) as not given and flies one of its family serving the runway (LOWW 11: LANU1E for LANU1F), and an approach for another runway as the runway's best; before, the route left the runway and looped back across the field to the SID's fixes.

## [0.74.1] - 2026-10-10

### Fixed

- Flight: the taxi-fast finding says the fastest taxi speed, not the first sample over 30 kt (live: 38 kt read "Taxied at 30 kt").

## [0.74.0] - 2026-10-10

### Added

- `pkg/checklist` (#1002): normal checklists as data, the A320 family's (Airbus FCOM) and the Fenix's on it, a local override file (local wins per value). Items: challenge, response, role (PF, PM, both, either), a check against `systems.State`, an action the PM may take (never gear, flaps, autopilot or thrust). Lists due by pilot phase or a caller's stage; a runner with the roles resolved by who is PF now. On recorded flights `ForAssess` gives `flight.Assess` real checklist findings (before take-off, landing by 1000 ft).
- Systems: beacon and strobe actions in the default profile.

## [0.73.0] - 2026-10-10

### Added

- Traffic: `ClearanceLimit(icao, city)`, the destination as said in a departure clearance: the city, but the airport's own name where a city has several airline airports ("cleared to Heathrow", "Orly", "Kennedy") and per-airport names (EHAM "Schiphol"); `SetClearanceLimits` merges a local table over the built-in one. The World's departure clearances use it, with the city from `World.SetAirportCity` (the host's lookup; the sim has only airport names).
- Flight: `Assess`, a recorded flight judged: its profile (taxi, take-off, gates at 1000 and 500 ft, touchdown and where on the runway), findings against common airline practice each with advice (hard or firm landing, unstable at 500 ft, tail-strike pitch, bank, speed limit, lights, taxi speed, bounces, long or off the centreline), and a score 0…100 to rank flights by.
- Traffic: replaying a recorded day (#845). `Flight.Path`, timed points (`PathPoint.At`): an arrival or overflight appears where its path has it when its en-route stage starts and flies the rest (an arrival to where it meets a STAR), a departure flies its SID then the path. Real and recorded flights share the same spawns.

## [0.72.0] - 2026-10-10

### Added

- Systems: `ReverseThrust` (0…100, reverse idle to full; by default the throttles). The pilot's rollout sets it, never the throttles, while the reversers are out. Pressing a `set` action writes its `on` value (TOGA as a lever position).

## [0.71.0] - 2026-10-10

### Added

- Airport: `FlownRouteFor`, a whole flight as the line flown: SID (or the runway heading, then on course), the filed route, the STAR, the approach through the transition where the STAR ends (else the nearest; the STAR's closing vectors left out), to the threshold, and the missed approach apart. Fixes carry their constraints, fly-over flags and phase. Without an approach, a synthetic 10 NM final with an intercept of 90° at most. `FlownRoute.Smoothed` rounds fly-by corners, never a fly-over fix or a reversal.

## [0.70.0] - 2026-10-10

### Added

- Systems: an action's `also`, further variables a `set` writes the same value to; `SetValue` on a `set` applies `scale` and `offset` (the Fenix's thrust levers: one L:var each, throttle 0…100 to lever 2…5).

## [0.69.0] - 2026-10-10

### Added

- Real-world traffic: overflights are flown (#845). One within 100 NM of the area's centre appears where it is now and crosses the area along its `route` (to the first point out of it), else straight on along its track; gone once out, or after its exit. `TrafficManager.AddOverflight`; a later sighting updates it, `Drop` ends it.
- Real-world traffic: a departure given a `route` flies its SID and then the route, at the route's levels or the planned cruise level (#845).
- Real-world traffic: `Observed.routeText`, the filed route ("DCT VLM UL86 KEPAD"), expanded over the known airways when no `route` is given. `nav.(*AirwayGraph).ExpandRoute` walks each airway between its fixes, passes over DCT, speed/level groups and tokens not known (reported), and picks the nearest of fixes sharing an ident.

### Changed

- Standard pushbacks are planned on demand: the stand of each departure placed, when not known yet, one at a time in the background, and saved after each. No sweep of every stand on an airport's first visit. `traffic.HasStandardPush`.

## [0.68.1] - 2026-10-10

### Fixed

- Standard pushbacks are kept per airport layout, not per graph: two graphs of one layout (the app's and the World's) share them, and planning on one no longer drops what the other planned. Before, the saved file held only the last stands planned, and the airport was planned again on every start (live: LOWW, about 3 CPU minutes per start).
- The World saves its standard pushes every 30 s while planning, through a temporary file, so a run stopped early keeps them; a file it cannot use is logged with why. The planner works 15 % of the time on one core (was 30 %).

## [0.68.0] - 2026-10-10

### Added

- Pilot: `Engine.GoAround()`, the pilot monitoring's "Go around" call, flown as ATC's on the approach or the landing (by hand or on the autopilot); ignored in other phases.
- Real-world traffic: `Observed.route` (#845), the way a real arrival goes on (points `lat`, `lon`, `altFt`): it flies it and joins the STAR where the route passes within 5 NM of a STAR point, the shortest way in; a route meeting none joins directly as before.

### Fixed

- Taxi routes stay on a taxiway instead of crossing to a parallel and back where it goes straight on (live LOWW: "L, EX9, M, EX6, L" from C36 is now "L, W, EX23, B2"). Applies to routes without given taxiways or via points, when the way along the taxiway is shorter, fits, keeps clear and crosses no more runways.
- After landing, arrivals stop clear of the runway and 25 m short of the next taxiway junction (`VacateJunctionMeters`), not on it. A junction too close to stop before it is passed, and the stop is short of the next one. New `ArrivalPlan.VacateBackMeters` and `VacateStop()`.

## [0.67.0] - 2026-10-10

### Added

- Pilot: ATC's go-around flown on the autopilot (without `HandFly`): TOGA, "Go around, flaps" with one flap step asked, the approach mode off, the runway heading held, level change to ATC's altitude, gear up, then the climb and the next approach armed again.
- Systems: `TOGA` action (`AUTO_THROTTLE_TO_GA`).

## [0.66.2] - 2026-10-10

### Fixed

- Ground: a route re-planned round traffic in the way that turns back on itself is refused (`Graph.TurnsBack`); the aircraft keeps its route and gives way (live LOWW: RYR1127 re-routed a second into its taxi turned round on the spot).
- Runway in use: kept while within its limits unless another gives `RunwayBetterByKts` (2.5 kt) more headwind; a preferred runway is still gone back to (live KSAN: 010/3-6 kt flipped 09 and 27 every ten minutes).

## [0.66.1] - 2026-10-10

### Fixed

- Pilot: the approach is armed and the descent asked for again after a go-around or `TakeControl` (both were once per engine).

## [0.66.0] - 2026-10-10

### Added

- Closed runways at the tower: no line-up, take-off or landing on one; ours on a final within 4 NM go around ("runway closed"); crossing it stays allowed.
- An airport with every runway closed: our arrivals in the air are told "all runways at LKPR are closed, expect holding" and hold at their STAR's fix. With a runway open again, the sequence releases them as from any hold. Held 20 minutes, each is cleared to the nearest other airport 30 NM or more away ("LKPR remains closed, cleared to LKKV, proceed direct"), sent direct toward it and removed once 25 NM from the airport. New radio intents `airport_closed` and `divert` (`traffic.AirportClosed`, `traffic.Divert`) with their readbacks.

## [0.65.0] - 2026-10-10

### Added

- Runway closures (works, a NOTAM). `World.CloseRunway(icao, runway, closed)` and `ClosedRunways`, with `GET`/`POST /api/closures`, close a runway by name ("06/24") or end ("24") or open it again. `nav.RunwayLimits.Closed` keeps closed runways out of the choice. A runway in use that closes is changed at once, even one held through a wind shift, and traffic on it changes runway as with a wind change; the ATIS and the player's runway follow. With every runway of an airport closed, no new flight starts there until one opens.

## [0.64.0] - 2026-10-10

### Added

- `traffic/world`: intersection departures for the queue. With two or more of our departures already queuing for a runway's full length (holding short, lining up or taxiing there), ground gives the next one an intersection with its taxi clearance: the named entry nearest it, never for a heavy, only where the runway left is long enough for the type. It departs from there instead of joining the queue.

## [0.63.0] - 2026-10-10

### Added

- Departure gaps follow the queue: with three or more departures waiting for a runway, the arrivals' gaps fit two departures each (a double gap, `DoubleGapQueue`). The gap is `DoubleGapExtraNM` (4.5 NM) wider than a single one, 10.5 NM in all instead of two 6 NM gaps, so a long queue drains while the arrivals keep coming. The tower releases the second departure inside the gap: on another route about a minute after the first (the arrival still about 8 NM out), on the same route two minutes after (about 5.5 NM).

## [0.62.0] - 2026-10-10

### Added

- `pilot` (hand flying): the go-around is flown, not handed back. It triggers when not cleared to land at minimums, or on `Clearance.GoAround` on the approach or the landing: the autopilot off, TOGA, "Go around, flaps" with one flap step asked of the player, the climb pitch for the approach speed + 10 on the runway track, "Positive climb, gear up", and the autopilot at the engage height to the cleared altitude. Crosswind landings: the final flies the ground track (a crab by itself), and below 10 ft the nose is straightened with the rudder while a wing held down into the wind keeps the track. The rollout uses the reversers (to 70 kt, idle reverse, stowed by 60 kt) and brakes for 4 kt/s, released at the handback. On the model: a 15 kt crosswind touches down 7 m off the centreline, 2° off the heading; 40 kt is reached about 885 m after touchdown.
- `systems`: `Reversers` (`SET_REVERSE_THRUST_ON`/`_OFF`, read as any engine's reverser engaged) and `BrakeLeft`/`BrakeRight` (0…100 on `AXIS_LEFT/RIGHT_BRAKE_SET`); actions take an `offset` after their `scale`.

## [0.61.1] - 2026-10-10

### Fixed

- The traffic director's Linux image builds again: `pkg/flight`'s recorder and replay were Windows-only, and the World's late-follower puppets (v0.51.0) use them, so the director failed to build on Linux and the images for v0.51.0 to v0.61.0 were never published. The pull-request check now builds the director for Linux.

## [0.61.0] - 2026-10-09

### Added

- `flight`: `Recorder.Watch(objectID, everyFrames)` and `Unwatch` stream an object's samples to the listeners without keeping a Track, for a pilot flying by hand every frame. Watched and recorded at once, one request serves both at the finer interval, and the Track keeps only its own share. `Recorder.Listen(f)` adds listeners beside `OnSample`.

## [0.60.0] - 2026-10-09

### Added

- `systems`: two kinds of knob for add-on FCUs, by profile, measured on the Fenix A319 live. A press with values (`on`/`off`) pushes and pulls a knob on one variable (+1, −1, released to 0), so managed and selected are one action. A relative encoder (`encoder`, `display`, `step`, `wake`) turns a counter by the clicks from the value shown to the one `SetValue` wants. A dashed display (0) is woken by one click first, with `ErrEncoderWoken` returned to be called again.

## [0.59.0] - 2026-10-09

### Added

- `flight`: `GhostReplay` replays a Track as an AI aircraft on its own. It creates the object where its `Player` stands, takes it over through the host's Injector when its object ID comes, flies it every frame as the Player says (play, pause, seek, rate) and removes it on `Stop`. For an app's ghost replay, as the World's puppets do it for late followers.

## [0.58.1] - 2026-10-09

### Fixed

- `traffic`: a pushback tug's straight leg between the vehicle roads and the aircraft's nose, driving in and home after the push, goes round the aircraft instead of through it. The aircraft is kept clear as its fuselage and its wing with the engines, each with a margin, and a leg into it goes by the nearer way round a wingtip. Live at LROP, a tug drove through a fuselage: 18 of LROP's 67 stands for an A320 had the way in across the aircraft.

## [0.58.0] - 2026-10-09

### Added

- `pilot` (#965 iteration B): hand flying (`Config.HandFly`). Given the runway (`Input.Runway`) and the take-off and landing clearances, the copilot flies the take-off: thrust set, the centreline by rudder, the rotation at VR at 3°/s to the learned pitch, the climb pitch for V2 + 10, "Positive climb, gear up", the autopilot at the engage height. It also flies the landing from minimums: the 3° glide path by pitch, the speed by throttle and the centreline by bank; a flare that eases the sink with the height; idle at 20 ft ("Retard"); the nose straightened below 15 ft; the nose lowered after touchdown; the rollout on the centreline; and "Your controls" below 40 kt. Not cleared to land at minimums, it calls the go-around and hands back. Learned values (`Config.Learned`) give the type's rotation, pitches and flare. Tuned on a model; needs a live check.

## [0.57.1] - 2026-10-09

### Fixed

- `traffic/world`: a layout found in the cache without its procedures (a host's `Options.Cache` holding layouts it loaded itself) has them loaded from the simulator once, while connected. Before, every departure failed with "procedures of LROP not loaded (yet)".

## [0.57.0] - 2026-10-09

### Added

- Runways: take-offs where no backtrack is needed. `airport.Graph.ThresholdEntry(end)` reports whether a runway end can be entered at its take-off threshold (an entry within 400 m, `RunwayEntries`), for that end in that direction only. `nav.RunwayLimits.ThresholdEntry` takes it. Parallels in use together where one can be entered at its threshold and the other only by backtracking (a 180 on the runway) become segregated: take-offs from the first, landings on the other, which landing traffic vacates ahead of its far end anyway. With entries on both, or on neither, the mode stays the spacing's. Live at LROP: 26L has no taxiway at its east end, so with a west wind take-offs use 26R and landings 26L; with an east wind both 08 ends have entries and the parallels stay independent. The World's tower, ATIS and airport info all take it.

## [0.56.0] - 2026-10-09

### Added

- `pilot`: handovers. The engine takes the controls at the first update airborne above the engage height, also in cruise, in the phase the flight is in (approach, descent, cruise or climb from the height, vertical speed, target level and distance). Before, it only engaged climbing through 1000 ft. `Engine.HandBack()` gives the controls to the player mid-flight ("Your controls") and `Engine.TakeControl()` takes them again ("I have control").

## [0.55.0] - 2026-10-09

### Added

- `traffic/world`: with parallel runways in use together, a departure takes the runway with the shorter taxi from its stand, counting each departure already on the ground for that runway as 900 m more (about a departure interval). A quiet airport keeps the near runway; a busy one hands departures to the other (live, LROP: the stands by 26L all queued for it while 26R stood empty). `World.DepartureRunway(icao, stand)` gives the same choice for the player's departure.

## [0.54.0] - 2026-10-09

### Added

- `flight` (#966): `Learn(model, tracks...)` learns how a type is flown from the player's recorded flights, each value the median of the flights that show it. Take-off: rotation and lift-off speed, lift-off and climb pitch, gear-up height. Climb-out: the acceleration height and the speed each flap detent was left at. Approach: the speed and height each detent was reached at, gear-down height, final approach speed. Landing: flare height and touchdown sink rate.
- `pilot`: `Config.WithLearned` takes a learned profile where the config leaves a value to its default (acceleration and gear heights, approach speed, flap step, landing flaps height).

## [0.53.1] - 2026-10-09

### Fixed

- `traffic`: `AirportLister` decodes an airport list by the entry size that fits the message and gives every entry a clean ident, a position on the globe and a sane elevation, trying the known sizes (MSFS 2024's 36, 40 or 41, MSFS 2020's 33). Before, it divided the message's bytes by the entries, and with a few bytes of padding in a short part 40-byte entries came out 41: every entry after the first was read a byte further off (live, MSFS 2024 at LROP: "?0?", "P", ",?R@@" thousands of miles away). Entries still not clean are left out, and the message is never read past its buffer.

## [0.53.0] - 2026-10-09

### Added

- `airport`: the taxiway gaps the graph joins are given for maps. `Graph.Bridges()` and `Layout.TaxiwayBridges()` return each as `TaxiBridge{Name, From, To, FromPoint, ToPoint, Length}`, and `Layout.FeatureCollection` includes them as taxi paths marked `"bridge": true`. `traffic/world`'s `GET /api/airport` returns them as `bridges`, and the airport map draws them as the taxiway they join (live, LROP's C was drawn open at the U junction).

## [0.52.0] - 2026-10-09

### Added

- `pkg/pilot` (#965, iteration A): a pilot flying for the user aircraft, logic only. It engages the autopilot at 1000 ft climbing ("I have control"). It climbs to the cleared level at 250 kt below FL100 and 290 above, asking the player as pilot monitoring for the gear and the flaps up as the speed allows. It descends from the 3-to-1 rule, asking for descent when none is cleared. On the approach it slows down configuration by configuration (each flown 10 kt under the next detent's limit), with gear down by 2000 ft or on the glideslope, landing flaps by 1500 ft and the approach armed when cleared. At minimums it disconnects and says "Your controls". Requests are done when the aircraft shows them, or by the copilot after `PMTimeout` when `CopilotActs`. Every height and speed is in `Config`. Docs: `docs/pilot.md`.

## [0.51.1] - 2026-10-09

### Fixed

- `airport`: gaps the scenery leaves in a taxiway are bridged when the taxi graph is built. Two ends of the same named taxiway, at most 40 m apart, each running on towards the other within 30°, become one taxiway, but never across a runway or at a hold-short point. Live at LROP, taxiway C stops at the U junction and goes on 21 m further, so stands 213 and 214 taxied round the airport: 3.6 km to 08R, now 1.7 km along C. The captured airports give 9 such gaps besides LROP's (EDDF, EDDM, EGLL, EHAM, LKTB); LKPR has none.

## [0.51.0] - 2026-10-09

### Added

- `traffic/world` (#964, review E26): a multiplayer follower joining late gets the flights already going on as puppets. The primary records each (`flight.Recorder`, five samples a second) and the director relays them only to the followers that joined after that flight began. Each of those creates the aircraft and flies it as recorded (`flight.Ghost`), one second behind the newest sample so it is always between two, jumping on when it falls 3 s behind. The puppet goes when its flight ends on the primary. Flights started after a follower joined run there as before. The director asks only the primary to stream (`wireClient.callVia`). IDs: the primary's recorder at 42100, a follower's creations 42200–42499 (`IDBase` +2100, +2200). Not checked with two sims yet.
- `flight`: `SampleFields`, `Sample.Row` and `RowDecoder` give a sample as a compact row of numbers, decoded by the sender's field names.

## [0.50.0] - 2026-10-09

### Added

- `pkg/flight` (#963): replay. `Player` is a Track's playback clock (play, pause, seek, rate). `UserReplay` flies the user aircraft as recorded: frozen, placed every frame, its gear, flap lever, spoilers, lights, throttles and control surfaces sent as they change; `Stop` frees it. `Ghost` flies an AI object as recorded through the traffic Injector, with its gear, flaps, spoilers, lights, engines and throttle following. Samples gain `FlapsHandle` (the lever, `FLAPS HANDLE PERCENT`).
- `traffic`: `Injector.PlaceFlown(objectID, FlownPose)` places a taken-over object as an aircraft was flown (altitude, pitch, bank, heading, on the ground), moved by the difference of the two models' static CG heights so another model keeps its wheels on the ground.

## [0.49.0] - 2026-10-09

### Added

- `systems` (#962): the autopilot by generic names. `State.AP` reads AP master, FD, A/THR armed and active, the selected heading, altitude, vertical speed, speed and Mach, managed or selected (the slot indexes), the modes engaged (HDG, ALT, VS, FLC, SPD, MACH, NAV, APPR, G/S) and armed (APPR, G/S, ALT). `Controls` sets them: AP and A/THR on and off, the selected values (`SetValue`), push and pull, the modes on and off, LOC armed. Flight controls for a pilot flying by hand: elevator, ailerons, rudder, throttles (all or per engine), flap lever, flaps one detent, gear, ground spoilers armed. The default profile uses the standard SimVars and key events, all checked against the SDK pages; an add-on's profile overrides them by the same names. Actions take `value` and `scale` for an event that carries a value (a negative one as two's complement). Not measured live: the slot meaning managed (2 assumed), negative vertical speeds, the elevator axis's sign.
- `registry`: the autopilot's slot indexes.

## [0.48.0] - 2026-10-09

### Added

- `pkg/flight` (#961): `Recorder` records any object, the user aircraft or AI, every sim frame into a `Track`: position, ground and CG height, attitude, IAS/GS/VS, gear, flaps (detent and %), spoilers, control surfaces, brakes, lights, throttle, N1 and reverser per engine, and the autopilot (holds engaged and the selected values). A Track is versioned JSON lines (gzipped for `.gz`) whose fields are read by name, so fields can be added; `At` and `Lerp` interpolate (headings the short way round, switches as the earlier sample). Docs: `docs/flight.md`.
- `registry`: 24 SimVars the recorder reads, checked against the SDK's SimVar pages or used live (the autopilot's FD, A/THR arm, G/S hold and arms, FLC, Mach; brakes, centre gear, CG height, flaps percent, spoilers armed, the lights, N1 and reverser); `GEAR HANDLE POSITION` takes percent over 100 as the SDK gives it.

## [0.47.0] - 2026-10-09

### Added

- `nav`: `CrawlOptions.Corridor` (`Corridor{From, To, HalfWidthNM}`, `Contains`): an airway crawl along a route, a band either side of the great circle rounded at its ends, instead of a circle around a centre.
- `traffic/world`: `PlanFlight` first reads the airways along the way from the simulator: a corridor 60 NM either side of the great circle between the airports, seeded from both airports' SID and STAR fixes and the known airway fixes nearest each, at most 3000 + 6 per NM fix requests (15000 at most; about 350 a second live), kept in DataDir/airways/route-DEP-ARR.json like an airport's and read again after `AirwaysMaxAge`. It waits for them up to 60 s (ctx sooner); without a connection loop (a director) it plans with what it knows. `PlanRequest.NoRouteAirways` skips the reading.

## [0.46.0] - 2026-10-09

### Added

- `traffic/world`: `World.PlanFlight(ctx, PlanRequest)` plans an IFR flight the way the World plans its own traffic's (`nav.Plan`): both airports loaded from the simulator, SID, airways (direct where the World knows none), STAR and approach, cruise level, distance, time and fuel; `FlightPlan.PLN()` gives the .pln for `FlightPlanLoad`. For an app planning the player's flight from where the aircraft is.

## [0.45.1] - 2026-10-09

### Fixed

- `traffic/world` player guard: a user aircraft seen on a runway or close in on its final blocks it even when its host's clearance still says holding short (a stale clearance, stopped mid-field after a rejected take-off); the clearance's holding-short entry used to win and nothing was blocked.
- `traffic/world` player stand: a stand label several spots share (a scenery's duplicates) is held at the one nearest the user aircraft; an unresolvable or unknown stand is logged instead of silently doing nothing.
- `manager`: a stall ends silently with its connection: the next connection's first message no longer tells a resume of the last one's stall.

## [0.45.0] - 2026-10-09

### Changed

- `traffic/world` link (review E24): the actuator sends the controllers' snapshot whole every 10 s and to each director newly attached, and in between only the reads that changed and the controllers gone, instead of every route and plan each second. A delta goes as a `ctlstate` feed with an empty array first, which a director from before cannot read as a snapshot and so ignores: it keeps working on the whole ones.

## [0.44.2] - 2026-10-09

### Fixed

- `traffic/world` tower view: a turn of more than a whole circle back left the yaw negative; it now stays in 0–360 (review #69).

## [0.44.1] - 2026-10-09

### Fixed

- `traffic/world` (review #85): a tower's grant times and judged rejected take-offs are dropped with the rest of a call sign's clearances, and the last positions of traffic gone from the scans for a minute are dropped; they were kept for good.
- `manager`: `TestOpenQuitForwarded` passes a whole OPEN record; `go test -race` (checkptr) refused a bare header. `-race` now passes for engine, manager, traffic/world, systems, camera, avionics, gsx, dict, nav, lvars, addons and internal (review #84).

## [0.44.0] - 2026-10-09

### Added

- `engine`, `manager`: `AICreateSimulatedObjectEX1` (a simulated object with a livery) and `UnsubscribeToFacilities` (the counterpart of `SubscribeToFacilities`; the manager forgets it for `ResubscribeOnReconnect`), both in the `Client` and `Manager` interfaces (review E2).

### Security

- `traffic/world` link (review E25): a JWKS RSA key under 2048 bits is left out (RFC 7518 3.3); PS256 accepts only a salt as long as the hash (RFC 7518 3.5) instead of any length; a link with session tokens (`Verify` or `TokenFunc`) but no TLS logs a warning, as anyone on the way could read and replay them.

## [0.43.1] - 2026-10-09

### Fixed

- `manager`: stall detection can take the consumer's own pause state (`WithStallPaused(func() bool)`, `Config.StallPaused`), say from `Pause_EX1`. Without it, it relies on the "Pause" event only, which in the MyCrew app has kept a pause with no resume: a stuck pause hid every stall, and a missed one could report a long pause as a stall.

## [0.43.0] - 2026-10-09

### Added

- `airport`: VFR points for LKTB (Brno), LKKV (Karlovy Vary) and LKMT (Ostrava), from the Czech VFR Manual (WEF 01 OCT 26): entry and exit points, with the holding points as route points. A test keeps every shipped point within 20 NM of its airport. LKVO publishes no points.

## [0.42.0] - 2026-10-09

### Added

- `manager`: `OnStall` and `Stalled()`. With no message from the simulator for `StallAfter` (5 s, `WithStallAfter`) while connected and not paused, the manager reports a stall, and a resume when data comes again, with the flight loaded meanwhile or just after (a new session). Live, MSFS died in flight with no disconnect or state change and the app saw nothing for 20 min.
- `systems`: the landing lights as a control (`Set(LightLanding, on)`): `LANDING_LIGHTS_ON`/`OFF` by default, the Fenix's switch `S_OH_EXT_LT_LANDING_BOTH` (on 2, off 1, from its L:var list, to verify live). Actions may give their own `on`/`off` values and an `offEvent`.
- `systems`: flap detents as said per profile (`flapDetents`, `State.FlapsSaid`): the A320 family "zero" … "full", a new 737 base profile "up", "1", "2", "5" … "40".
- `systems`: the minimums by default from `DECISION ALTITUDE MSL` (`DAFt`) and `DECISION HEIGHT` (`DHFt`, new); a model overrides them.
- `traffic.LandingETA`: time to landing along the route still to fly (its speed now, then 250, 180 and 140 kt from 40, 15 and 5 NM out).
- `World.CircuitConfig` and `World.PlayerCircuitJoin`: the circuit the World flies and how its tower joins the player from where it is.
- `world`: `Options.KeepOnStop`, `Options.Cache`, and the player's queries over HTTP (`/api/player/runway`, `/traffic`, `/circuit`).

### Changed

- When `RunOn`'s context ends, the World removes its aircraft, their vehicles and its en-route flights (`Options.KeepOnStop` keeps them).
- The World's console lines, its errors included, go to `Options.Output`. Several went to stderr, lost in a windowed app.
- `Layout.FrequencyFor`: a field without tower, ground, clearance or CTAF falls back to its flight information where the VFR data gives one (`FreqFIS`); IFR positions never do. Still false when there is none.

## [0.41.0] - 2026-10-09

### Added

- The player and the World coordinated on the runways, the ground and in the air (a review after the player was cleared for take-off with OKRAX lined up; all eight gaps found). See docs/traffic-world.md.
  - The user aircraft on a runway or close in on a final counts for the World's tower whether or not its ATC cleared it (it vanished below 3000 ft), on that runway and on every runway crossing it. A clearance onto 24 keeps ours off 12/30 too.
  - `PlayerCrossing`: a runway crossing while taxiing, which `PlayerRunway` answers too ("hold short", "cross behind").
  - Stale player clearances expire (airborne off the runway, landed and off it, crossed, or after a time). A take-off clearance never reported vacated sent every arrival on that runway around, forever.
  - On the ground the user aircraft's way ahead, or its push corridor, is in the ground picture each second (`GroundPicture.ReportUserMotion`): ours and the service vehicles give way to it, not only to its body.
  - `PlayerClearance.Stand` holds that stand for it (`PlayerStandOwner`). An arrival of ours not yet landed is moved off it.
  - In the air, a pair with the user aircraft is the tower's only below 1000 ft (`UserTowerBelowFt`), and `World.PlayerTraffic` gives its ATC traffic information on ours.
- `airport.VFRFor(icao)`: an airport's visual reporting points (entry and exit, route) and its flight information station, shipped for LKPR (Czech VFR Manual, WEF 01 OCT 26) and LKPD (06 AUG 26) with Praha Information 126.100 (AIP ČR ENR 2.1). Replaceable through the dict table `airport.vfr`. For the VFR phrases `ReportAt`, `AtPoint` and `ContactFIS`.

## [0.40.0] - 2026-10-09

### Added

- `World.PlayerRunway(PlayerQuery)`: the player's ATC asks before clearing the user aircraft to line up, take off or land, and the World's tower answers as it decides for its own traffic, on a copy of the runway's state (`RunwayController.Clone`): free, or what to say instead ("line up and wait", "hold position", "continue approach", "go around") with the traffic as said, its number for departure and when to ask again (`RunwayAnswer`). Live, the MyCrew app's tower cleared the player for take-off while OKRAX, cleared already, was lined up on 24.

## [0.39.0] - 2026-10-09

### Added

- VFR phrases through a controlled zone, with their readbacks (the MyCrew app's request): `ReportAt` ("report at NOVEMBER") and `ReportLeavingZone` ("report leaving the zone via NOVEMBER"), both read back "wilco"; the crew's reports `AtPoint` and `LeavingZone`; `FrequencyChangeApproved` (optionally with a squawk) and `ContactFIS` to flight information (`PosInformation`). See docs/traffic-vfr.md.

### Fixed

- CHANGELOG: the 0.37.0 heading, lost in the 0.38.0 entry, is back.

## [0.38.0] - 2026-10-09

### Added

- IFR go-arounds rejoin their own STAR's downwind (`goAroundToDownwind`): out ahead, across to it (5 NM off the centreline at LKPR, ERASU or RATEV), then the rest of the STAR and approach. They are back in the stream, where the sequencer can extend their downwind again. A straight-in without a downwind keeps the go-around circuit (live, CSA1958 sent round a 3.5 NM circuit inside the stream's downwind and back in at 10 NM).
- Go-around radio by the conventions:
  - The tower orders "go around, I say again, go around, traffic on the runway, climb to 4200 feet, fly runway heading", read back with the climb and heading (`GoAroundWith`). The reason names no call sign.
  - A crew going around on its own hears "roger, climb to 4200 feet, fly runway heading" (`GoAroundAcknowledged`).
  - The crew checks in with approach "going around, passing 2100 feet climbing 4200 feet", and approach answers "radar contact, maintain 4200 feet, expect ILS approach runway 24" (`RadarContactAfterGoAround`).
  - VFR circuit go-arounds are as before.
- A stretched STAR downwind is told when it is decided: "number 3, extend downwind, expect vectors" (`Absorption.Downwind`), with the base turn vectored. It is no longer a silent extension with a late "fly heading, for spacing" just before the base.

### Fixed

- "Make another circuit" is flown when it is said, not when it is decided. It is dropped if the arrival has been cleared to land or is landing by its turn on the frequency (live, OKVML re-routed 55 s before it was told, and told "make another circuit" after "cleared to land").
- VFR departures leave `VFRDepartAboveFt` (500 ft) above circuit height and arrivals come in at `VFRExitAboveFt` 1500 ft (was 1000 for both): 1000 ft apart on the same reporting point (live, OKLOF out and OKVML in via NOVEMBER at one height: TCAS RA at 0.8 NM, 88 ft).
- Traffic information is checked when it is said: dropped once the traffic is behind and moving away, else with the clock position and distance of that moment (live, "7 o'clock, 1 mile, opposite direction" 37 s late, the traffic gone by).
- Circuit instructions rank with landing clearances on the agenda, ahead of traffic information.
- Approach no longer gives instructions to an IFR arrival still going around with the tower, from the conflict watch either (live, CSA1958 "number 4, reduce speed" 12 s into its go-around).

## [0.37.0] - 2026-10-09

### Changed

- Talking speed follows the frequency's queue too: per minute of transmissions queued (readbacks, crews' calls), 0.5 faster (`tempoPerQueueMinute`, up to 1.3×). Live, ground ran 45 s behind with three pushbacks asked at once, and the agenda's new log showed answers 25 s to 1m23s after they were decided.

### Added

- Crews ask for another runway (#621): `crewRunwayShare` (2 %) of departures with a SID ask with their taxi request for the nearest runway end that has a SID to the same fix and at most 5 kt tailwind ("request taxi, request runway 30 for departure", `RequestTaxiRunway`). Ground gives it, re-clearing them to that runway and SID, only when it crosses no runway in use and takes no arrivals: our aircraft are coordinated per runway, so a take-off through a runway in use is not. Otherwise "unable, runway 24 in use" (`UnableRunway`). At LKPR, with 12/30 crossing 06/24, the answer is mostly unable; at airports with parallel runways it is given.
- Crews ask for a delay on the stand (#621): `standDelayShare` (3 %) of departures, once with ground and not yet ready, say "request delay on stand, about 10 minutes, waiting for passengers" (`RequestStandDelay`). Ground answers "roger, call when ready for pushback" (`StandDelayApproved`), and the push or start-up request comes 5–15 min later (`TaxiController.DelayPushback`, on the World's departure interface and the remote actuator).

## [0.36.1] - 2026-10-09

### Changed

- Crews go around from an approach "not stable" and reject a take-off on their own only rarely: 1 in 2000 each (`crewUnstableShare`, `crewRejectShare`; were 1 in 100 and 1 in 333).
- The controllers' agenda logs a call said 20 s or more after it was decided, with why it waited ("behind X", "the frequency busy until …"), to find slow answers (live, FVKNF's taxi request answered after 46 s on a quiet ground frequency).

### Fixed

- Fuel trucks park beyond the wingtip of a low-wing aircraft (span `FuelOffWingMaxSpanM` 25 m or less: light aircraft, business jets), `FuelWingtipClearMeters` 2.5 m out, not in the wing. Live, a fuel truck went through a small aircraft's wing at 7 m from the fuselage. Airliners are refuelled under the wing as before.

## [0.36.0] - 2026-10-09

### Added

- `pkg/systems`: copilot callout values (the MyCrew app's request). `State.SpoilersArmed` and `SpoilersPct` (the higher side deployed), and per engine `Reverser`, `ReverserPct` and `N1`. They read the default SimVars `SPOILERS ARMED`, `SPOILERS LEFT/RIGHT POSITION`, `GENERAL ENG REVERSE THRUST ENGAGED:n`, `TURB ENG REVERSE NOZZLE PERCENT:n` and `TURB ENG N1:n` (not measured live yet), and a model can override them in its profile. Value names `spoilersArmed`, `spoilersPct`, `Reverser(n)`, `ReverserPct(n)`, `N1(n)`.

### Changed

- Quicker line-ups: onto the runway and aligned at `LineUpSpeedKts` 10 kt (was 6) over `LineUpAlignMeters` 50 m (was 80). Live, a line-up and wait took 70–85 s from the holding point; real crews take about 30–60 s.

## [0.35.0] - 2026-10-09

### Added

- Tugs sized to the aircraft (`TugTitleFor`): GSX's Mototok Spacer 200 for light aircraft (span up to 16 m), the Mototok 8600MA for business and regional jets (up to 30 m), the airliner tug for the rest. The towbarless Mototoks sit closer to the nose wheel (`TugAheadFor`; their placement is still to be checked live). A spawn request's own `tugTitle` still wins.

### Changed

- Power-out from GA stands up to 25 m span (was 20 m): business jets such as the Citation Latitude, Legacy 500 and Praetor 600 taxi out under their own power where the stand faces the taxiway and the loop fits (live, pushed from S14A, S20A and S24).

## [0.34.1] - 2026-10-09

### Fixed

- An arrival in its flare past the threshold, over the runway, counts as landing now. It is no longer measured along its route as minutes out (live, TVS837 was cleared for take-off 2 s before OKZLK touched down, then the clearance was cancelled).
- No random "approach not stable" go-arounds. Our approaches are flown stable, and one with the runway free looked wrong (live, FTHAB).
- Approach says nothing to an IFR arrival going around until the tower hands it back (live, FTHAB told "number 3, reduce speed to 210 knots" 8 s into its go-around, on the tower frequency).

## [0.34.0] - 2026-10-09

### Added

- Line-ups in a rush: with `RushQueue` (2) departures at the holding points, or an arrival landing within `RushArrivalWithin` (4 min), the first departure behind ours on its take-off roll is told "behind the departing A320, line up and wait" (`RunwayClearances.LineUpBehindDeparting`, `ClearedLineUpBehindDeparting`). It is given only when the arrivals leave it time on the runway until its own take-off. From the full length it lines up once the leader rolls at 40 kt; from an intersection, once the leader is airborne. Before, it waited for the runway, and each gap lost a line-up.

## [0.33.0] - 2026-10-09

### Added

- Calls by urgency: separation instructions (stop descent, descend, the en route resolutions), go-arounds for spacing and TCAS acknowledgements go on the controllers' agenda in the new `prioSeparation` class. They are said in the first gap on the frequency, with no answer pause, before any routine call. Traffic information has its own class after them (`prioTraffic`). Circuit instructions, holds, the final approach speed and resumed climbs go on the agenda too, so routine calls no longer jump the queue.
- Talking speed adapts: `Transmission.Tempo` and `RadioOptions.TempoOf`. The World speeds a frequency up by 5% per call waiting (up to 1.3×), and to at least 1.15× for 15 s after a safety call. Readbacks follow the same pace, and the frequency is held for the shorter time (`Transmission.SpeakingTime`). The airport map passes it to its voice (voice-goio v0.14.0 `Utterance.Tempo`).

## [0.32.0] - 2026-10-09

### Added

- VFR circuit: "extend downwind, I'll call your base" (read back "extend downwind"), and the tower calls "turn base now" at the extended base turn (`ArrivalController.BaseDue`).
- `ArrivalController.DescendTo`: an arrival on its STAR descends to a level now.

### Fixed

- Sequencing at the merge: a new arrival goes ahead of one already sequenced on a converging STAR only when it reaches their shared fix a full merge spacing ahead. Otherwise it goes behind, whatever its time to the runway. Tactical swaps follow the same rule (live, OKGOZ on GOLO4S slotted ahead of EZY131 on LOMK8S, side by side at FL100; they met at 0.4 NM with TCAS RAs).
- No shortcut for an arrival with traffic within 10 NM and 2000 ft (live, OKGOZ sent direct PR517 2.6 NM from EZY131 at its level).
- Arrival conflicts: the trailer level with or below the other is sent down 1000 ft under it (`ArrivalController.DescendTo`), never climbed and no longer only slowed. The one-time level step now counts per pair, not per aircraft (EZY131's earlier one skipped it for OKGOZ).
- Conflict and TCAS checks see our turnarounds by their own call sign, not the arrival's ATC ID (live, KLM1433 seen as KLM185, so it was not steered as our departure).
- Spoken taxi routes: a single short taxiway between longer ones is said. Only chains of short stubs are left out (live, AFR898 "via JB, J, B" skipped 128 m of D).
- Crews ask "say again" for 1 in 200 clearances by default, down from 1 in 50, which came up too often.
- Sequencing: a newcomer, or an arrival moving up, passes one already sequenced only when that costs the one behind no more than a tactical swap may (live, OKUFC, a DA62 joining 11 NM out, moved up past TVS220 and TVS1568 and cost them minutes).
- Take-off clearances: an arrival lined up on the final within 4 NM counts by its straight distance, not along its route (live, OKUFC on a 1 NM final after another circuit counted as minutes out; OKQOL was cleared for take-off in front of it).
- Holds only at a named STAR fix, never at a point of a rounded turn (live, TVS1568 told "hold at WP0"). The dog-leg's vectors are dropped on entering the hold (it was told "fly heading 061" right after "hold").
- `DescendTo` and `BaseDue` are on the World's arrival interface and the remote actuator.

## [0.31.5] - 2026-10-09

Releases 0.19.0 to 0.31.4 are described in their GitHub release notes.

### Fixed

- Injected aircraft: no rest height learnt from a take-off roll or a rollout. `PlaceAir` placements now count as placements, so the samples of a roll are not taken for the simulator settling the aircraft (live, TVS495 learnt its own roll, 8.48 → 8.08 → 9.00 ft). Found on the #370 run with 31 aircraft.
- VFR circuit arrivals joining from beyond the base turn are no longer taken for on base. The route is now tracked from its first point, so the delay goes into a longer downwind (live, OKKKQ from N63 told to orbit before it reached its downwind).
- No orbits for spacing. A VFR arrival in the circuit gets its downwind extended, or another circuit when that is not enough, or a go-around once on base or final. IFR arrivals near the end of the STAR get vectors or the hold. `Orbit()` stays for emergencies.
- VFR departures are no longer cleared by radar to the IFR departure level (live, OKQQS out VFR via NOVEMBER told "climb to flight level 240").

## [0.18.12] - 2026-10-04

### Added

- Radio: `StationFor(layout, pos)` names a position and its frequency as the airport map says them ("Ruzyne Tower", "134.56"): the AIP's unit call sign where known (`AIPUnitName`; LKPR AD 2.18), else the scenery's name for what the frequency is; `KindPosition` and `FreqKind` map frequency kinds and positions. Moved from the airport map, which now calls it.
- `AirportLister.RequestAll()`: every airport the simulator knows, worldwide (`RequestAllFacilities`), collected by the same `Handle`; live in MSFS 2024, 85,723 airports in about a second. `AirportRef` gains `Region` and `AltM` (elevation in metres), read from the list entries.
- Sequencing: a compression buffer behind a slower leader, 1 NM per 30 kt the follower is faster on final, at most 2 NM (`CompressionMaxNM`; live, a B738 behind a PC-24 at 108 kt went around).
- Departures: on the same route a follower climbing 20 kt or more faster waits 1 min more per 40 kt (at most 3 min, `DepartureIntervalSpeeds`), and at the holding points one 40 kt or more faster, there within 2 min of the other, goes first (`RunwayUser.ClimbKts`; live, a B738 four minutes behind a C25C on VENO7D flew through it). The project's own numbers: no source read gives them.
- Arrivals: shortcuts. One with a minute or more of room ahead of it in the sequence is sent direct to a named fix further on its STAR, using at most 70 % of the room (number 1 up to 15 NM), only where it can still descend to that fix at 320 ft/NM or less, and only where it makes sense: a turn of 60° at most, the fix 10 NM or more from the threshold, never across the field (4 NM clear of the runway) or the final within 20 NM, with 20 NM or more of the STAR left (live, CSA1909 was sent direct PR532 over the airport); to the IAF at the latest, never a point of the approach beyond it, and not before it has flown 3 min and is reported over 100 kt (live, TVS1442 was sent direct PR532 two seconds after it appeared): "cleared direct to PR722" (`ArrivalController.Shortcut`).
- Pushback: a junction of another taxiway counts as blocked by a push pose only within 8 m of the fuselage, not under the wings (`pushBlockBodyMeters`): counting the wings sent a 777 at LKPR C22 the long way round for 24 (facing south-east, then a 180), 220 m more. Reviewed on the map before and after (`?overlay=`, a `variant` property draws the old plan dashed).
- Pushback tug: a tug never created (no object 20 s after it was asked for) is created once more 40 m further along its way in before the push goes on without it (`SimObjectTug.RetryCreate`): live, seven tugs of the day never came, each as another had just set off from the same depot.
- Airport map: departure gaps in the arrivals open for departures still taxiing within 2.5 km of their runway (about 5 min), not only for those already waiting there, when the arrivals close in are fixed and can no longer be slowed for them.
- Sequencing: tactical swaps. Two arrivals already in the sequence, neither fixed, change places when that cuts their delay by `TacticalSwapGain` (60 s) and costs the one moved back no more than `TacticalSwapMaxCost` (3 min); the new order holds (live, OKYDV could land before TVS223 turning base). Never a newcomer, a moved or a following arrival. An arrival swapped is not swapped again for `TacticalSwapHold` (3 min): live, without it RYR730, CSA1119 and CSA1009 traded places every few seconds.
- `airport.Charts(l, p)`: an airport's SIDs, STARs and approaches as charts show them (paths, fixes with constraints and roles, track labels, radar vectors), moved from the airport map so the MyCrew app draws the same; JSON as the map's `/api/procedures`. `Layout.DepartureStart`.
- Power-out: a small aircraft (span 20 m or less) on a general aviation ramp starts up and taxis out under its own power, no tug and no pushback: forward, a loop round to the side and back past the stand onto the taxilane, where the loop stays on the pavement and off any stand taken; else it is pushed (`TaxiRequest.PowerOut`). LKPR S16: a King Air out under its own power.
- `ReadPLN` reads the MSFS 2024 .pln layout (AppVersionMajor 12, SimBrief's "M24" export) too: the runways, SID, STAR and approach from `DepartureDetails`, `ArrivalDetails` and `ApproachDetails`; its waypoints carry no position (0,0). Golden test on CEF007 LKPR→LKPD: runway 06, BEKV1Q, RNAV 09.
- Conflicts: a departure is never told to reduce speed (live, AUA818 was told "reduce speed to 200 knots" climbing out). Crossing traffic is parted by altitude first, traffic on the same route (tracks within 45°, `SameRouteDeg`) by speed first, then a shortcut direct to a named fix ahead (`ResolveDirect`, `ConflictOptions.DirectFixes`; at least 5 NM away, at most 60° off) or a leg extended, altitude last. An aircraft climbing or descending is first stopped on its way ("stop climb at 4000 feet, due traffic"; `Resolution.Stop`), never sent back the other way, and once clear of the traffic it is cleared on to its planned level ("clear of traffic, climb to flight level 240"; `ContinueLevel`). A departure goes on to the level the departure radar cleared it to (FL240), others to a whole thousand (live, KLM704 "climb to flight level 192").
- Approach radio: the number in traffic is never said twice. The approach tab's "slow" (used by the conflict watch), the direct to final and the release from a hold went through their own calls and repeated it (live, AFR850 heard "number 4" and "number 3" twice); a call that would carry only a number already told is not made.
- Conflicts are predicted along the routes of our aircraft (`ConflictOptions.Route`, `RouteAhead`), turning where they turn: arrivals on their STAR and approach, departures on their climb, en route on their plan; other traffic still straight on (live, CSA786 was told to stop descent at 8000 ft for KLM130, predicted straight on where its STAR turned away).
- En route arrivals are handed to the arrival controller as they fly: it adopts the aircraft (`ArrivalRequest.ObjectID`) and sends it on along its STAR from where it is, instead of removing it and creating another at the entry; live, after a map restart CSA877 jumped 15 km and 11,000 ft (#643).
- `pkg/systems`: the user aircraft's power, radios, engines, brakes, lights, doors, transponder, flaps and gear through profiles as data: the standard SimVars by default, shipped per-model profiles (the Fenix A320 family on its L:vars, measured live), local override files winning per value. Guide: [Aircraft Systems Profiles](docs/systems.md).
- Fenix COM radios: the profile reads COM working from the RMP power (`L:B_PED_RMP1/2_POWER`, measured) and the frequencies from the RMPs (`com1Active`, `com1Standby`, … in MHz; `scale`), and gives the COM swap as the RMP transfer key (`actions`: `L:S_PED_RMP1_XFER`); `avionics.Radios.Use(profile.Actions)` presses it instead of the swap event. Checked live, powered: set standby, swap, restore. The earlier "the Fenix ignores the standby set" was an unpowered RMP.
- Airport map: a crew rejects its take-off on its own, rarely (1 in 300), between 40 and 100 kt: "CSA1, stopping". The tower acknowledges; the aircraft stops, vacates and taxis back to the holding point for a new clearance. Past V1 it goes on (#621). `traffic.RejectingTakeoff`.
- Airport map: a crew with the departure radar asks now and then for direct to a fix further along its route ("request direct VENOX"); cleared unless it is in a predicted conflict ("unable direct due traffic") (#621). `TaxiController.DirectTo`, `traffic.UnableDirect`.
- Taxi routes: of routes about as long (within 15 %, at least 150 m), the one past fewer stands wins, across no runway the other does not cross; the push choice uses the same last word. LKPR N51 to 06 now passes no stand instead of nine for 186 m more. `Graph.StandsPassed`, `FewerStands*`.
- `nav.ReadPLN` / `ReadPLNFile`: read an MSFS flight plan (.pln): departure, destination, rules, cruise altitude, SID, STAR and approach with their runways, waypoints with airways. `ParseLLA`.
- `pkg/avionics`: set the user aircraft's COM active and standby frequencies, swap them, set the squawk (key events, checked live in MSFS 2024; the Fenix ignores the standby set). Guide: [Radios and Transponder](docs/avionics.md).
- `traffic.VFRDepartureInstructions` / `VFRDepartureReadback` (CAP 413 Figure 24: "after departure, … climb not above altitude 2500 feet until reaching the zone boundary") and `traffic.Squawks`, discrete SSR codes from a bank avoiding codes in use and the special ones.
- `nav.RunwaySelector.Seed` (start from the runway in use already, e.g. saved by an earlier run), `Ready` (the moment of a change that is due, waited for at most `MaxChangeWait`, 15 min) and `Pending` (the change coming).
- Delay absorption: near the end of the STAR, a delay of about one turn or more is flown as a 360 where the arrival is ("orbit left for spacing"), smoother than out and back on a short leg (live, OKYDV). `Absorption.Orbit`.
- `ArrivalController.StopDescent` and `traffic.StopDescent`: an arrival on its STAR levels off above traffic merging below it ("stop descent at 7000 feet, due traffic").

### Changed

- Airport map: the radio voice runs on voice-goio's `speaker` package (v0.5.0), the same code the MyCrew app uses; the map keeps COM1 following and tuning, the HTTP API, the clip cache and the camera cut. The rules are unchanged (a voice per position with shifts and per pilot, one queue, 60 s lag, 1–5 s gaps, one frequency, the ATIS loop).
- Sequencing: arrivals inside the freeze distance keep the order they had: one established on the final is not passed by another closer in by its prediction (live, a DA62 from SIERRA on a short base took number 1 from AUA529 on a 5 NM final, which was sent around). A VFR aircraft in the circuit short of its spacing orbits, or goes around on the final.
- Airport map: an arrival conflict at the merge is met as a radar controller would: the later one stops its descent 1000 ft above the other (only while more than 20 NM out: closer in both are bound for the same final, and the trailer is slowed and vectored instead; live, TVS251 levelled behind a PC-24 at 108 kt on the same ILS went around), then speed and vectors, a hold only when nothing more absorbs it; and a sequence holds only with 4 minutes or more left (was 1). AUA529 held five minutes for a two-minute delay.
- Airport map, runway change: a transition, not a switch. The new runway is prepared for everyone at once, while departures holding at their runway, lined up or rolling take off from the old one, and at most the 2 nearest arrivals on its final land there; every other arrival is rerouted. The change comes once no more than those 2 are within 10 NM.
- Radio: "for spacing reduce speed to … knots" without "expect N minutes delay"; a delay of 5 minutes or less is never mentioned; nothing is said again unless the number or the speed changed, and "number N" only once an approach (a repeated speed call is just "CSA1, for spacing reduce speed to 190 knots"); after a go-around the tower hands over to approach once the aircraft is climbing away, not before "go around".

### Fixed

- Crew requests: a departure is cleared direct only when the direct itself keeps clear of all traffic (`PathClear`), else "unable"; before, only an aircraft already in a conflict was refused (live, PHGVV cleared direct DONAD was stopped at 4000 ft for TVS440 eleven seconds later).
- Schedule: a flight that can never be spawned (`ErrSpawnImpossible`, e.g. an overflight whose plan never enters the area) is cancelled at once instead of tried again (live, DLH112 OMDB → EGLL retried three times).
- Sequencing: an arrival is predicted at most at 250 kt (`TerminalKts`) in the last 40 NM (`TerminalNM`) before the final; a jet at cruise was predicted at its cruise speed down to the final and, as its place only moves up, took the place of a newcomer told its number a minute before (live, TVS440 at FL410 took number 2 from ENT1816). One with no speed yet (the first look after it is created or adopted reports 0 kt) is predicted at 250 kt too (live, TVS440 adopted at LOMKI: number 3 with 8 min delay, then number 2 a second later).
- Conflicts between our arrivals: one the sequence slowed less than 90 s before is left to fly that speed while the conflict is more than 3 min off (live, ENT1816 told 220 kt, then 210 kt three seconds later).
- Airport map voice: the last syllable of a call is no longer clipped now and then. voice-goio (#9) cut the end of each call by a once-measured length that varies from one synthesis to the next; it now cuts at the silence before its end marker. 19 of 80 test calls ended mid-sound before, 1 of 80 after.
- Conflict watch: our departures handed to MSFS AI are steered as well (speed, level or heading on their climb), said by the departure radar. Before, RYR1527 flew through OKCVY ahead of it on the same SID, at the same level (#639). `TaxiController.ClimbPlan` and `Reroute` give and change a handed-over departure's climb.
- Arrival exit choice: runways crossed on the taxi-in count (1000 m each), and crossing back over the runway just vacated is avoided whenever another exit allows.

## [0.18.11] - 2026-10-03

Add-on detection without SimConnect, and the traffic picture tells which airport an aircraft flies to and how it climbs from its altitude.

### Added

- `pkg/addons`: what is installed in the sim, without SimConnect. It finds the packages folder from `UserCfg.opt` and scans Community, Official and streamed packages (manifest fields; publisher and ICAO from streamed airport names). It also finds the package of the loaded aircraft, fingerprints the package set and snapshots the running processes. Guide: [Installed Add-ons](docs/addons.md).
- Airport map: a crew goes around on its own without a landing clearance inside 0.6 NM, or on a rare unstable approach; the tower acknowledges and the aircraft is sequenced again (#621).

### Changed

- Level of detail: full frame rate out to 8 km and half rate to 20 km (was 3 km and 10 km), so traffic in view no longer judders.

### Fixed

- `SIMCONNECT_RECV_SYSTEM_STATE`: `fFloat` is 4 bytes on the wire, so the string was read 4 bytes late ("bjects\Airplanes\..." for AircraftLoaded). `SystemStateFloat64` reads the 4-byte float (#634).
- Business and GA turnarounds keep the arrival's registration as the departure's call sign.
- `TrafficPicture`: a departing or arriving aircraft belongs to its origin or destination (`Observation.From`/`To`), otherwise the airport ahead of it (behind it departing): first one with a runway lined up with its track, then one with a layout loaded and the longest runway, then the nearest (OKLTU on LKPR 06 read LKHY). Before, it got the nearest airport: BAW1989 descending toward LKPR read LKKQ.
- `TrafficPicture`: climbing and descending come from the altitude (the reported rate only on the first scan), and `VSFpm` of aircraft other than the user's is the altitude trend (`VSDerived`). MSFS reports FSLTL AI on short final climbing (+500 to +940 fpm while descending about 1,100), so a landing read as departing. `ProfileMin` is now 4 s.

## [0.18.10] - 2026-10-03

The traffic picture reads AI on the ground right and tells more of what each aircraft is doing.

### Added

- `TrafficPicture` phases `pushback`, `holding`, `takeoff`, `landing`, `climbing`, `descending` and `approach`; `parked`, `taxiing`, `runway`, `departing`, `arriving` and `enroute` keep their meaning (#623).
- `PictureOptions.Layout` and `TrackedAircraft.Where`/`WhereName`: where on the airfield an aircraft on the ground is (runway, parking, taxiway, by `airport.Locate`) (#623).

### Fixed

- `TrafficPicture`: AI on the ground report 0 kt however they move, so every taxiing AI read as parked and no take-off roll was seen; below 1 kt the speed is worked out from the movement between scans (`SpeedDerived`), counting as movement above 2 kt (#622).
- `TrafficPicture`: climbing and descending follow a 30 s altitude trend with hysteresis (400/150 ft/min) instead of one scan's vertical speed, so an altitude blip no longer flips the phase (#623).
- Airport map voice: calls are spoken to their end (the last syllable was clipped).

## [0.18.9] - 2026-10-03

VFR traffic through the circuit (touch-and-goes, stop-and-goes, joins, reporting points, airspace classes), business aviation at large airports, airport detection (`airport.Locate`), and a day of live fixes: the gear after lift-off, take-off pitch, smooth stops and give-way on the ground, tug connection, arrival corners.

### Added

- `airport.Locate` and `airport.Tracker`: the airport a position is at, on the ground by the nearest surface (runways, taxiways, parking; the reference point of an airport without geometry) and in the air by approach and departure corridors, with aliases and fields inside larger ones handled; the tracker follows a flight (origin while climbing out, destination on approach). `tools/locate-eval` measures them on facility dumps: 99.99–100% on the ground, 99.1–99.7% in the air from one position, 100% for whole departures and approaches to a destination (#616).
- Business aviation at large airports (#619): `BusinessFlights` (business jets and turboprops, IFR between airports, on the GA apron), `LargeAirport` (a 3000 m runway and 10 gates), 20 business and mid-size types with published figures (Citation CJ3/CJ4/XLS/Latitude/Longitude/Sovereign/X, Phenom 100/300, Praetor 500/600, PC-24, SF50, PC-12, TBM 930, King Air 200/350, DA42, DA62, Baron). At a large airport VFR flights are half as many, mid-size and cross-country, with no training circuits.
- VFR flights flown by the airport's GA operators: a flying school, an aero club and private owners, with their fleets and training circuits (`GAOperatorsAt`, #565).
- VFR circuit traffic: touch-and-goes and stop-and-goes (#569, #567), circuit delays by a longer downwind then an orbit, go-arounds in their own circuit, another circuit, the standard overhead join; arrivals joined by the tower on the downwind, base or straight-in and never across the runway; reporting points (`ReportingPoint`, `DepartureVia`, #566).
- Airspace classes: separation where the class requires it, traffic information elsewhere (`SeparationRequired`, `TrafficInformation`, #570).
- Tower: a take-off with traffic close behind on final is "cleared for take-off, no delay, traffic on N mile final" (`RunwayClearances.NoDelay`).
- `types.DecodeJetwayData` and SIMCONNECT_JETWAY_DATA as the SDK documents it.
- Airport map: IFR and VFR scheduled traffic switched on and off separately (Schedule tab, `ifr`/`vfr` on POST /api/schedule); VFR traffic in its own colour; the VFR circuits and reporting points edited on the Airport tab and drawn in Layers.

### Fixed

- Departures: the gear stayed up from the first airborne frame — the simulator snaps it up when an aircraft is first placed in the air; it is held down (handle and gear positions, `Injector.HoldGearDown`) until the crew raises it between 300 and 700 ft.
- Take-off: lift-off at 6–7°, the climb pitch, then settling to about 10° once the gear is up (`TakeoffProfile.SettlePitch`); the take-off roll keeps the aircraft's rest height and pitch (no "wheelie" as the roll began).
- Aircraft taken over rest at the height and pitch they rested at (their gear compressed), not the static values that left them a foot high with the nose wheel off the ground.
- Ground: braking onto a holding point plans for the brakes coming on and the acceleration winding down; an aircraft released just short of it no longer stopped from 6–7 kt in one frame. An aircraft giving way slows down early and gently. Traffic ahead going the same way is followed, not given way to.
- Tug: no 3 m hop as it connects at the nose.
- Arrivals: corners keep their full radius through points in line with the legs (a 425 m arc where the turn needs 2.5 km).
- Sequencing: a VFR arrival told to follow traffic stays behind it; turning in from a short circuit it had been put in front of the jet it followed and cleared to land (`ApproachSequencer.Behind`).
- Tower: a take-off clearance is cancelled only for someone on the runway, never for an arrival closing in (a departure stopped on the runway sent the arrival around).
- Radio: registrations are said in the phonetic alphabet ("Oscar Kilo Victor Quebec Yankee"); the pushback facing names the nearest of eight points ("south-east").
- Airport map: the page no longer scrolls when the Airport tab opens (the map looked short); the vertical speed is derived from the altitude change (a landing aircraft showed climbing); the context panel no longer crashes for arrivals in the approach sequence.

## [0.18.8] - 2026-10-02

Fuel trucks, service vehicles that give way to aircraft and to each other, controllers who call the most urgent first, and VFR flights that depart, arrive on a schedule and fit into the landing sequence.

### Fixed

- Arrivals in conflict on their STARs: one that cannot slow down further now holds at once, and stays in the hold for at least 2 minutes and until the conflict is over. Before, the sequence released it a second later. Live, CSA1257 and AFR1552 merging on GOLO4S and LOMK8S met at 0.5 NM (#589).
- Light aircraft land shorter: they cross the threshold at 30 ft (`LightThresholdHeightFt`), and the exit choice uses their own touchdown point, speed and braking. They take the first exit they can make. At LKPR a C172 clears 24 at C in 56 s, where it rolled 83 s to D before; a PA-28 clears 30 at R in 31 s. Airliners keep their exits (#583).
- Airport map: an aircraft whose position did not change since the last poll is not dead-reckoned ahead. Frozen aircraft no longer jitter forward and back (#591).
- Stands: a stand next to one whose aircraft is due off within 8 minutes ranks lower (`StandRequirements.OffBlock`), so neighbours rarely push at the same time (#581).
- Pushback and start-up are approved together by default: crews ask for both in one call 85% of the time. The map's "with start-up" switch is on by default (#580).
- Tower: an arrival's time to land is measured along the route it still flies, not in a straight line. An arrival passing near the field on its STAR no longer holds every departure: live, RYR1485 "landed in 1m38s" 11 minutes early (#574).
- Tower: a conditional line-up or crossing ("behind the landing …") is given only behind an arrival established on the final. It also needs time for the departure before the arrival after that one: the first off the runway, the departure's roll, the margin. The room is checked again when the line-up happens; without it the crew is told to hold position. Live, a departure lined up behind a landing aircraft, and the next arrival had to go around.
- Tower: a vacating arrival frees the runway once it is clear of it (its reference point 40 m beyond the edge), not when it stops past the holding point, 35 s later live (#574).
- Departures: a rolling take-off aligns at 12 kt, not 6 (`LineUpRollingKts`). From the holding point to the take-off roll takes 55 s instead of 70 in simulation, within the tower's 60 s (#574).
- Tugs drive in from their depot smoothly: the aircraft waiting on its stand had its frames slowed, and the tug moved in jumps (#574).
- Sequencing: near the end of a STAR, with no leg long enough to stretch, a delay is lost by vectors from where the aircraft is, out and back to its next point, not in a hold. Live, LOT775 held at PR532 for a one-minute delay. Holds are for what 30 NM of stretching cannot absorb.
- Pushbacks turn wider and start their turn earlier, ending aligned on the taxiway (`PushWideRadiusCost` 3), wherever that is the same push, only smoother: facing the same way, no tow involved, aligned and no hairpin where the current one is, and at most 25 m longer (`PushWideMaxExtraMeters`). Reviewed at the ten test airports; LKPR C21, LFPG B2 and F14, KJFK A10 and LKPR A3 for 24 keep their push. To revert, set `PushWideRadiusCost = PushTurnRadiusCost`.
- `tools/push-review`: before/after overlays of the pushes with and without the wide-turn rule, for the map's `?overlay=` review. `traffic.PlanPush` plans a stand's push without a simulator (`PlannedPush`).
- Take-off: the nose holds its lift-off pitch to 35 ft and then rises a degree per 5 ft (it looked close to a tail strike). The gear comes up between 500 and 1000 ft above the runway, a height drawn per crew, and at the latest at the hand-over to MSFS AI. After a go-around the gear comes up 400 ft above the runway; it stayed down all round the circuit before.
- Aircraft we drive on the ground sit at their model's STATIC PITCH (0.7–1.2° for the airliners measured), not level, which had put the nose wheel into the ground.
- Tower: a conditional line-up counts the line-up itself (`LineUpWaitTime`, 85 s as measured): live, TVS158 took 84 s and the arrival behind had to go around. A departure already lined up may go with the next arrival 3 NM out (`MinArrivalLinedUpNM`), its time rule kept.
- Ground: an aircraft already in a junction when it should give way clears the junction instead of being pulled up inside it (live, QTR1788); a pushback is still always given way to.
- Sequence tab: a runway nobody uses no longer keeps showing its last user ("12/30 · WZZ100 on the runway" long after the crossing).
- Airport map: aircraft and their data tags are drawn above the taxiway signs and other labels.
- Airport map voice: about one controller and crew in nine speaks with a female voice (1:8), from voice-goio's documented speaker genders (`PoolOptions.FemaleShare`; VCTK's genders and accents from the corpus's speaker-info.txt).
- Tugs are sent `TugLeadTime` (3 min) before the crew is due to ask for the push, or at once when the push is cleared. They no longer wait at the nose ten minutes early: live, EZY775 at C29.
- VFR circuit arrivals stay with the tower from their first call: no approach clearance and no "established" report. "1 mile south", not "1 miles".
- The vacated report names the runway and the exit: "runway 24 vacated at D".
- Airport map: our aircraft are labelled with our call sign. A turnaround flies on in the same aircraft object, whose ATC ID the simulator keeps, so TVS1124 showed as TVS1482 on the runway.

### Added

- Fuel trucks (#582, #585): `TaxiRequest.Fuel` and `SimObjectFuelTruck`. A departure waiting on its stand long enough is refuelled before the tug comes. The truck drives from the nearest vehicle depot along the vehicle roads to the right wing and refuels for about 8 minutes (twice that for a widebody). It leaves by the nose or the tail, never across the aircraft, and the push waits for it. The map uses GSX hydrant dispensers at gates, GSX fuel trucks elsewhere, and MSFS's own fuel truck without GSX, with two fuel companies per airport. It is shown on the map as **F**.
- Service vehicles give way (#586, #592). Tugs and fuel trucks stop for moving aircraft crossing their way. They drive 2 m right of the road's centreline, so oncoming vehicles pass, and follow each other with a gap; side by side or crossing, a fixed tie-break decides who waits. After a minute waiting for a vehicle they drive on.
- Controllers call the most urgent first (#587). Each frequency has an agenda: a go-around, a landing, a take-off or line-up, approach, taxi for a vacated arrival, taxi for a departure, pushback, and a departure clearance last. Within a class the longest waiting goes first. A runway clearance no longer granted when its turn comes is dropped and given again later.
- VFR departures (#590): `Circuit.Departure` leaves the circuit towards an exit point 5 NM out by the side the exit is on, never across the circuit. `TaxiRequest.VFR` hands the take-off to MSFS AI at 400 ft. On the map: *VFR through the circuit* for departures too. There is no departure clearance; the aircraft calls ground first and stays with the tower.
- Scheduled VFR flights (#588, #590): `VFRFlights` adds light aircraft arriving through the circuit and departing, about one an hour each way. They fly by day only (`SunElevation`, `Daylight`: civil twilight) and in visual weather (5 km, 1,500 ft). Call signs are local registrations (OKABC at LKPR). They appear 8 minutes before landing (`ManagerOptions.VFRLead`) and park on GA ramps.
- VFR in the landing sequence (#594, part of #569). On its downwind report the tower gives a VFR arrival its place: "number 2, follow the Airbus A320 on 4 mile final". When it must lose time, its downwind is extended ("extend downwind") at circuit speed: no 210 kt instruction and no hold.
- Airport map strips show the scheduled time (STD for a departure, STA for an arrival), amber 6–15 minutes late, red beyond. Flights without a schedule keep the wait clock (#579).
- Airport map: IFR or VFR tag by the call sign on strips, in the panel and on VFR aircraft labels (#584).
- Airport map: an aircraft's popup with structure: call sign and kind, type and model, route with STD/STA, then altitude, speed, vertical rate, heading, phase, lights by name, gear when low, and span. `/api/traffic` gives each aircraft's ICAO `type` (#593).

- VFR circuit arrivals (#568): `ArrivalRequest.Circuit` and `PlanCircuitArrival`. The aircraft appears at the 45° entry to the downwind, MSFS AI flies the circuit, and the injected approach takes over on the short final (`ArrivalProcedure.MinJoinMeters`). On the map: New flight → *VFR: join the circuit*; per-airport circuit settings at `GET/POST /api/circuits` (`circuits.json`).
- VFR radio (#569): the Doc 4444 12.3.4.13–17 phrases, with readbacks: `VFRForLanding`, `JoinCircuit`, `StraightIn`, `CircuitReport`, `FollowTraffic`, `CircuitInstruction`, `CircuitDelay`, `ClearedTouchAndGo`, `MakeFullStop`. A circuit arrival calls the tower for landing, is told to join downwind and reports downwind. GA types are named on the radio ("Cessna 172").
- VFR traffic (v0.19, #431), first part. Light aircraft: C152, C172, PA-28 (P28A), DA40 and SR22 profiles from the published figures. They match the simulator's AI models (`Asobo PassiveAircraft …`) and are single-engined with light wake (#565). Circuits: `NewCircuit` builds a runway end's circuit (upwind, crosswind, downwind, base, final) with altitudes and speeds and the 45° join to the downwind (`JoinDownwind`). The side, height and leg distances are configurable per airport and runway end (`CircuitConfig`); defaults are left-hand at 1000 ft, with the downwind spacing taken from the aircraft's turns (#567). See [VFR Traffic](docs/traffic-vfr.md).

---

## [0.18.7] - 2026-10-02

Network play grows up: tokens to control or only watch (spectators), and changes pushed to every open map as they happen. The approach sequence opens gaps on final for waiting departures, and the tower clears a crossing conditionally, behind the landing aircraft.

### Added

- Airport map: optional access for network play. With `-token`, another device needs a link with that token to control the traffic. With `-view-token`, a device can watch as a spectator: it reads everything, changes nothing, and its controls are hidden. `auto` makes a random token. A link's token goes into a cookie and out of the address. This computer always has full access. The host sees the links in the Quick reference.
- Airport map: push updates. Open maps hear of a change (a clearance, a state, a transmission) as it happens through server-sent events (`GET /api/events`) and fetch it at once. The polls remain as a fallback; the radio polls slow down while the stream is up.
- Departure slots: with departures waiting at the runway (holding short, lining up, lined up), the approach sequence opens a gap on final for each one (`ApproachSequencer.SetDepartureSlots`). The gap is at least 6 NM (`DepartureGapNM`) and at least what the tower needs to let the departure go: the arrival ahead off the runway, then the next one still 4.5 NM out. The Sequence tab shows it as "departure gap".
- Conditional runway crossing: an aircraft holding short of a crossing, with only the next arrival in the way, is told "behind the landing A320, cross runway 12, behind" (`ClearedCrossBehind`, `RunwayClearances.CrossBehind`). It crosses once that arrival is off the runway.

---

## [0.18.6] - 2026-10-02

Radio call signs for every airline and air force ("Czech Air Force 001"). Working one position now turns off every control for aircraft on other frequencies. Audio played on another device follows frequency changes without replaying old calls, and it no longer stalls. New flight keeps the entry you picked, and its window is tidier.

### Added

- Radio call signs for every airline and air force: about 5500 ICAO designators with their telephony (`traffic.Telephony`), from Wikipedia's List of airline codes (CC BY-SA 4.0, attributed in `pkg/traffic/telephony.tsv`). `CEF001` is said "Czech Air Force 001", `GAF615` "German Air Force 615". Airlines in the schedule keep their own.

### Fixed

- Radio: short words in a call sign are said as words ("Czech Air Force", "Sky", "Jet"); only initialisms are spelled ("KLM", "CSA Lines", "UPS"). LOT is "Pollot", as assigned.
- Airport map, working one position: every control for an aircraft on another frequency is now off, not only the clearance buttons. That includes the approach actions and go-around, the Sequence tab's buttons, Manual, Entry, "with start-up" and "Clear up to here" on the map. The card shows the position working it instead of "sending…".
- Airport map, Play on this device: switching frequency no longer plays the calls said there before. Clips are fetched as soon as a call is heard and retried once; a stalled clip no longer stops the rest; calls more than 20 s behind are skipped (a tablet waking up). The server makes each clip once, however many devices ask.
- Airport map, New flight: the entry picked ("06 at E") stays when the aircraft type changes; if it is too short for the new type, a message says so.
- Airport map, New flight window: the kind (Departure, Arrival) sits in the header; the stand has a row of its own with *Pick on map*; one column below 900 px; *Clear stand* instead of *Hide route*; Ctrl+Enter spawns.
- Sequence tab: an aircraft spawned on the map no longer shows on a crossing runway as other traffic ("12/30 · CEF001 on the runway" while it took off from 06).

---

## [0.18.5] - 2026-10-02

The airport map on a tablet: install it as an app, keep the screen on, and plan a new flight in a window over the map. The panel docks left, right or at the bottom. Tugs drive back over the stand onto the service road instead of along the taxiways among the aircraft. Crossing reports name the taxiway. This is the first version under the Business Source License 1.1.

### Added

- Airport map: install it as an app on a tablet or phone (web app manifest, icons, a pass-through service worker). A full install needs HTTPS or localhost. Over plain http on the LAN, browsers add a home-screen shortcut instead; on an iPad it still opens full screen.
- Airport map: Keep the screen on (More menu), on by default on touch screens.
- Airport map: tugs on the map, with a T marker and their route on the vehicle roads as a dashed line (`SimObjectTug.Track`, `ControlView.Tug`).
- Airport map: a double click of the middle mouse button locks the tower look to the mouse. Moving the mouse turns the view and the wheel zooms it, with no button held. Another middle click or Esc unlocks it.
- Airport map: Enter in the strip filter selects the first matching aircraft and centres the map on it.
- Airport map: when the map server stops answering, the lost overlay says so and names the address.
- Airport map: New flight opens in a window over the blurred map, full screen on a tablet or phone. *Pick on map* and *Pick via points* move it aside for a bar on the map; it comes back when you pick a stand or press Done. Esc or a click outside closes it (the stand stays), and it closes after a spawn.
- Airport map: dock the panel left, right or at the bottom (Map → Panel), remembered per device. At the bottom, the strips sit in columns. A phone keeps the bottom sheet.
- Website: copy buttons for the airport map's run command on the landing and examples pages, with the links on their own line.

### Changed

- License: new versions are under the Business Source License 1.1 instead of Apache-2.0. Non-commercial use (personal and hobby use, the flight-simulation community, education, research, non-profits) is allowed; commercial use, such as a paid add-on or product, a paid service or use inside a business, needs a separate licence. Each version becomes Apache-2.0 four years after it is published. Versions up to and including v0.18.4 stay under Apache-2.0.

### Fixed

- Injected departures holding short of a runway they are to cross now name their taxiway ("holding short of runway 12 at F", spoken "at foxtrot"). They reported none, because the injected departure never tracked the taxiway it was on.
- Tugs no longer drive among the aircraft on the taxiways. After the push, a tug drives back over the stand the aircraft has left and onto the closest point of the vehicle road behind it. Driving in, it leaves the road at the point closest to the aircraft (`airport.Layout.VehicleRoute` joins the nearest vehicle road within `VehicleRoadReachM`, 150 m; `NearVehicleRoad`).
- Website: the airport map's run command is `cd cmd/airport-map && go run .`, and its GitHub link points to `cmd/airport-map`.

---

## [0.18.4] - 2026-10-02

A validation release: a live sweep and four code reviews, with what they found fixed. That includes a lock ring that could freeze the map, departures that stayed stuck, go-around circuits that started at the wrong corner, and tugs that held departures too long. The traffic log is readable. The squawk can be set. The tower camera turns by keys and the middle mouse button, with its angle shown on the map. Working one position greys out the aircraft on other frequencies.

### Added

- Airport map: working one position (As), the clearance buttons of aircraft on other frequencies are greyed out and their cards dimmed (the server refuses them anyway).
- Airport map: the tower camera's angle on the map, off by default (the Angle switch in Tower look). A cone of its field of view from the tower, labelled with its bearing, tilt and width; it follows the camera as it turns.
- Airport map: the squawk can be set in New flight (four octal digits; empty: automatic). The emergency codes are refused, and the aircraft's card shows it (`SpawnRequest.Squawk`, `ControlView.Squawk`).
- Airport map: the tower camera looking round can also be turned with keys (arrows turn, + and − zoom, Shift faster). Holding the middle mouse button and dragging turns it; rolling the wheel while holding it zooms. The middle button is caught before the map and the browser act on it.

### Fixed

- Airport map: a lock ring that could freeze the map. The traffic scan held the state lock while it reported traffic, the map's polling held the controller's then an aircraft's, and an aircraft handing off read the weather.
- Airport map: game aircraft get their call sign and model again; an inline comment had swallowed them (as #477).
- Airport map: a go-around from the Sequence tab runs on the SimConnect loop. Network play checks the position for manual control, entry, rush and the approach actions too.
- Runway: a call sign spawned again (a replayed scene) is not taken as cleared already. A departure aborted by hand is cleared again. A departure told to line up behind an arrival that goes around is cleared afresh. A take-off clearance is cancelled only for someone on the runway or an arrival inside 3 NM, not one just under 4 NM. "Go around" goes only to an arrival established on the final.
- Sequencing: an arrival slowed or broken off is handled afresh on its next approach. A second delay absorption at the minimum speed no longer sets a 0 kt waypoint or an infinite time to hold. A go-around's circuit starts at its first corner, not the nearest (which skipped the upwind and crosswind).
- Tugs: the departure starts once the tug is off the nose, not once it is home. A tug that fails or is never created costs 20 s, not 4 min.
- Runway in use: no flip with every gust when no runway is within the wind limits. Departures spawned from the map without a stand take the parallel nearest the stand they are given.
- Camera: the tower's look plays again after 30 min. Switching to the tower drops the shots queued before. A simulator camera picked right after ours is released is not set back.
- Airport map: procedures without runway transitions load. "Active" no longer keeps the previous airport's runway. A cancelled "Place on the map" no longer moves the tower on the next click. Play on this device no longer compares this device's clock with the server's. Following an aircraft wins over following your own, so the map no longer jumps. The aircraft card updates after picking an entry, and a failed entry list is asked again. The voice's output picker hides when the voice is off.
- Checks: a tower placed more than 5 km from its airport is refused, and the tower file is written whole. A COM1 frequency outside 118–137 MHz is refused. A runway record shorter than expected is not read. The debug camera probe is gone.
- Airport map: Space no longer pauses the simulation. The Pause button asks first, since it stops the simulation for everyone playing.
- Airport map: the traffic log is readable. One entry per row, newest first, with the time, call sign and kind on one line and the message under them (it was one block of wrapped text).

---

## [0.18.3] - 2026-10-02

Several runways at once, and a better view of the airport. Parallel runways are used together, with the mode set by their spacing. The tower camera works from the real tower, at a height you set, and you can turn it yourself. The simulator's own cameras switch from the map. Tugs drive in from their depot on the vehicle roads and drive back. The map shows ILS frequencies, the simulator's clock and a weather popup, and covers itself when the simulator is lost.

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
- Airport map: the tower can be placed on the map and its cab height set (Airport tab, saved per airport). With no aircraft selected, the tower camera stands at the tower and the Tower look buttons turn it (left, right, up, down, zoom; hold to keep turning); Swing makes it look round by itself.
- Airport map: the simulator's time in the status strip, UTC and local at your aircraft ("14:32Z · 16:32 LT", `ZULU TIME` and `LOCAL TIME`; `/api/aircraft` `zuluSec`, `localSec`).
- Pushback tugs come from their depot and go back. A tug appears at the vehicle parking spot nearest the stand, drives along the vehicle roads to the nose (the push waits for it), and after the push drives home and disappears there (`SimObjectTug.Layout`, `airport.Layout.VehicleRoute`, `VehicleDepots`). Airports without vehicle roads keep the old drive-off. LKPR's tower is known (`airport.Limits.Tower`); the facility puts it elsewhere.
- Airport map: the ILS of each runway end, with its frequency, loaded from the simulator. `airport.RunwayEnd.ILS` and `ILSRegion` come from the RUNWAY record, the frequency and name from the navaid record (LKPR: 24 PR 109.10). Shown in the Airport tab, and in `/api/airportinfo` as `ils`.

### Fixed

- Airport map: the status strip fits from 1024 to 1279 px wide (it ran off the edge). The theme switch, the weather chip, the help button and the position picker move into the More menu there, the picker as "Working as"; the menu's sim rate spans its width.
- Airport map: an arrival's sequence actions (direct, slow, hold, go-around) act on its own airport's sequence, not the one on the map.
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
