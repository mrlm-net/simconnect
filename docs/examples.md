---
title: "Examples"
description: "Start with the airport map, the SDK's main example and debugging tool, then find a small example for each part of the API."
order: 1
section: "examples"
---

# Examples

Every example is a standalone `main` package under [`examples/`](https://github.com/mrlm-net/simconnect/tree/main/examples). Run one from the repository root with `go run ./examples/<name>` while the simulator is running.

Start with the airport map. It uses most of the SDK at once, shows what the simulator reports, and is the tool the SDK itself is debugged with.

## The airport map

![The airport map at LKPR](images/airport-map/ui-traffic-light.png)

[`cmd/airport-map`](https://github.com/mrlm-net/simconnect/tree/main/cmd/airport-map) serves a live map of an airport on <http://127.0.0.1:8080>. It shows:

- **The ground layout** as SimConnect reports it: runways, taxi paths by `TYPE`, taxi points, hold-short points, taxiway names and parking stands. Every feature's popup shows its raw facility index and field values, so the taxi graph can be checked against the data ([Airport Layout](airport-layout.md)).
- **Taxi routing:** click a stand and pick a runway to see the departure route, its taxiways and runway crossings, or the taxi-in from a runway exit.
- **AI traffic under your control:** spawn departures and arrivals and give their clearances: pushback, taxi, progressive taxi, runway crossings, line-up, take-off ([Traffic Commands](traffic-commands.md)).
- **Scheduled traffic:** airlines fly a timetable at the airport by themselves, with departure and arrival boards, en route arrivals and overflights ([Traffic Schedules](traffic-schedules.md), [Traffic Manager](traffic-manager.md)).
- **The landing sequence and the tower:** landing order and spacing per runway, delays absorbed by speed, path stretching and holds, go-arounds, and a tower that clears line-up, take-off and crossings ([Airborne Separation](traffic-separation.md)).
- **Conflicts:** airborne pairs predicted, along their routes and the levels of their SIDs and STARs, to come within 5 NM (3 NM in the terminal area) and 1000 ft, and the change given to resolve each.
- **The ATC game:** work ground and tower yourself, scored ([ATC Game](atc-game.md)).

### Run it

From the repository root (the map is its own module: `-C` runs it from its folder):

```bash
# Live: connect to the simulator and open LKPR
go run -C cmd/airport-map .

# Another airport
go run -C cmd/airport-map . -icao LOWW

# Also save each fetched airport's raw data to <ICAO>.json
go run -C cmd/airport-map . -dump

# Offline: serve a saved dump, no simulator needed (layout and routes only)
go run -C cmd/airport-map . -file LKPR.json
```

The map builds against the library in the checkout through a Go workspace (`go.work`, not committed); create it once in the repository root:

```bash
go work init . ./cmd/airport-map ./cmd/simvar-cli
```

Open <http://127.0.0.1:8080/?icao=LKPR>. Click the airport at the top left and type another ICAO code to load it; **↻** fetches it again from the simulator.

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address |
| `-icao` | `LKPR` | Airport opened on start |
| `-dump` | `false` | Write each fetched airport's raw facility records to `<ICAO>.json` |
| `-dump-dir` | `.` | Directory for `-dump` files |
| `-file` | | Serve a `-dump` file instead of connecting to the simulator |
| `-log-dir` | `.` | Directory for the traffic control log, `traffic-<YYYYMMDD-HHMMSS>.log` |
| `-airways` | `pkg/nav/testdata/LKPR-airways.json` | Airway graph for flight plans (see `spike-airways`); `""` for direct routes |
| `-airspace` | `D` | Class of the managed airports' control zones for the VFR rules: `C`, `D`, `E` or `G` |
| `-piper` | `bin/piper/piper.exe` | Piper executable for the voice (see the map's README) |
| `-voices` | | Folder of piper voice models (`""`: voice-goio's user data folder) |
| `-token` | | Network play: the token another device needs to control the traffic (`auto`: a random one; `""`: none needed) |
| `-view-token` | | Network play: a token to watch only, as a spectator (`auto`: a random one) |
| `-pprof` | | Serve the Go profiler on this address (e.g. `127.0.0.1:6060`) |

The page loads Leaflet from cdnjs, the IBM Plex fonts from Google Fonts and map tiles from Esri and OpenStreetMap, so the browser needs internet access.

### A tour

The interface has three layers: a **status strip** across the top, the **selected aircraft** in its own panel, and the **sections** at the side (a bottom sheet on a phone). Light, dark or the system's theme is picked at the top right.

![The airport map at LKPR: the status strip, a departure selected, the traffic list](images/airport-map/ui-traffic.png)

The **status strip** shows:

- the airport and the runway in use;
- the ATIS letter, the wind and the QNH;
- Pause and the simulation rate;
- the game score;
- the camera;
- the frequency you listen to;
- the position this device works (**As**, for network play);
- the connection.

**The selected aircraft** (click it on the map or in a list) shows:

- its flight rules (IFR, or VFR for a circuit arrival), its state, and its scheduled time (STD or STA, amber past 5 minutes late, red past 15) or, without a schedule, how long it has waited;
- the next clearance as one big button. Pushback has its facing (N, E, S, W) and "with start-up" by it, on by default.
- the other clearances by phase;
- Hold position, Go around and Abort take-off, always in the same place;
- its route, procedure, frequency, motion and lights;
- its last calls on the radio;
- Show, Follow, Camera, Manual, Rush and Remove.

Click a point of its route on the map to clear it up to there. The first clearance you give an aircraft makes it **Manual**: from then on you give all its clearances. **Manual** off hands it back to ATC automation.

**Traffic** lists the aircraft waiting for you (oldest first), the ones moving, and those airborne or done. **New flight** spawns one aircraft:

1. Pick the kind, click a stand (or *Free stand*), and pick the runway with an entry or exit. The route is previewed on the map.
2. Optionally set a flight plan, the model and livery, the call sign, a tug, the procedures, a turnaround, de-icing, or a custom taxi route.
3. **Spawn** says what it will do.

The traffic log has every clearance as said.

**Sequence** has:

- the landing sequence per runway: call sign, wake category, distance to go, delay, and what the aircraft is doing;
- controls to move an arrival up or down, send it direct to the final, slow it, hold it or send it around;
- a ladder of the final, with the gaps green when kept and red when short;
- the tower (who is at each runway, and what they wait for) and the predicted conflicts.

![The landing sequence and the final ladder](images/airport-map/ui-sequence.png)

**Schedule** runs the timetable at the airport. It has the departure, arrival and overflight boards, and the [ATC game](atc-game.md).

**Radio** lists the airport's frequencies with the aircraft on each, and the conversation on the one followed. The voice can play:

- on the map's computer, on the output picked;
- on the device you are using (**Play on this device**).

**Follow my COM1** and **Tune my COM1** link the radio to your aircraft.

![The radio: frequencies and the conversation](images/airport-map/ui-radio.png)

**Airport** has:

- the runway in use, the weather and the ATIS (Listen reads it out);
- the SIDs, STARs and approaches of a runway, drawn on the map;
- the de-icing pads.

![Airport: runway in use, weather and ATIS](images/airport-map/ui-airport.png)

**Map** has:

- the base map;
- the layers: our aircraft, other traffic, safe zones, the final, taxiway names, overlapping and occupied stands, and taxi paths and points by `TYPE`;
- the traffic picture's centre and radius, and the world view.

On a phone the sections are a bottom sheet over the full-screen map:

![On a phone](images/airport-map/ui-phone.png)

The map's [README](https://github.com/mrlm-net/simconnect/tree/main/cmd/airport-map) covers network play (`-addr :8080`, one position per device) and lists the HTTP API, which scripts and tests can use too. The previous interface is at `/classic` while the new one settles.

## Other examples

Smaller examples, each showing one part of the API.

### Connection and lifecycle

| Example | What it shows | Docs |
|---------|---------------|------|
| [basic-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/basic-connection) | Connect, wait two seconds, disconnect | [Getting Started](getting-started.md) |
| [await-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/await-connection) | Retry until the simulator is up, shut down on Ctrl+C | [Client API](usage-client.md) |
| [lifecycle-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/lifecycle-connection) | Reconnect whenever the simulator quits and comes back | [Client API](usage-client.md) |
| [simconnect-manager](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-manager) | `pkg/manager`: auto-reconnect and connection state callbacks | [Manager Usage](usage-manager.md) |
| [simconnect-state](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-state) | Manager `SimState`: pause, sim running, camera state | [Manager Usage](usage-manager.md) |
| [simconnect-subscribe](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-subscribe) | Manager channel subscriptions for messages and connection state | [Manager Usage](usage-manager.md) |
| [simconnect-events](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-events) | Manager system events: flight and aircraft loaded, flight plan, objects added and removed, crash, sound, view, and a custom `1sec` event | [Event Lifecycle](events-lifecycle.md) |
| [simconnect-benchmark](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-benchmark) | Load test of the manager stack with CPU and memory profiles | |

### Data and events

| Example | What it shows | Docs |
|---------|---------------|------|
| [read-messages](https://github.com/mrlm-net/simconnect/tree/main/examples/read-messages) | Data definitions, periodic SimVar requests for the user aircraft and camera, and a one-shot request for the aircraft within 10 km | [Client API](usage-client.md) |
| [read-objects](https://github.com/mrlm-net/simconnect/tree/main/examples/read-objects) | `EnumerateSimObjectsAndLiveries`: the aircraft and liveries installed | [Client API](usage-client.md) |
| [set-variables](https://github.com/mrlm-net/simconnect/tree/main/examples/set-variables) | Write a SimVar: change `CAMERA STATE` | [Client API](usage-client.md) |
| [using-datasets](https://github.com/mrlm-net/simconnect/tree/main/examples/using-datasets) | The dataset registry, cloning, the builder and merging | [Dataset Composition](dataset-composition.md) |
| [emit-events](https://github.com/mrlm-net/simconnect/tree/main/examples/emit-events) | Map and transmit client events: toggle aircraft doors | [Client API](usage-client.md) |
| [subscribe-events](https://github.com/mrlm-net/simconnect/tree/main/examples/subscribe-events) | System event subscriptions: pause, sim start and stop, sound | [Event Lifecycle](events-lifecycle.md) |
| [flow-events](https://github.com/mrlm-net/simconnect/tree/main/examples/flow-events) | `SubscribeToFlowEvent` (MSFS 2024 only) | [Client API](usage-client.md) |
| [cmd/simvar-cli](https://github.com/mrlm-net/simconnect/tree/main/cmd/simvar-cli) | Get, set and watch SimVars, emit and listen for events, an interactive REPL (own `go.mod`) | [SimVar CLI](simvar-cli.md) |

### Facilities

| Example | What it shows | Docs |
|---------|---------------|------|
| [read-facility](https://github.com/mrlm-net/simconnect/tree/main/examples/read-facility) | Facility data definitions for one airport | [Facilities](guide-facilities.md) |
| [read-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/read-facilities) | Facility lists, paginated | [Facilities](guide-facilities.md) |
| [subscribe-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/subscribe-facilities) | Subscribe to the airport list and its updates | [Facilities](guide-facilities.md) |
| [all-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/all-facilities) | Request the full airport list at once | [Facilities](guide-facilities.md) |
| [airport-details](https://github.com/mrlm-net/simconnect/tree/main/examples/airport-details) | Parking, taxi paths, taxi points and taxiway names of one airport (EDDM), multi-packet responses | [Facilities](guide-facilities.md) |
| [locate-airport](https://github.com/mrlm-net/simconnect/tree/main/examples/locate-airport) | The nearest airport to the user aircraft | [Facilities](guide-facilities.md) |
| [read-waypoints](https://github.com/mrlm-net/simconnect/tree/main/examples/read-waypoints) | The waypoint list (`RequestFacilitiesList`, its count printed) and LKPR's airport facility data | [Facilities](guide-facilities.md) |
| [simconnect-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-facilities) | The ready-made facility datasets (airport, runways, parking, frequencies) with `RegisterFacilityDataset` | [Datasets](usage-datasets.md) |

### Traffic

| Example | What it shows | Docs |
|---------|---------------|------|
| [ai-taxi](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-taxi) | One departure: pushback, taxi, line-up and take-off with `pkg/traffic` | [Departure Taxi](traffic-taxi.md) |
| [ai-arrival](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-arrival) | One arrival: final, touchdown, runway exit and taxi-in to a stand | [Arrivals](traffic-arrival.md) |
| [ai-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-traffic) | Raw SimConnect: parked and en route ATC aircraft from `planes.json` (run it from its folder) | [Traffic Guide](traffic-guide.md) |
| [manage-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/manage-traffic) | Raw SimConnect: non-ATC aircraft parked at LKPR gates and one departure driven by waypoints from pushback through taxi to take-off (writes taxi CSV and GPX files) | [Traffic Guide](traffic-guide.md) |
| [monitor-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/monitor-traffic) | Every aircraft within 25 km, polled every 5 seconds | [Client API](usage-client.md) |
| [simconnect-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-traffic) | The manager's traffic fleet: a parked aircraft at LFPG and a non-ATC aircraft at LKPR on waypoints, removed on exit | [Traffic Guide](traffic-guide.md) |

### Navigation and weather

| Example | What it shows | Docs |
|---------|---------------|------|
| [atis](https://github.com/mrlm-net/simconnect/tree/main/examples/atis) | Weather at the user aircraft, the runway in use and the airport's ATIS | [Weather & ATIS](nav-weather.md) |
| [flight-plan](https://github.com/mrlm-net/simconnect/tree/main/examples/flight-plan) | An IFR plan between two airports over the airways, printed, and written as a `.pln` with `-out` | [Flight Plans](nav-flight-plans.md) |
| [spike-airways](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-airways) | Crawl the airway network around a centre and save it as JSON (the map's `-airways`) | [Airways](nav-airways.md) |

### Spikes

Experiments kept as a record of how the traffic features were found. Each answers one question about the simulator; they are not maintained as examples.

| Example | Question |
|---------|----------|
| [spike-airlines](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-airlines) | Can the airlines of a parking spot be read? |
| [spike-approach](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-approach) | Can a final approach, flare and rollout be flown by position injection? |
| [spike-camera](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-camera) | What does the MSFS 2024 add-on camera API report, and can it hold a camera behind and above the aircraft? |
| [spike-flare](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-flare) | Which flare profile gives a good touchdown? |
| [spike-gear](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-gear) | How is the gear of a non-ATC aircraft lowered? |
| [spike-geometry](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-geometry) | What does the simulator report of an aircraft's gear, span and CG? |
| [spike-inject](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-inject) | Can a frozen AI aircraft be moved along a taxi route by injection? |
| [spike-landing](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-landing) | Waypoints or ATC for landing, and a takeover on the ground |
| [spike-lights](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-lights) | How are the lights of a non-ATC aircraft switched? |
| [spike-lighttiming](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-lighttiming) | When does MSFS AI switch the lights of a taxiing aircraft? |
| [spike-procedures](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-procedures) | The record layout of SIDs, STARs and approaches |
| [spike-profile](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-profile) | Which type SimVars do AI aircraft report reliably? |
| [spike-speed](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-speed) | How can an AI approach be flown slower? |
| [spike-tug](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-tug) | Which ground vehicles are pushback tugs? |
| [spike-tugwatch](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-tugwatch) | Where is an injected tug relative to its aircraft? |
