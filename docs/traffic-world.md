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

On a host's connection: the host feeds every message of its connection and runs the World on its client. `Feed` never blocks; with its queue (4096 messages) full, a message is dropped and counted in `Snapshot().Dropped`.

```go
w := world.New(world.Options{OnTransmission: say})
mgr.OnMessage(func(m engine.Message) { w.Feed(m) })
go w.RunOn(ctx, client) // all its SimConnect calls happen here; again on the next connection
```

## Options and hooks

| Option | What |
|---|---|
| `LogDir` | where the traffic log goes (`traffic-*.log`); "" none |
| `Airways` | the airway graph for flight plans; nil: direct routes |
| `Airspace` | the control zone class for VFR rules (default D) |
| `DataDir` | de-icing pads (`deicing.json`) and review overlays |
| `DumpDir` | write each fetched airport's raw facility records |
| `IDBase` | moves the library helpers it creates off their default IDs (see SimConnect IDs) |
| `Scenes` | a directory of camera scenes; "" the built-in ones |
| `OnTransmission` | every transmission once logged: the host says it with its own voice |
| `OnChange` | a part of the picture changed (`control`, `radio`): fetch it again |
| `OnCom1` | the user aircraft's COM1 frequency, each second |
| `OnTune` | the COM1 tuner of each connection (nil once it ends) |
| `SceneFrequency` | the frequency a camera scene's radio is on |

A transmission (`traffic.Transmission`) has everything a voice needs: the text, the intent and its parameters, the call sign, whether a pilot or which position says it, the airport and the frequency.

## What a host sees and asks

- `Snapshot()`: our aircraft (`ControlView`: state, ATC position and frequency, routes still to fly, the clearances available now) with their ground vehicles (`VehicleView`: tug or fuel truck, its sim object id, model, state, position and the way still ahead), whether the traffic runs, and how many fed messages were dropped. A vehicle's state is what it says of itself (`traffic.VehicleState`: waiting, inbound, attached, fuelling, outbound, removed).
- `Do(method, path, body)` and `Get(path, &v)`: the HTTP API in process, the same calls a remote client makes. For example, `Get("/api/airportinfo?icao=LKPR", &v)` gives the runways in use, the ATIS (letter and text: the World owns it), the ILS and the weather. `/api/sequence?icao=` gives the landing sequences, `/api/stands?icao=` the stands, and `POST /api/schedule {"enabled":true,"icao":"LKPR","density":1}` starts the schedule. `POST /api/control/{id}/{action}` gives a clearance.
- Typed actions over the same API: `SetSchedule(ScheduleSettings{Enabled, ICAO, Airports, Density, IFR, VFR, Generator, Others})`, `AddFlights(flights)`, `Clear(id, action)` and `Approach(icao, callsign, action)`.
- `Register(mux)`: serve that API on the host's own server (the airport map does).

## Flights at a chosen time

