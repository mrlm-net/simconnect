# SimConnect Traffic Example (Manager)

## Overview

This example demonstrates how to monitor AI traffic (both parked and in-flight aircraft) using the **manager pattern** for automatic connection lifecycle handling and reconnection support. It shows how to subscribe to aircraft data, handle periodic updates, and track multiple AI aircraft in real-time.

## What It Does

1. **Connects to simulator** — Uses the manager to establish connection with automatic reconnection
2. **Registers dataset** — Uses the pre-built `traffic.AircraftDataset` for aircraft data
3. **Monitors traffic** — Requests periodic position and state updates for all aircraft within 25km
4. **Handles lifecycle** — Automatically reconnects if simulator disconnects or restarts
5. **Tracks positions** — Displays current position, heading, altitude, and speed for each aircraft

## Key Differences from monitor-traffic Example

This example uses the **manager pattern** instead of the raw client:

- **Automatic reconnection** — Manager handles disconnection and reconnection automatically
- **Lifecycle management** — Connection state changes trigger appropriate callbacks
- **Pre-built dataset** — Uses `traffic.NewAircraftDataset()` instead of manual field definitions
- **Simplified code** — No manual connection retry loops or stream handling
- **State-driven setup** — Data definitions registered on connection state change

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
cd examples/simconnect-traffic
go run .
```

Or from the project root:

```bash
go run ./examples/simconnect-traffic
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
🔄 Connection state changed: Disconnected -> Connecting
⏳ Connecting to simulator...
🔄 Connection state changed: Connecting -> Connected
✅ Connected to SimConnect
✅ Setting up aircraft data definitions...
📨 Message received -  SIMCONNECT_RECV_ID_OPEN
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator 2024'
  Application Version: 1.0
  Application Build: 1.0
  SimConnect Version: 12.0
  SimConnect Build: 61637.0
🔄 Connection state changed: Connected -> Available
🚀 Simulator connection is AVAILABLE
📨 Message received -  SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Request ID: 4001, Define ID: 3000, Object ID: 1, Flags: 0, Out of: 2, DefineCount: 1
     Aircraft Title: Airbus A320neo, Category: Airplane, Livery Name: Asobo, Livery Folder: Asobo_A320neo, Lat: 48.354100, Lon: 11.786100, Alt: 1500.000000, Head: 270.000000, GroundSpeed: 0.000000, AtcID: N12345, AtcAirline: AAL
📨 Message received -  SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Request ID: 4001, Define ID: 3000, Object ID: 2, Flags: 0, Out of: 2, DefineCount: 2
     Aircraft Title: Boeing 747-8i, Category: Airplane, Livery Name: Delta, Livery Folder: Delta_747, Lat: 48.355000, Lon: 11.787000, Alt: 1505.000000, Head: 275.000000, GroundSpeed: 145.000000, AtcID: DAL123, AtcAirline: DAL
```

## Code Explanation

### Manager Configuration

The example creates a manager with:

```go
mgr := manager.New("GO Example - SimConnect Traffic Monitor",
    manager.WithContext(ctx),
    manager.WithAutoReconnect(true),
    manager.WithBufferSize(512),
    manager.WithHeartbeat("6Hz"),
)
```

- **WithContext** — Allows graceful shutdown via context cancellation
- **WithAutoReconnect** — Enables automatic reconnection on disconnect
- **WithBufferSize** — Sets message buffer size for high-frequency data
- **WithHeartbeat** — Sets heartbeat frequency for connection monitoring

### Connection State Handling

The manager provides state change callbacks:

```go
mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
    switch newState {
    case manager.StateConnected:
        // Setup data definitions
        setupDataDefinitions(mgr)
    case manager.StateAvailable:
        // Start periodic data requests
        startPeriodicRequests()
    }
})
```

States: `Disconnected` → `Connecting` → `Connected` → `Available`

### Dataset Registration

Uses the pre-built traffic dataset:

```go
func setupDataDefinitions(mgr manager.Manager) {
    // Register complete dataset with one call
    mgr.RegisterDataset(3000, traffic.NewAircraftDataset())

    // Request data for all aircraft within 25km
    mgr.RequestDataOnSimObjectType(4001, 3000, 25000,
        types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
}
```

### Data Structure

The `AircraftData` type wraps `traffic.AircraftDataset`:

```go
type AircraftData traffic.AircraftDataset

func (data *AircraftData) TitleAsString() string {
    return engine.BytesToString(data.Title[:])
}
// ... other helper methods
```

Fields include:
- **Title**, **Category**, **LiveryName**, **LiveryFolder** — Aircraft identification
- **Lat**, **Lon**, **Alt**, **Head** — Position and heading
- **GroundSpeed**, **AirspeedIndicated**, **AirspeedTrue** — Speed data
- **AtcID**, **AtcAirline** — ATC information
- **Vs**, **Pitch**, **Bank** — Flight dynamics
- **OnAnyRunway**, **SurfaceType**, **SimOnGround** — Ground state

### Periodic Data Requests

Uses atomic flag to prevent multiple ticker goroutines on reconnect:

```go
var tickerStarted atomic.Bool

if tickerStarted.CompareAndSwap(false, true) {
    ticker := time.NewTicker(5 * time.Second)
    go func() {
        for {
            select {
            case <-ctx.Done():
                ticker.Stop()
                return
            case <-ticker.C:
                mgr.RequestDataOnSimObjectType(...)
            }
        }
    }()
}
```

### Message Handling

Processes incoming messages via callback:

```go
mgr.OnMessage(func(msg engine.Message) {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
        // Process aircraft data
        simObjData := msg.AsSimObjectDataBType()
        if simObjData.DwDefineID == 3000 {
            aircraftData := engine.CastDataAs[AircraftData](&simObjData.DwData)
            // Display aircraft information
        }
    }
})
```

## Use Cases

- **Traffic monitoring** — Watch AI aircraft movement in real-time
- **Air traffic control simulation** — Manage traffic flow from control tower
- **Flight analysis** — Track multiple aircraft positions and states
- **Testing** — Validate traffic scenarios and procedures
- **Flight schools** — Monitor multi-aircraft training environments

## Performance Considerations

- **Search radius** — 25km (25000 meters) balances coverage and performance
- **Update frequency** — 5 seconds provides good balance for traffic monitoring
- **Buffer size** — 512 messages handles high-frequency updates
- **Automatic cleanup** — Manager handles resource cleanup on disconnect

## Configuration Tips

1. **Adjust search radius** — Increase/decrease `25000` for wider/narrower coverage
2. **Change update frequency** — Modify ticker duration for faster/slower updates
3. **Filter messages** — Add filters to only process specific aircraft types
4. **Add error handling** — Extend error handling for production use

## Related Examples

- [`monitor-traffic`](../monitor-traffic) — Same functionality using raw client pattern
- [`simconnect-manager`](../simconnect-manager) — Basic manager usage example
- [`simconnect-state`](../simconnect-state) — Connection state and sim state tracking
- [`simconnect-subscribe`](../simconnect-subscribe) — Channel-based message subscriptions

## See Also

- [Manager API Documentation](../../docs/manager.md) — Manager interface and options
- [Traffic Dataset](../../pkg/datasets/traffic) — Pre-built traffic data definitions
- [Connection Lifecycle](../../docs/connection-lifecycle.md) — State transitions and handling
