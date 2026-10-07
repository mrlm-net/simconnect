---
title: "Traffic World"
description: "The airport map's traffic engine as a package: scheduled traffic with its ATC, run on its own simulator connection or a host's, with its HTTP API, a snapshot and hooks for the host's voice."
order: 20
section: "traffic"
---

# Traffic World

`pkg/traffic/world` is the traffic engine of the [airport map](https://github.com/mrlm-net/simconnect/tree/main/cmd/airport-map) as a package (#710). It runs scheduled traffic around the focus airports with its ATC: spawning and pushback, taxi, runways and their clearances, landing sequences, separation and conflicts, holds, approaches and VFR circuits, enroute traffic, crews, fuel trucks and de-icing, and the phrases per position. The airport map is a front end over it: its page and its voice.

The package uses only the standard library and this module. It is Windows-only, like the rest of the simulator side.

## Running it

On its own connection (the airport map does this; it reconnects when the simulator goes away):

```go
w := world.New(world.Options{LogDir: ".", Airways: graph})
go w.Run(ctx)
```

On a host's connection: the host feeds every message of its connection and runs the World on its client. `Feed` copies the message and never blocks; with its queue (`Options.QueueSize`, default `DefaultQueueSize` 4096 messages) full, a message is dropped and counted in `Snapshot().Dropped`.

```go
w := world.New(world.Options{OnTransmission: say})
mgr.OnMessage(func(m engine.Message) { w.Feed(m) })
go w.RunOn(ctx, client) // all its SimConnect calls happen here; again on the next connection
```

## Options and hooks

| Option | What |
|---|---|
| `LogDir` | where the traffic log goes (`traffic-*.log`); "" none |
| `Airways` | an airway graph for flight plans; the airways around every airport loaded are read from the sim and added (#799) |
| `AirwaysMaxAge` | how long an airport's airways read from the sim are kept in `DataDir/airways` before they are read again (default `DefaultAirwaysMaxAge`, 7 days) |
| `Airspace` | the control zone class for VFR rules (default D) |
| `DataDir` | de-icing pads (`deicing.json`), review overlays and the airways read from the sim |
| `DumpDir` | write each fetched airport's raw facility records |
| `IDBase` | moves the library helpers it creates off their default IDs (see SimConnect IDs) |
| `QueueSize` | how many fed messages wait for the World (default 4096) |
| `Output` | where the console lines go (nil: stdout); a host speaking a protocol on stdout gives stderr or `io.Discard` |
| `Scenes` | a directory of camera scenes; "" the built-in ones |
| `Schedule` | the schedule's timing (`ScheduleTiming`, see below) |
| `OnTransmission` | every transmission once logged: the host says it with its own voice |
| `OnChange` | a part of the picture changed (`control`, `radio`): fetch it again |
| `OnCom1` | the user aircraft's COM1 frequency, each second |
| `OnTune` | the COM1 tuner of each connection (nil once it ends) |
| `SceneFrequency` | the frequency a camera scene's radio is on |

A transmission (`traffic.Transmission`) has everything a voice needs: the text, the intent and its parameters, the call sign, whether a pilot or which position says it, the airport and the frequency.

## What a host sees and asks

- `Snapshot()`: our aircraft (`ControlView`: state, ATC position and frequency, routes still to fly, the clearances available now) with their ground vehicles (`VehicleView`: tug or fuel truck, its sim object id, model, state, position and the way still ahead), whether the traffic runs, and how many fed messages were dropped. A vehicle's state is what it says of itself (`traffic.VehicleState`: waiting, inbound, attached, fuelling, outbound, removed).
- `Do(method, path, body)` and `Get(path, &v)`: the HTTP API in process, the same calls a remote client makes. For example, `Get("/api/airportinfo?icao=LKPR", &v)` gives the runways in use, the ATIS (letter and text: the World owns it), the ILS and the weather. `/api/sequence?icao=` gives the landing sequences, `/api/stands?icao=` the stands, and `POST /api/schedule {"enabled":true,"icao":"LKPR","density":1}` starts the schedule. `POST /api/control/{id}/{action}` gives a clearance. `GET /api/traffic` lists every aircraft the sim reports around the user aircraft (`Traffic`; ours flown by MSFS AI carry `route`, the way they still fly).
- Typed actions over the same API: `SetSchedule(ScheduleSettings{Enabled, ICAO, Airports, Density, IFR, VFR, Generator, Others})`, `AddFlights(flights)`, `Clear(id, action)` and `Approach(icao, callsign, action)`.
- `Register(mux)`: serve that API on the host's own server (the airport map does).

## Flights at a chosen time

A host can time traffic around its own flight (#737, #738): an arrival a few minutes before the player's ETA, a departure just after the player's off-block.

- `SetSchedule(ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Generator: &off})` runs the scheduled airports with no generated timetable: only the flights added.
- `AddFlights([]traffic.Flight{...})` (`POST /api/flights`) adds flights. Each needs a call sign, an origin and destination (one of them a scheduled airport), the STD and STA in traffic time (`GET /api/schedule`'s `now`), and an airline or type (default A320). The manager spawns them as it spawns the timetable's: a departure on its stand `DepartureLead` before its STD, an arrival `ArrivalLead` before its STA to fly the STAR and approach.
- An arrival added later than `ArrivalLead` minus `ArrivalLate` before its STA (15 min with the defaults) is refused with 422, not cancelled later. `GET /api/flights` lists the manager's flights with their status.
- `Options.Schedule` (`ScheduleTiming`, #741) sets the horizon, the leads and the late limits; zero values keep the defaults (2 h; 10, 25 and 8 min; 15 and 10 min).

## Real-world traffic (v0.22)

With real-world traffic on, the World flies the aircraft that a feed such as ADS-B observes, instead of the generated timetable (#841). Each aircraft gets everything the World's own traffic gets: stand services, push, taxi, ATC and radio.

- `SetRealTraffic(true, "LKPR")` (`POST /api/realtraffic {"on":true,"icao":"LKPR"}`) turns the generator off and sets the managed airport. Generated flights not yet in the simulator go at once, and those flying finish their flight. `false` brings the generator back. `GET /api/realtraffic` says whether it is on, with the managed airports.
- `Observe([]traffic.Observed)` (`POST /api/realflights`) takes a feed's snapshot. A sighting is an `id` (the ICAO 24-bit address), `callsign`, `registration`, `type`, `lat`/`lon`, `altFt`, `groundKts`, `trackDeg`, `vsFpm`, `onGround` and `seenAt`, plus optional `kind`, `origin`, `destination` and `departAt`.
  - It returns one `ObserveResult` per sighting, with the status `added`, `updated`, `retimed`, `turnaround` or `ignored` (with a reason).
  - The aircraft flies under its call sign, else its registration, else its ID.
  - An unknown origin or destination stays `""` and is shown as unknown, never guessed.
- `Drop(id)` (`DELETE /api/realflights/{id}`) ends an aircraft the feed no longer sees. One not spawned yet, waiting on its stand or parked goes at once. One in progress plays out first: an arrival lands and parks, then goes; a departure leaves. Nothing is ever taken off the final.

The kind, when not given, comes from `traffic.ClassifyObserved`:

| Kind | When (no `kind` given) | What the World does |
|---|---|---|
| parked | On the ground within 3 NM of the airport, still | It goes on the free stand within 80 m of where it is seen (else a stand by its type) and waits, with no push and no call to delivery, until a departure is seen for its ID. That departure re-times its push (`traffic.TaxiController.SetPushbackAt`), and only then does the aircraft call delivery. |
| departure | On the ground at the airport, moving | It goes on a stand the same way and pushes at `departAt` or now. With no destination it flies a SID of the runway and leaves the area. |
| arrival | Airborne within 150 NM, heading for the airport (within 60°), not climbing away | Its sighting is projected to now along its track (at most 10 min). It appears there, flown by MSFS AI, and joins a STAR of the runway in use at the point that gives the shortest way in, up to the initial approach fix. Approach takes it over at that point, as it takes over an en-route arrival. It is never held back by the landing flow (it is already in the air) and never cancelled by time. |
| overflight | Anything else airborne | Not flown yet. |

A sighting is `ignored` when it is on the ground away from the airport, climbing out (over 500 fpm within 30 NM), or a ground station or vehicle (type `TWR`, `GND`, `GRND`, `SVC`, `VEH`). The projection uses its vertical rate for at most a minute.

Each ID is one aircraft and is never spawned twice. Once the World flies an ID, later sightings don't move it; they only refresh its registration, origin and destination.

Each kind of flight shows as real:
- `ControlView` has `real`, `observedId` and `registration`.
- The manager's flights carry their `observed` sighting.

A parked aircraft keeps the call sign it was spawned with when its departure is seen under another.

## TCAS (v0.24)

Each of the World's airborne aircraft carries TCAS II (#450). It watches every aircraft within 12 NM: the World's own, the user's and other traffic. When the test is met it gives a traffic advisory (TA) or a resolution advisory (RA): `traffic.Evaluate`, then `traffic.SelectRA`.

The thresholds come from the FAA's *Introduction to TCAS II Version 7.1* (2011), Table 2:

| Own altitude | SL | TA tau (s) | RA tau (s) | TA DMOD (NM) | RA DMOD (NM) | TA ZTHR (ft) | RA ZTHR (ft) | ALIM (ft) |
|---|---|---|---|---|---|---|---|---|
| < 1000 ft AGL | 2 | 20 | — | 0.30 | — | 850 | — | — |
| 1000–2350 ft AGL | 3 | 25 | 15 | 0.33 | 0.20 | 850 | 600 | 300 |
| 2350–5000 ft | 4 | 30 | 20 | 0.48 | 0.35 | 850 | 600 | 300 |
| 5000–10000 ft | 5 | 40 | 25 | 0.75 | 0.55 | 850 | 600 | 350 |
| 10000–20000 ft | 6 | 45 | 30 | 1.00 | 0.80 | 850 | 600 | 400 |
| 20000–42000 ft | 7 | 48 | 35 | 1.30 | 1.10 | 850 | 700 | 600 |
| > 42000 ft | 7 | 48 | 35 | 1.30 | 1.10 | 1200 | 800 | 700 |

How an advisory is decided and flown:
- **Advisory:** given when both the range test and the vertical test pass. The range test is the range tau, modified towards DMOD at slow closure. The vertical test is the vertical tau, or within ZTHR.
- **Inhibits:** no RA below 1000 ft AGL, no descend RA below 1100 ft AGL, nothing on the ground.
- **Sense:** the non-crossing sense when it gives ALIM at the closest point, otherwise the one that gives the most separation. The pilot is modelled as responding in 5 s at 0.25 g to 1500 fpm.
- **Strength:** the least disruptive RA. *Monitor Vertical Speed* when the current rate already gives ALIM; *Level Off* when stopping the climb or descent does; otherwise *Climb* or *Descend*, crossing when it passes through the intruder's level.
- **Coordination:** between two of the World's aircraft the second takes the opposite sense to the first.

The crew flies the RA after 5 s:
- **Level off:** holds its level.
- **Climb or descend:** goes 1000 ft from where the RA began, as a level on its route. An arrival on its injected final takes a climb RA as a go-around.

The crew reports on its frequency: "(station), (callsign), TCAS RA", then "clear of conflict, returning to assigned altitude" (FAA JO 7110.65 2-1-28, its examples). Approach gives the aircraft no instruction until then, and the conflict resolver leaves it alone.

On the map and over the API:
- `ControlView.tcas` holds the advisory (`TA`/`RA`), the intruder, the aural (Table 4, Version 7.1) and the sense. The map shows a TA in amber and an RA in red with ↑ or ↓.
- `GET /api/tcas` returns the counts and the last 200 events (TA, RA, clear).

## Traffic along the user's route

In cruise, the World can keep a few airliners around the host's flight (#740). `SetCorridor(CorridorSettings{...})` (`POST /api/corridor`) takes the user's route ahead (two or more points, in its direction), its cruise level and speed, and how many of each kind (default one):

- **same:** ahead on the route, 25 to 45 NM, going the same way 2000 ft above or below.
- **opposite:** 70 to 100 NM ahead, coming the other way 1000 ft above or below.
- **crossing:** across the route 45 to 70 NM ahead, at 60 to 120 degrees, 1000 or 2000 ft above or below.

They are airlines of the schedule, with a jet that cruises at that level, created airborne and flown by MSFS AI (`traffic.CorridorRoute`, the en-route machinery). Each is kept at least 15 s from the last; none appears on top of other traffic. One more than `DespawnNM` (default 80) from the user aircraft and moving away is taken out and replaced. `GET /api/corridor` shows the settings and the aircraft; `"enabled": false` takes them all out. The user aircraft's position comes from the World's feed.

## Beside the host's own ATC

The World never controls nor calls the user aircraft. A host whose own ATC works the player tells the World what it does:

- `Heard(t)`: the host's ATC said `t` on `t.Frequency` at `t.Airport`. The World's traffic waits for the frequency instead of talking over it.
- `ClearPlayer(world.PlayerClearance{ICAO, Runway, Phase})` (`POST /api/player`), where the phase is `pushback`, `taxi`, `holding_short`, `lineup`, `takeoff`, `landing` or `vacated`. Pushing back, ours on stands near it wait to push; holding short, it takes its place in the departure queue. While the player lines up, takes off or lands on a runway, none of the World's traffic is cleared onto it (line up, take-off, landing, crossing). Landing, the player is in that runway's landing sequence (as `Callsign`, else "Player"), so the traffic fits around it; `Snapshot().Player` is its place (number, the call sign and type it follows, the spacing and both distances to go). `vacated` ends it.

## Ground services (v0.20)

Each airport has a fleet of service vehicles. The defaults are sized by its stands:

| Vehicle | Default size | Model (GSX first, then MSFS's own) | Where it parks | When it comes |
|---|---|---|---|---|
| Pushback tug | 1 per 10 stands, at least 2 | GSX tug | Nose gear | Before the push |
| Fuel truck | 1 per 15 stands, at least 1 | GSX fuel truck or hydrant dispenser | Right wing | While the departure waits |
| Boarding stairs | 1 per 10 stands, at least 2 | `FSDT_Staircase_*`, else `ASO_Boarding_Stairs` | Front left door, square to the fuselage; backs straight out | Remote stands only (no jetway) |
| GPU | 1 per 10 stands, at least 2 | `FSDT_GPU_TLD_406` or `FSDT_GPU_Hobart_4400`, else `Car Ground Power Unit` | Forward right of the nose gear | Remote stands only |

- **Overrides:** `airport.Limits` sets `Tugs`, `FuelTrucks`, `Stairs` and `GPUs` per airport, and a local override file wins per value.
- **Taking and giving back:** a departure takes each vehicle from the fleet before it is sent, and gives it back once the vehicle has driven off, or when the flight ends or is cancelled.
- **When none is free:**
  - The push waits for a tug.
  - Fuelling, stairs and the GPU are left out once it is too late for them. Stairs and the GPU leave 2 minutes before the tug comes, and the push waits for them to clear.
- **Log:** the traffic log gives each airport's fleet sizes, and logs a departure waiting for one ("waits for a tug at LKPR: all 8 busy").
- **Library:** `traffic.VehicleFleet` and `traffic.ServiceFleet`; `TaxiRequest.Stairs` and `TaxiRequest.GPU`.
- **Control API:** `pushInMin` gives a manual departure a push time, so its stand services have a window.

## SimConnect IDs

The World uses these definition, request and event IDs on the connection; a host keeps its own clear of them. A host that uses the same library helpers on its connection moves the World's off their defaults with `Options.IDBase`: airport loader at IDBase/+100, procedure loader +200/+300, nav loader +400/+500, airport list +600, injector +700/+800/+900, airway crawl +1000/+1010, enroute creations +1100–+2099. In each controller block of 10 request IDs the last four are its GPU, stairs, fuel truck and tug.

| IDs | What |
|---|---|
| 2000–2021 | user aircraft, traffic scan, model and vehicle lists, sim events, camera state |
| 7100–7999 | library defaults: airport loader 7100/7200, taxi 7300/7400, arrivals 7500/7600, injector 7700–7999 |
| 8200–8999 | stand allocators 8200/8300 (+4 per airport), procedures 8400/8500, airway crawl 8600/8610–8625, nav loader 8700/8800, airport list 8900 |
| 10010–10011 | weather at the user aircraft |
| 20000–21279, 30000–31279 | the controllers' ID blocks (128 × 10) |
| 41000–41999 | enroute traffic (IDBase +1100–+2099 when set) |

## Split: a director anywhere, an actuator beside the simulator

The World can run in two parts (#710). The **actuator** runs beside Microsoft Flight Simulator. It keeps the SimConnect connection, each aircraft's controller and their injection at frame rate. The **director** takes every decision (schedule, ATC, sequencing, separation, conflicts) and needs no simulator, so it can run on Linux. Decisions cross the network about once a second; injection never does.

```sh
traffic-actuator -listen :7710 -token s3cret                      # on the simulator's PC (Windows)
traffic-director -actuator simpc:7710 -token s3cret -addr :8080 \
                 -web cmd/airport-map/web -airways airways.json   # anywhere; the map's page on :8080
```

The link is JSON lines over TCP, and the director opens it with the token. If the director goes away, the actuator keeps flying, and the next director to connect takes over. In a program, `world.ServeActuator(ctx, w, addr, token)` and `world.DialDirector(ctx, w, addr, token)` do the same. `world.Loopback(ctx, actuator, director)` links the two parts in one process (the airport map's `-split`) to check the split against the World in one piece.

The actuator sends the departures' tugs and fuel trucks (with their routes) for the director's map each second. It also sends each controller's state each second, and the director answers its plain reads (state, route, plan, hold) from that; a command to a controller drops its snapshot, and reads with arguments are still a call across the network.

`ScheduleSettings.OffsetMin` (`"offsetMin"`) flies the airline timetable of that many minutes later now (#738): `600` puts a morning wave into an evening. VFR flights keep the daylight of now.

### Multiplayer: local first, one director for several sims

In multiplayer each player's sim is an actuator; the director (on a server) decides for all of them (#774, #779). Motion stays local: every aircraft and vehicle is moved and injected by the actuator on the player's PC at the sim's frame rate. Only decisions cross the network.

- **On the player's PC, with the host's own connection:** `w.LinkDirector(ctx, "director:7710", token)` before `RunOn`. It dials out (no way in needed), dials again 5 s after the director is lost, and keeps the link across sim reconnects (`RunOn` restarted per connection). `Snapshot().Link` is "dialling", "attached" or "gone". `DialActuator` does the same with a connection of its own (`traffic-actuator -director`).
- **On the server:** `ListenDirector(ctx, w, ":7710", token)` (`traffic-director -listen :7710`). Its HTTP API wants a token on a public server: `-api-token` to control the traffic, `-view-token` to read it ("auto": a random one, printed), sent as `Authorization: Bearer <token>`; the server itself always has access. The first actuator to dial in is the primary: its replies, events and feed drive the director. Later ones follow: they get the same commands, so each sim creates and moves the same traffic; what they send back is dropped. A follower joining late gets the flights started after it; a model a follower does not have is not created there. When the primary is gone the director starts again with the next one.
- **Radio:** every transmission is relayed to the actuators and reaches their host's `OnTransmission`, so each player's voice speaks it locally.
- **What the host does on the director, not locally:** in multiplayer the schedule, flights, corridor and player clearances (`SetSchedule`, `AddFlights`, `SetCorridor`, `ClearPlayer`) go to the director's HTTP API. The actuator's own `Snapshot().Aircraft` is empty: the aircraft are listed by the director (`GET /api/control`).
- **Session tokens and TLS (#792):** on a public server the link carries a player's position and the credential that drives their sim, so it runs over TLS with per-player session tokens. Server: `ListenDirectorWith(ctx, w, ":7710", world.LinkOptions{Verify: world.NewJWKS(jwksURL, iss, aud).LinkVerify, TLS: serverTLS})` (`traffic-director -listen :7710 -jwks <url> -tls-cert <pem> -tls-key <pem>`, optional `-jwks-iss`/`-jwks-aud`). Each greeting's token is verified against the keys published at the JWKS address (RS256/384/512, PS256, ES256/384, EdDSA; exp and nbf with a minute of leeway); the link closes when the token expires. The same tokens open the HTTP API (`-jwks-api control|view|none`, default control). Player: `w.LinkDirectorWith(ctx, "director:7710", world.LinkOptions{TokenFunc: session, TLS: &tls.Config{}})` (an empty config verifies the director with the system roots). `TokenFunc` is called for every greeting, so a link closed on expiry comes back with a fresh token on its own. `traffic-actuator -director host:7710 -tls` (`-tls-ca` for a private CA). The `token` forms stay for a trusted network.
