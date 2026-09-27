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

## What it shows

- **Taxi paths** per `TYPE` (0 NONE … 8 PAINTEDLINE), each its own toggleable layer. Paths with an endpoint index outside the point/parking lists are drawn as magenta rings.
- **PARKING path endpoints.** The SDK documents `TAXI_PATH.START/END` as "taxiway point *or parking space*" indexes. The side panel switches between reading a PARKING path's `END` as a parking index or a taxi point index, and compares how many resolve and how long the resulting segments are.
- **Taxi points** by `TYPE`; hold-short types (2, 4, 5, 6) get their own layer.
- **Parking spots** as circles of their `RADIUS`, labelled from `NAME`, `NUMBER` and `SUFFIX` (e.g. `C22`, `S22A`). Stands whose circles overlap are outlined in orange.
- **Live traffic:** every aircraft within 20 km (sim AI, injected AI, other clients' aircraft), updated every second, with tail, height, speed, vertical speed, gear and AI state on the map and in a side-panel table.
- **Departure route:** click a parking spot and pick a runway end to draw the route, its taxiway sequence, length, runway crossings and target hold-short.

## HTTP API

| Endpoint | Response |
|----------|----------|
| `GET /api/airport?icao=XXXX[&refresh=1]` | The `airport.Layout` plus `fetchedAt` |
| `GET /api/geojson?icao=XXXX` | The layout as a GeoJSON FeatureCollection |
| `GET /api/route?icao=XXXX&from=<parking index>&to=<runway end>` | `airport.Route` from `RouteToRunway` (`&runwayPaths=1` allows taxiing on runway paths) |
| `GET /api/aircraft` | User aircraft position, or `204` when unavailable |
| `GET /api/traffic` | Every aircraft within 20 km of the user aircraft: title, tail, AI state, position, height above ground, ground speed, vertical speed, heading, on-ground, gear extension (0–1), `user` flag |

The map page loads Leaflet from cdnjs and map tiles from OpenStreetMap and Esri, so the browser needs internet access.
