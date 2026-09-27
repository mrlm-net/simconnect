# airport-map

Interactive map of an airport's ground layout as SimConnect reports it: runways, taxi paths (coloured by `TYPE`), taxi points (hold-short highlighted), taxiway names, parking spots and the user aircraft's live position.

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
| `-dump` | `false` | Write each fetched airport to `<ICAO>.json` |
| `-dump-dir` | `.` | Directory for `-dump` files |
| `-file` | | Serve a `-dump` file instead of connecting to the simulator |

## What it shows

- **Taxi paths** per `TYPE` (0 NONE … 8 PAINTEDLINE), each its own toggleable layer. Paths with an endpoint index outside the point/parking lists are drawn as magenta rings.
- **PARKING path endpoints.** The SDK documents `TAXI_PATH.START/END` as "taxiway point *or parking space*" indexes. The side panel switches between reading a PARKING path's `END` as a parking index or a taxi point index, and compares how many resolve and how long the resulting segments are.
- **Taxi points** by `TYPE`; hold-short types (2, 4, 5, 6) get their own layer.
- **Parking spots** as circles of their `RADIUS`. The gate label (e.g. `C22`) decodes `NAME` using the BGL enum and is a best guess; the popup shows the raw value.

## HTTP API

| Endpoint | Response |
|----------|----------|
| `GET /api/airport?icao=XXXX[&refresh=1]` | Airport facility data (same JSON as `-dump`) |
| `GET /api/aircraft` | User aircraft position, or `204` when unavailable |

The map page loads Leaflet from cdnjs and map tiles from OpenStreetMap and Esri, so the browser needs internet access.
