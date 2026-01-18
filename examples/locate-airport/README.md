# Locate Airport Example

## Overview

This example demonstrates geolocation-based airport discovery. It shows how to retrieve your current position from the simulator and calculate distances to airports using the haversine formula, allowing you to find the nearest airport or airports within a specific radius.

## What It Does

1. **Connects to the simulator** — Establishes a connection and waits for the simulator to load
2. **Reads user position** — Periodically requests current aircraft latitude, longitude, and altitude
3. **Requests airport data** — Queries the facilities database for all airports
4. **Calculates distances** — Uses haversine formula to compute great-circle distances between aircraft and airports
5. **Finds nearby airports** — Identifies the nearest airports and those within specified radius
6. **Displays information** — Shows airport names, ICAO codes, and distances

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded and flying in the simulator

## Running the Example

```bash
cd examples/locate-airport
go run main.go
```

## Expected Output

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect...
📡 Requesting airport data...
✅ Ready to search for nearby airports...
🎯 Your position: Lat: 48.3521°, Lon: 11.7861°, Alt: 3500 ft
🛫 Nearby Airports (within 50 km):
   1. EDDM (Munich) - 12.4 km away
   2. EDMM (Augsburg) - 28.6 km away
   3. EDST (Straubing) - 42.1 km away
```

## Code Explanation

### Key APIs Used

- **AddToDataDefinition()** — Define camera/position data structure
- **RequestDataOnSimObject()** — Request periodic position updates
- **RequestFacilitiesList()** — Query all airports from facilities database
- **haversineMeters()** — Calculate great-circle distance between coordinates

### Haversine Formula

The example uses the haversine formula to calculate the shortest distance between two points on Earth's surface:

```go
func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
    const earthRadius = 6371000.0 // meters
    // Calculate distance using spherical law of cosines
    // Result is in meters
}
```

### GPS Position Tracking

Position data is requested every second:

```go
client.AddToDataDefinition(2000, "GPS POSITION ALT", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
client.AddToDataDefinition(2000, "GPS POSITION LAT", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
client.AddToDataDefinition(2000, "GPS POSITION LON", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)

client.RequestDataOnSimObject(2001, 2000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
```

### Facility Data Parsing

Airports are retrieved from the facilities database:

```
Airport Name
├── ICAO Code
├── Latitude
├── Longitude
└── Altitude
```

### Distance Calculation

Once you have your position and airport coordinates:

```go
distanceMeters := haversineMeters(yourLat, yourLon, airportLat, airportLon)
distanceKm := distanceMeters / 1000.0
```

## Use Cases

- **Navigation planning** — Find alternate airports within range
- **Approach procedures** — Identify surrounding airfields for diversion
- **Route analysis** — Determine airports along flight path
- **Emergency planning** — Locate nearest suitable landing options

## Related Examples

- [`read-facility`](../read-facility) — Retrieve single facility by ICAO code
- [`read-facilities`](../read-facilities) — Query facilities with filtering
- [`all-facilities`](../all-facilities) — Enumerate complete facility database

## See Also

- [Facilities Data](../../pkg/datasets/facilities) — Pre-built facility data definitions
- [SimConnect Facilities API](../../docs/config-client.md) — Request facilities documentation
