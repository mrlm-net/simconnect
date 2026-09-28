# airport-map

Interactive map of an airport's ground layout as SimConnect reports it: runways, taxi paths (coloured by `TYPE`), taxi points (hold-short highlighted), taxiway names, parking spots and the user aircraft's live position. It is built on [`pkg/airport`](../../docs/airport-layout.md): `airport.Loader` fetches the facility data from the application's message loop, `airport.Cache` keeps layouts and taxi graphs, and departure routes come from `Graph.RouteToRunway`.

It is a debugging tool for taxi routing. Every feature's popup shows its raw facility index and field values, so the taxi graph can be checked against the real data.

## Run

```bash
# Live: connect to the simulator and open LKPR
go run ./examples/airport-map

# Also save each fetched airport's raw data to <ICAO>.json
go run ./examples/airport-map -dump

# Offline: serve a saved dump, no simulator needed
go run ./examples/airport-map -file LKPR.json
```

Open <http://127.0.0.1:8080/?icao=LKPR>. Type another ICAO code in the side panel to load it; **↻** fetches it again from the simulator.

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address |
| `-icao` | `LKPR` | Airport opened on start |
| `-dump` | `false` | Write each fetched airport's raw facility records (`airport.RawAirport`) to `<ICAO>.json`; the same format as `pkg/airport/testdata` |
| `-dump-dir` | `.` | Directory for `-dump` files |
| `-file` | | Serve a `-dump` file instead of connecting to the simulator |
| `-log-dir` | `.` | Directory for the traffic control log, `traffic-<YYYYMMDD-HHMMSS>.log` (one file per run) |

## What it shows

