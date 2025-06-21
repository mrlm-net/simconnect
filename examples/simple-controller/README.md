# Simple Controller Example

Basic demonstration of SimConnect client usage for monitoring and controlling aircraft systems.

## What it demonstrates

- **SimVar Monitoring**: Read `EXTERNAL POWER ON` state in real-time
- **Event Triggering**: Toggle external power using `TOGGLE_EXTERNAL_POWER` event
- **Basic Message Loop**: Process SimConnect messages with proper error handling
- **Graceful Shutdown**: Handle system signals and cleanup resources

## How to run

```bash
cd examples/simple-controller
go run main.go
```

## Usage

1. Start MSFS and load any aircraft
2. Run the example
3. Press `t` + Enter to toggle external power
4. Press `q` + Enter to quit
5. Use Ctrl+C for emergency shutdown

## Key code patterns

```go
// Setup data definition for SimVar
client.AddToDataDefinition(EXTERNAL_POWER_DEFINITION, "EXTERNAL POWER ON", "Bool", types.DATATYPE_INT32)

// Request periodic data updates
client.RequestDataOnSimObject(EXTERNAL_POWER_REQUEST, EXTERNAL_POWER_DEFINITION, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

// Map and trigger events
client.MapClientEventToSimEvent(EVENT_TOGGLE_EXTERNAL_POWER, "TOGGLE_EXTERNAL_POWER")
client.TransmitClientEvent(EVENT_TOGGLE_EXTERNAL_POWER, 0)
```

## Requirements

- Running MSFS with any aircraft loaded
- External power source available at airport
