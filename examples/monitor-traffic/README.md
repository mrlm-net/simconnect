# Monitor Traffic Example

## Overview

This example polls every aircraft within 25 km of the user aircraft every 5 seconds with the raw client API and prints what SimConnect reports for each. It spawns nothing and reads no configuration file.

## What It Does

1. **Connects to the simulator** — Retries every 2 seconds until MSFS is running, and reconnects 5 seconds after the simulator quits
2. **Defines the data** — Registers data definition `3000` with 20 SimVars per aircraft (title, category, livery, position, altitude, headings, vertical speed, pitch, bank, speeds, runway and ground flags, ATC ID and airline)
3. **Polls the traffic** — Calls `RequestDataOnSimObjectType` for `SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT` within 25,000 m once at start and then every 5 seconds
4. **Prints each aircraft** — For every `SIMOBJECT_DATA_BYTYPE` message: the request and object IDs, then title, category, livery, latitude, longitude, altitude, heading, ground speed, ATC ID and airline

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
go run ./examples/monitor-traffic
```

There are no flags. Press Ctrl+C to exit.

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📨 Message received -  SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Request ID: 4001, Define ID: 3000, Object ID: 1, Flags: 0, Out of: 3, DefineCount: 20
     Aircraft Title: ..., Category: Airplane, Livery Name: ..., Lat: 50.098471, Lon: 14.285940, Alt: 1247.0, Head: 169.0, GroundSpeed: 0.0, AtcID: ..., AtcAirline: ...
```

## Code Explanation

### Data Structure

The struct fields follow the order of the `AddToDataDefinition` calls in `addPlanesRequestDataset`:

```go
type AircraftData struct {
    Title             [128]byte
    Category          [128]byte
    LiveryName        [128]byte
    LiveryFolder      [128]byte
    Lat               float64
    Lon               float64
    Alt               float64
    Head              float64
    HeadMag           float64
    Vs                float64
    Pitch             float64
    Bank              float64
    GroundSpeed       float64
    AirspeedIndicated float64
    AirspeedTrue      float64
    OnAnyRunway       int32
    SurfaceType       int32
    SimOnGround       int32
    AtcID             [32]byte
    AtcAirline        [32]byte
}
```

### Polling

```go
// All aircraft within 25 km, request 4001, definition 3000
client.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

SimConnect answers with one `SIMOBJECT_DATA_BYTYPE` message per aircraft; `DwOutOf` says how many there are. Each is read with `engine.CastDataAs[AircraftData]`.

The `ParkedAircraft` and `IFRAircraft` types and the commented-out `AICreate...` calls in `main.go` are unused leftovers; spawning is shown in [`ai-traffic`](../ai-traffic) and [`manage-traffic`](../manage-traffic).

## Related Examples

- [`simconnect-traffic`](../simconnect-traffic) — Spawn traffic through the manager's fleet
- [`ai-traffic`](../ai-traffic) — Spawn parked and en route aircraft from `planes.json`
- [`airport-map`](../airport-map) — All traffic around an airport on a live map

## See Also

- [Client API](../../docs/usage-client.md) — Data definitions and requests
- [Traffic Picture](../../docs/traffic-picture.md) — Tracking all traffic around a centre with `pkg/traffic`
