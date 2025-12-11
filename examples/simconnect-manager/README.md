# SimConnect Manager Example

## Overview

This example demonstrates how to use the Manager interface for robust connection lifecycle management in Microsoft Flight Simulator. The Manager automatically handles connection, reconnection, and state transitions, allowing you to focus on your application logic rather than connection management.

## What It Does

1. **Automatic connection management** - Manager handles connect/disconnect lifecycle automatically
2. **State-based setup** - Registers data definitions and subscriptions when connection becomes available
3. **Automatic reconnection** - Reconnects automatically if the simulator disconnects or restarts
4. **Event-driven architecture** - Uses callbacks for state changes and message handling
5. **Subscribes to system events** - Monitors Pause, Sim, and Sound events
6. **Requests periodic data** - Retrieves camera state updates every second
7. **Monitors nearby aircraft** - Requests detailed data for all aircraft within 10km radius
8. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see data updates)

## Running the Example

```bash
cd examples/simconnect-manager
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
🔄 State changed: disconnected -> connecting
⏳ Connecting to simulator...
🔄 State changed: connecting -> connected
✅ Connected to SimConnect, simulator is loading...
✅ Setting up data definitions and event subscriptions...
📨 Message received - SIMCONNECT_RECV_ID_OPEN
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator'
  Application Version: 1.0
  Application Build: 1.0
  SimConnect Version: 12.0
  SimConnect Build: 62651.0
🔄 State changed: connected -> available
🚀 Simulator connection is AVAILABLE. Ready to process messages...
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1001, Data: 1
  🏁 Simulator SIM STARTED
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA
  => Received SimObject data event
     Camera State: 2, Camera Substate: 0, Category: Airplane
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Aircraft Title: Boeing 747-8i Asobo, Livery Name: ...
🔄 State changed: available -> disconnected
📴 Disconnected from simulator...
🔄 State changed: disconnected -> reconnecting
🔄 Reconnecting to simulator...
```

## Code Explanation

### Manager vs Direct Client

The Manager provides a higher-level abstraction compared to using `simconnect.NewClient()` directly:

| Feature | Manager (`manager.New`) | Direct Client (`simconnect.NewClient`) |
|---------|------------------------|---------------------------------------|
| Auto-reconnect | ✅ Built-in | ❌ Manual implementation |
| State tracking | ✅ Automatic | ❌ Manual implementation |
| Connection lifecycle | ✅ Managed | ❌ Manual management |
| Use case | Long-running services | Simple scripts |

### Creating the Manager

```go
mgr := manager.New("GO Example - SimConnect Manager",
    manager.WithContext(ctx),
    manager.WithAutoReconnect(true),
)
```

Options:
- `WithContext(ctx)` - Provides cancellation context for graceful shutdown
- `WithAutoReconnect(true)` - Enables automatic reconnection when simulator disconnects

### Connection States

The Manager tracks these connection states:

| State | Description |
|-------|-------------|
| `StateDisconnected` | Not connected to simulator |
| `StateConnecting` | Connection attempt in progress |
| `StateConnected` | Connected, but OPEN message not yet received |
| `StateAvailable` | Fully connected and ready to process messages |
| `StateReconnecting` | Attempting to reconnect after disconnect |

### State Change Handler

Register a callback to respond to state transitions:

```go
mgr.OnStateChange(func(oldState, newState manager.ConnectionState) {
    switch newState {
    case manager.StateConnected:
        // Setup data definitions when connected
        if client := mgr.Client(); client != nil {
            setupDataDefinitions(client)
        }
    case manager.StateAvailable:
        // Connection fully ready
    case manager.StateDisconnected:
        // Handle disconnect
    }
})
```

### Message Handler

Register a callback to process incoming messages:

```go
mgr.OnMessage(func(msg engine.Message) {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {
    case types.SIMCONNECT_RECV_ID_EVENT:
        // Handle events
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
        // Handle sim object data
    }
})
```

### Accessing the Client

The underlying `engine.Client` is available via `mgr.Client()` for direct API calls:

```go
if client := mgr.Client(); client != nil {
    client.SubscribeToSystemEvent(1000, "Pause")
    client.AddToDataDefinition(...)
    client.RequestDataOnSimObject(...)
}
```

### Starting the Manager

```go
if err := mgr.Start(); err != nil {
    fmt.Printf("Manager stopped: %v\n", err)
}
```

The `Start()` method blocks until the context is cancelled. It handles all connection lifecycle events internally.

## Data Structures

### CameraData

```go
type CameraData struct {
    CameraState    int32
    CameraSubstate int32
    Category       [260]byte
}
```

### AircraftData

```go
type AircraftData struct {
    Title             [128]byte
    LiveryName        [128]byte
    LiveryFolder      [128]byte
    Lat               float64
    Lon               float64
    Alt               float64
    // ... additional fields
}
```

## When to Use Manager vs Direct Client

**Use Manager when:**
- Building long-running services or background applications
- You need automatic reconnection handling
- You want event-driven architecture with state callbacks
- Connection reliability is important

**Use Direct Client when:**
- Building simple scripts or one-off tools
- You need fine-grained control over connection timing
- You're implementing custom connection logic
- Minimal overhead is required