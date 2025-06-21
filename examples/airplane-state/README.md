# Airplane State Example

Comprehensive aircraft telemetry monitoring with real-time web dashboard.

## What it demonstrates

- **Complex Data Definitions**: Monitor 16+ aircraft parameters simultaneously
- **Data Conversion**: Handle radians-to-degrees conversion and unit scaling
- **Web Dashboard**: Serve real-time telemetry via HTTP server on port 8080
- **JSON API**: RESTful endpoint for aircraft data consumption
- **Concurrent Processing**: Separate goroutines for SimConnect and web server

## How to run

```bash
cd examples/airplane-state
go run main.go
```

Then open http://localhost:8080 in your browser for the live dashboard.

## API Endpoints

- `GET /` - Web dashboard with live telemetry
- `GET /api/aircraft` - JSON aircraft data

## Monitored Parameters

| Parameter | Unit | Description |
|-----------|------|-------------|
| Altitude | feet | Aircraft altitude above sea level |
| Ground Speed | knots | Speed over ground |
| Position | degrees | Latitude/longitude coordinates |
| Vertical Speed | fpm | Rate of climb/descent |
| Attitude | degrees | Pitch, bank, and heading |
| Airspeed | knots/mach | Indicated, true, and mach speed |
| Surface Info | - | Runway, parking, surface conditions |

## Key code patterns

```go
// Complex data structure matching SimConnect order
type AircraftData struct {
    Altitude          float64
    GroundSpeed       float64
    Latitude          float64
    // ... more fields
}

// HTTP server integration
http.HandleFunc("/api/aircraft", handleAircraftData)
go http.ListenAndServe(":8080", nil)
```

## Requirements

- Running MSFS with aircraft in flight or on ground
- Web browser for dashboard access
