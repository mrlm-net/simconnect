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

![The airport map at LKPR: the ground layout, scheduled traffic and the Traffic tab](images/airport-map/traffic.jpg)

[`examples/airport-map`](https://github.com/mrlm-net/simconnect/tree/main/examples/airport-map) serves a live map of an airport on <http://127.0.0.1:8080>. It shows:

- **The ground layout** as SimConnect reports it: runways, taxi paths by `TYPE`, taxi points, hold-short points, taxiway names and parking stands. Every feature's popup shows its raw facility index and field values, so the taxi graph can be checked against the data ([Airport Layout](airport-layout.md)).
- **Taxi routing:** click a stand and pick a runway to see the departure route, its taxiways and runway crossings, or the taxi-in from a runway exit.
- **AI traffic under your control:** spawn departures and arrivals and give their clearances: pushback, taxi, progressive taxi, runway crossings, line-up, take-off ([Traffic Commands](traffic-commands.md)).
- **Scheduled traffic:** airlines fly a timetable at the airport by themselves, with departure and arrival boards, en route arrivals and overflights ([Traffic Schedules](traffic-schedules.md), [Traffic Manager](traffic-manager.md)).
- **The landing sequence and the tower:** landing order and spacing per runway, delays absorbed by speed, path stretching and holds, go-arounds, and a tower that clears line-up, take-off and crossings ([Airborne Separation](traffic-separation.md)).
- **Conflicts:** airborne pairs predicted to come within 5 NM and 1000 ft, and the change given to resolve each.
- **The ATC game:** work ground and tower yourself, scored ([ATC Game](atc-game.md)).

### Run it

```bash
# Live: connect to the simulator and open LKPR
cd examples/airport-map && go run .

# Another airport
cd examples/airport-map && go run . -icao LOWW

# Also save each fetched airport's raw data to <ICAO>.json
cd examples/airport-map && go run . -dump

# Offline: serve a saved dump, no simulator needed (layout and routes only)
cd examples/airport-map && go run . -file LKPR.json
```

Open <http://127.0.0.1:8080/?icao=LKPR>. Type another ICAO code in the side panel to load it; **↻** fetches it again from the simulator.

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address |
| `-icao` | `LKPR` | Airport opened on start |
| `-dump` | `false` | Write each fetched airport's raw facility records to `<ICAO>.json` |
| `-dump-dir` | `.` | Directory for `-dump` files |
| `-file` | | Serve a `-dump` file instead of connecting to the simulator |
| `-log-dir` | `.` | Directory for the traffic control log, `traffic-<YYYYMMDD-HHMMSS>.log` |
| `-airways` | `pkg/nav/testdata/LKPR-airways.json` | Airway graph for flight plans (see `spike-airways`); `""` for direct routes |

The page loads Leaflet from cdnjs and map tiles from OpenStreetMap and Esri, so the browser needs internet access.

### A tour

The side panel has one tab per task. The map buttons on the left: **✈** shows your aircraft, **⛶** full screen, **◨** hides the panel, **🌐** the world view.

**Traffic** is where traffic is made and controlled.

- **▶ Start game** starts the [ATC game](atc-game.md).
- **Scheduled traffic ▶ Start** runs the timetable at the loaded airport (or several), with a density and a maximum number of aircraft. **Departures**, **Arrivals** and **Overflights** are the boards: STD/STA, estimates, status and why.
- **New flight** spawns one aircraft: pick the kind, click a stand (or tick *free stand*), pick the runway and an entry or exit; the route is previewed on the map and **▶ Spawn** says what it will do. Options: hold at every clearance, pushback tug, injected approach, turnaround, SIDs and STARs, a flight plan to or from another airport, de-icing, a custom taxi route.
- **Aircraft** has a card per aircraft with its state and the clearances available now. Select a card to draw its route; click a point of the route to clear it up to there. **Traffic log** shows every clearance as ATC says it.

![A selected aircraft's taxi route, drawn from its stand to the runway](images/airport-map/taxi-route.jpg)

**Approach** shows the landing sequence per runway: call sign, wake category, distance to go, delay and what the aircraft is doing. ▲▼ change the order, ⤳ sends an arrival direct to the final, 🐢 slows it, ⟳ holds it, ⏵ leaves the hold, ↺ is a go-around. Below are the **Tower** (who is at each runway, and what they wait for) and the **Conflicts**. While the tab is open, the final is drawn on the map: the extended centreline to 15 NM and each arrival within 20 NM at its distance to go, red when it is closer than the spacing it needs.

![The Approach tab: the landing sequence, the tower, and the final on the map](images/airport-map/approach.jpg)

**Airport** has the airport, de-icing pads, the weather at your aircraft with the runway in use, the ATIS (🔊 reads it out), and the SIDs, STARs and approaches of a runway drawn on the map.

![The Airport tab: weather, ATIS and a STAR of runway 24](images/airport-map/charts.jpg)

**Layers** has the airport data, the traffic picture (its centre and radius), live traffic (our aircraft, other traffic, safe zones), taxiway names, overlapping stands and the taxi paths and points by `TYPE`. **🌐** switches the map to the world view: the traffic picture's circle, airports in range and every aircraft with call sign, level and phase.

![The world view: airports in range and traffic around them](images/airport-map/world.jpg)

**?** is the quick reference: every button, clearance and colour on one page.

The example's [README](https://github.com/mrlm-net/simconnect/tree/main/examples/airport-map) lists its HTTP API, which scripts and tests can use too.

## Other examples

Smaller examples, each showing one part of the API.

### Connection and lifecycle

| Example | What it shows | Docs |
|---------|---------------|------|
| [basic-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/basic-connection) | Connect, print the simulator's open message, disconnect | [Getting Started](getting-started.md) |
| [await-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/await-connection) | Retry until the simulator is up, shut down on Ctrl+C | [Client API](usage-client.md) |
| [lifecycle-connection](https://github.com/mrlm-net/simconnect/tree/main/examples/lifecycle-connection) | Reconnect whenever the simulator quits and comes back | [Client API](usage-client.md) |
| [simconnect-manager](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-manager) | `pkg/manager`: auto-reconnect and connection state callbacks | [Manager Usage](usage-manager.md) |
| [simconnect-state](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-state) | Manager `SimState`: pause, sim running, camera state | [Manager Usage](usage-manager.md) |
| [simconnect-subscribe](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-subscribe) | Manager channel subscriptions instead of callbacks | [Manager Usage](usage-manager.md) |
| [simconnect-events](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-events) | Manager system events: flight and aircraft loaded, flight plan, objects added and removed | [Event Lifecycle](events-lifecycle.md) |
| [simconnect-benchmark](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-benchmark) | Load test of the manager stack with CPU and memory profiles | |

### Data and events

| Example | What it shows | Docs |
|---------|---------------|------|
| [read-messages](https://github.com/mrlm-net/simconnect/tree/main/examples/read-messages) | Data definitions and periodic SimVar requests for the user aircraft and camera | [Client API](usage-client.md) |
| [read-objects](https://github.com/mrlm-net/simconnect/tree/main/examples/read-objects) | `EnumerateSimObjectsAndLiveries`: the aircraft and liveries installed | [Client API](usage-client.md) |
| [set-variables](https://github.com/mrlm-net/simconnect/tree/main/examples/set-variables) | Write a SimVar: change `CAMERA STATE` | [Client API](usage-client.md) |
| [using-datasets](https://github.com/mrlm-net/simconnect/tree/main/examples/using-datasets) | The dataset registry, cloning, the builder and merging | [Dataset Composition](dataset-composition.md) |
| [emit-events](https://github.com/mrlm-net/simconnect/tree/main/examples/emit-events) | Map and transmit client events: toggle aircraft doors | [Client API](usage-client.md) |
| [subscribe-events](https://github.com/mrlm-net/simconnect/tree/main/examples/subscribe-events) | System event subscriptions: pause, sim start and stop, sound | [Event Lifecycle](events-lifecycle.md) |
| [flow-events](https://github.com/mrlm-net/simconnect/tree/main/examples/flow-events) | `SubscribeToFlowEvent` (MSFS 2024 only) | [Client API](usage-client.md) |
| [cmd/simvar-cli](https://github.com/mrlm-net/simconnect/tree/main/cmd/simvar-cli) | Read, write, watch and stream SimVars from the terminal (own `go.mod`) | [SimVar CLI](simvar-cli.md) |

### Facilities

| Example | What it shows | Docs |
|---------|---------------|------|
| [read-facility](https://github.com/mrlm-net/simconnect/tree/main/examples/read-facility) | Facility data definitions for one airport | [Facilities](guide-facilities.md) |
| [read-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/read-facilities) | Facility lists, paginated | [Facilities](guide-facilities.md) |
| [subscribe-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/subscribe-facilities) | Subscribe to the airport list and its updates | [Facilities](guide-facilities.md) |
| [all-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/all-facilities) | Request the full airport list at once | [Facilities](guide-facilities.md) |
| [airport-details](https://github.com/mrlm-net/simconnect/tree/main/examples/airport-details) | Parking, runways and taxi paths of one airport, multi-packet responses | [Facilities](guide-facilities.md) |
| [locate-airport](https://github.com/mrlm-net/simconnect/tree/main/examples/locate-airport) | The nearest airports to the user aircraft | [Facilities](guide-facilities.md) |
| [read-waypoints](https://github.com/mrlm-net/simconnect/tree/main/examples/read-waypoints) | Waypoint facility data | [Facilities](guide-facilities.md) |
| [simconnect-facilities](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-facilities) | The ready-made facility datasets through the manager | [Datasets](usage-datasets.md) |

### Traffic

| Example | What it shows | Docs |
|---------|---------------|------|
| [ai-taxi](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-taxi) | One departure: pushback, taxi, line-up and take-off with `pkg/traffic` | [Departure Taxi](traffic-taxi.md) |
| [ai-arrival](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-arrival) | One arrival: final, touchdown, runway exit and taxi-in to a stand | [Arrivals](traffic-arrival.md) |
| [ai-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/ai-traffic) | Raw SimConnect: parked and en route ATC aircraft from `planes.json` (run it from its folder) | [Traffic Guide](traffic-guide.md) |
| [manage-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/manage-traffic) | Raw SimConnect: parked and airborne aircraft at LKPR driven by waypoints | [Traffic Guide](traffic-guide.md) |
| [monitor-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/monitor-traffic) | Every aircraft within 25 km, polled every 5 seconds | [Client API](usage-client.md) |
| [simconnect-traffic](https://github.com/mrlm-net/simconnect/tree/main/examples/simconnect-traffic) | The manager's traffic fleet: a parked aircraft at LFPG and a non-ATC aircraft at LKPR on waypoints, removed on exit | [Traffic Guide](traffic-guide.md) |

### Navigation and weather

| Example | What it shows | Docs |
|---------|---------------|------|
| [atis](https://github.com/mrlm-net/simconnect/tree/main/examples/atis) | Weather at the user aircraft, the runway in use and the airport's ATIS | [Weather & ATIS](nav-weather.md) |
| [flight-plan](https://github.com/mrlm-net/simconnect/tree/main/examples/flight-plan) | An IFR plan between two airports over the airways, written as a `.pln` | [Flight Plans](nav-flight-plans.md) |
| [spike-airways](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-airways) | Crawl the airway network around a centre and save it as JSON (the map's `-airways`) | [Airways](nav-airways.md) |

### Spikes

Experiments kept as a record of how the traffic features were found. Each answers one question about the simulator; they are not maintained as examples.

| Example | Question |
|---------|----------|
| [spike-airlines](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-airlines) | Can the airlines of a parking spot be read? |
| [spike-approach](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-approach) | Can a final approach, flare and rollout be flown by position injection? |
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