A host can time traffic around its own flight (#737, #738): an arrival a few minutes before the player's ETA, a departure just after the player's off-block.

- `SetSchedule(ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Generator: &off})` runs the scheduled airports with no generated timetable: only the flights added.
- `AddFlights([]traffic.Flight{...})` (`POST /api/flights`) adds flights. Each needs a call sign, an origin and destination (one of them a scheduled airport), the STD and STA in traffic time (`GET /api/schedule`'s `now`), and an airline or type (default A320). The manager spawns them as it spawns the timetable's: a departure on its stand `DepartureLead` before its STD, an arrival `ArrivalLead` before its STA to fly the STAR and approach.
- An arrival added later than `ArrivalLead` minus `ArrivalLate` before its STA (15 min with the defaults) is refused with 422, not cancelled later. `GET /api/flights` lists the manager's flights with their status.
- `Options.Schedule` (`ScheduleTiming`, #741) sets the horizon, the leads and the late limits; zero values keep the defaults (2 h; 10, 25 and 8 min; 15 and 10 min).

## Traffic along the user's route

In cruise, the World can keep a few airliners around the host's flight (#740). `SetCorridor(CorridorSettings{...})` (`POST /api/corridor`) takes the user's route ahead (two or more points, in its direction), its cruise level and speed, and how many of each kind (default one):

- **same:** ahead on the route, 25 to 45 NM, going the same way 2000 ft above or below.
- **opposite:** 70 to 100 NM ahead, coming the other way 1000 ft above or below.
- **crossing:** across the route 45 to 70 NM ahead, at 60 to 120 degrees, 1000 or 2000 ft above or below.

They are airlines of the schedule, with a jet that cruises at that level, created airborne and flown by MSFS AI (`traffic.CorridorRoute`, the en-route machinery). Each is kept at least 15 s from the last; none appears on top of other traffic. One more than `DespawnNM` (default 80) from the user aircraft and moving away is taken out and replaced. `GET /api/corridor` shows the settings and the aircraft; `"enabled": false` takes them all out. The user aircraft's position comes from the World's feed.

## Beside the host's own ATC

The World never controls nor calls the user aircraft. A host whose own ATC works the player tells the World what it does:

- `Heard(t)`: the host's ATC said `t` on `t.Frequency` at `t.Airport`. The World's traffic waits for the frequency instead of talking over it.
- `ClearPlayer(world.PlayerClearance{ICAO, Runway, Phase})`, where the phase is `lineup`, `takeoff`, `landing` or `vacated`. While the player lines up, takes off or lands on a runway, none of the World's traffic is cleared onto it (line up, take-off, landing, crossing). Landing, the player is in that runway's landing sequence (as `Callsign`, else "Player"), so the traffic fits around it; `Snapshot().Player` is its place (number, the call sign and type it follows, the spacing and both distances to go). `vacated` ends it.

## SimConnect IDs

The World uses these definition, request and event IDs on the connection; a host keeps its own clear of them. A host that uses the same library helpers on its connection moves the World's off their defaults with `Options.IDBase`: airport loader at IDBase/+100, procedure loader +200/+300, nav loader +400/+500, airport list +600, injector +700/+800/+900, airway crawl +1000/+1010.

| IDs | What |
|---|---|
| 2000–2021 | user aircraft, traffic scan, model and vehicle lists, sim events, camera state |
| 7100–7999 | library defaults: airport loader 7100/7200, taxi 7300/7400, arrivals 7500/7600, injector 7700–7999 |
| 8200–8999 | stand allocators 8200/8300 (+4 per airport), procedures 8400/8500, airway crawl 8600/8610–8625, nav loader 8700/8800, airport list 8900 |
| 10010–10011 | weather at the user aircraft |
| 20000–21279, 30000–31279 | the controllers' ID blocks (128 × 10) |
| 41000–41999 | enroute traffic |

## Split: a director anywhere, an actuator beside the simulator

The World can run in two parts (#710). The **actuator** runs beside Microsoft Flight Simulator. It keeps the SimConnect connection, each aircraft's controller and their injection at frame rate. The **director** takes every decision (schedule, ATC, sequencing, separation, conflicts) and needs no simulator, so it can run on Linux. Decisions cross the network about once a second; injection never does.

```sh
traffic-actuator -listen :7710 -token s3cret                      # on the simulator's PC (Windows)
traffic-director -actuator simpc:7710 -token s3cret -addr :8080 \
                 -web cmd/airport-map/web -airways airways.json   # anywhere; the map's page on :8080
```

The link is JSON lines over TCP, and the director opens it with the token. If the director goes away, the actuator keeps flying, and the next director to connect takes over. In a program, `world.ServeActuator(ctx, w, addr, token)` and `world.DialDirector(ctx, w, addr, token)` do the same. `world.Loopback(ctx, actuator, director)` links the two parts in one process (the airport map's `-split`) to check the split against the World in one piece.

Not yet in the split: fuel trucks, and the tug and fuel-truck routes on the director's map. Each read of an aircraft's controller is a call across the network, which suits a LAN better than the internet.

`ScheduleSettings.OffsetMin` (`"offsetMin"`) flies the airline timetable of that many minutes later now (#738): `600` puts a morning wave into an evening. VFR flights keep the daylight of now.

### Multiplayer: local first, one director for several sims

In multiplayer each player's sim is an actuator; the director (on a server) decides for all of them (#774, #779). Motion stays local: every aircraft and vehicle is moved and injected by the actuator on the player's PC at the sim's frame rate. Only decisions cross the network.

- **On the player's PC, with the host's own connection:** `w.LinkDirector(ctx, "director:7710", token)` before `RunOn`. It dials out (no way in needed), dials again 5 s after the director is lost, and keeps the link across sim reconnects (`RunOn` restarted per connection). `Snapshot().Link` is "dialling", "attached" or "gone". `DialActuator` does the same with a connection of its own (`traffic-actuator -director`).
- **On the server:** `ListenDirector(ctx, w, ":7710", token)` (`traffic-director -listen :7710`). Its HTTP API wants a token on a public server: `-api-token` to control the traffic, `-view-token` to read it ("auto": a random one, printed), sent as `Authorization: Bearer <token>`; the server itself always has access. The first actuator to dial in is the primary: its replies, events and feed drive the director. Later ones follow: they get the same commands, so each sim creates and moves the same traffic; what they send back is dropped. A follower joining late gets the flights started after it; a model a follower does not have is not created there. When the primary is gone the director starts again with the next one.
- **Radio:** every transmission is relayed to the actuators and reaches their host's `OnTransmission`, so each player's voice speaks it locally.
- **What the host does on the director, not locally:** in multiplayer the schedule, flights, corridor and player clearances (`SetSchedule`, `AddFlights`, `SetCorridor`, `ClearPlayer`) go to the director's HTTP API. The actuator's own `Snapshot().Aircraft` is empty: the aircraft are listed by the director (`GET /api/control`).
- **Session tokens and TLS (#792):** on a public server the link carries a player's position and the credential that drives their sim, so it runs over TLS with per-player session tokens. Server: `ListenDirectorWith(ctx, w, ":7710", world.LinkOptions{Verify: world.NewJWKS(jwksURL, iss, aud).LinkVerify, TLS: serverTLS})` (`traffic-director -listen :7710 -jwks <url> -tls-cert <pem> -tls-key <pem>`, optional `-jwks-iss`/`-jwks-aud`). Each greeting's token is verified against the keys published at the JWKS address (RS256/384/512, PS256, ES256/384, EdDSA; exp and nbf with a minute of leeway); the link closes when the token expires. The same tokens open the HTTP API (`-jwks-api control|view|none`, default control). Player: `w.LinkDirectorWith(ctx, "director:7710", world.LinkOptions{TokenFunc: session, TLS: &tls.Config{}})` (an empty config verifies the director with the system roots). `TokenFunc` is called for every greeting, so a link closed on expiry comes back with a fresh token on its own. `traffic-actuator -director host:7710 -tls` (`-tls-ca` for a private CA). The `token` forms stay for a trusted network.