- **Taxi paths** per `TYPE` (0 NONE … 8 PAINTEDLINE), each its own toggleable layer. Paths with an endpoint index outside the point/parking lists are drawn as magenta rings.
- **PARKING paths** are drawn from their START taxi point to the parking spot at END (END indexes the parking list for PARKING paths, verified on LKPR).
- **Taxi points** by `TYPE`; hold-short types (2, 4, 5, 6) get their own layer.
- **Parking spots** as circles of their `RADIUS`, labelled from `NAME`, `NUMBER` and `SUFFIX` (e.g. `C22`, `S22A`). Stands whose circles overlap are outlined in orange.
- **Live traffic:** every aircraft within 20 km (sim AI, injected AI, other clients' aircraft), updated every second, with tail, height, speed, vertical speed, gear and AI state on the map and in a side-panel table.
- **Route viewer:** click a parking spot and pick a runway end. Departure mode draws the route to the runway (full length or from an entry), its taxiway sequence, length, runway crossings and target hold-short. Arrival mode draws the taxi-in from a runway exit, with the vacate stop and the stop point on the stand.

## Traffic control

While connected to the simulator, the **Traffic control** panel spawns AI aircraft driven by [`pkg/traffic`](../../docs/traffic-arrival.md) with position injection, and gives their clearances.

- **Spawn:** the panel spawns what the **Route** section shows. Pick the mode (departure or arrival), click a stand and pick a runway (and an entry for departures); the single **▶ Spawn** button says what it will do, e.g. "▶ Spawn departure: C22 → runway 24 at B" or "▶ Spawn arrival: runway 24 → C22". A departure pushes back, taxis, lines up and takes off; an arrival lands and taxis to the stand (the exit is chosen by the controller). Only the options of the mode are shown: **Pushback tug** for departures, **Fly the approach** for arrivals.
- **Model:** a searchable list of the aircraft the simulator can spawn. Type words to filter, ▾ shows all, arrow keys and Enter pick one. Entries are `Title :: Livery`; the part after ` :: ` is passed as the livery.
- **Hold at every clearance** (`gates`): departures stop at every gate (`HoldForClearances`); arrivals wait clear of the runway and short of runway crossings (`HoldForClearance`, `HoldAtCrossings`). Unticked, the gates clear themselves after a short wait.
- **Fly the approach (injected)** (`injectApproach`): arrivals fly the approach, flare and touchdown by injection; unticked, MSFS AI lands and injection takes over on the runway.
- **Stands:** a stand held by another aircraft (controlled, or found standing there by the stand scan) is refused with the reason. **Assign a free stand** picks one instead: suitable for an A320, the airline's own stands first (the airline is read from a callsign-style tail such as `BAW851`), for arrivals the shortest taxi-in from the runway. The **Occupied stands** layer shows reserved stands in blue and aircraft found on stands in red; departures free their stand when they push back.
- **Clearances:** each aircraft in the list shows its state, speed, lights and the buttons available now: Pushback, Taxi, Cross, Line up, Take-off, and ✕ to remove it.
- **Progressive taxi:** select an aircraft to draw its route on the map; click a route point to clear it up to there (`ClearUpTo`). The limit is drawn in magenta, and the list shows *at limit* while the aircraft holds there. **Taxi** removes the limit.

### Traffic log

Everything traffic control does is logged: spawns, clearances given or refused, state changes (with the touchdown distance and rate), light changes, arrival at and departure from a clearance limit, and errors. Each line is time-stamped and goes to the console, to the log file in `-log-dir`, and to the **Traffic log** panel (the last 200 lines).

## HTTP API

| Endpoint | Response |
|----------|----------|
| `GET /api/airport?icao=XXXX[&refresh=1]` | The `airport.Layout` plus `fetchedAt` |
| `GET /api/geojson?icao=XXXX` | The layout as a GeoJSON FeatureCollection |
| `GET /api/route?icao=XXXX&from=<parking index>&to=<runway end>[&entry=<taxiway>]` | `airport.Route` from `RouteToRunwayEntry` (empty entry = full length; `&runwayPaths=1` allows taxiing on runway paths) |
| `GET /api/entries?icao=XXXX&runway=<runway end>` | `airport.RunwayEntry` list, each with the `position` where it meets the runway |
| `GET /api/exits?icao=XXXX&runway=<runway end>` | `airport.RunwayExit` list for landings on that end, nearest the threshold first, each with its `position` |
| `GET /api/arrival?icao=XXXX&runway=<runway end>&to=<parking index>[&exit=<index>]` | Arrival plan from `traffic.PlanArrival`: `route`, `exit`, `vacate` and `stop` positions; `exit` indexes `/api/exits`, without it the controller's choice |
| `GET /api/aircraft` | User aircraft position, or `204` when unavailable |
| `GET /api/traffic` | Every aircraft within 20 km of the user aircraft: title, tail, AI state, position, height above ground, ground speed, vertical speed, heading, on-ground, gear extension (0–1), `user` flag |
| `GET /api/control` | Controlled aircraft: id, kind, tail, model, stand, runway, state, `holdingShortOf`, `atLimit`, `limitNode`, position, heading, ground speed, lights, error, `route` / `nodes`, `actions` (clearances available now), `done` |
| `POST /api/control` | Spawn a controlled aircraft. JSON body: `kind` (`departure` or `arrival`), `icao`, `stand` (parking index), `runway`, `entry` (departure), `model` (`Title` or `Title :: Livery`; default FSLTL A320 Air France SL), `tail` (default `MAPnn`), `gates`, `injectApproach`. Returns its view; `422` if the controller refuses it, `503` when not connected |
| `POST /api/control/{id}/{action}[?node=N]` | A clearance: `pushback`, `taxi`, `upto` (with `node`, a graph node ID on the route), `cross`, `lineup`, `takeoff` (departures), `remove`. `204` on success, `422` with the reason when refused, `404` for an unknown id |
| `GET /api/control/log` | The last 200 traffic log lines, newest last |
| `GET /api/stands?icao=X` | Held stands: index, label, owner (controlled traffic), detected, object ID, half span |
| `GET /api/models` | The aircraft titles the simulator can spawn, sorted, as `Title :: Livery` where a livery is known |

The map page loads Leaflet from cdnjs and map tiles from OpenStreetMap and Esri, so the browser needs internet access.
