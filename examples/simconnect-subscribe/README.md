# SimConnect Subscribe Example

## Overview

This example demonstrates how to use the **Subscribe pattern** for receiving messages and state changes from the Manager interface. Instead of using callbacks (`OnMessage`, `OnStateChange`), this example shows how to create channel-based subscriptions for both message handling and state change monitoring, which is useful for isolating processing in separate goroutines or implementing fan-out patterns.

## What It Does

1. **Channel-based message delivery** - Uses `Subscribe()` to receive messages via Go channels
2. **Channel-based state change delivery** - Uses `SubscribeStateChange()` to receive state changes via Go channels
3. **Automatic connection management** - Manager handles connect/disconnect lifecycle automatically
4. **State-based setup** - Registers data definitions and subscriptions when connection becomes available
5. **Automatic reconnection** - Reconnects automatically if the simulator disconnects or restarts
6. **Subscribes to system events** - Monitors Pause, Sim, and Sound events
7. **Requests periodic data** - Retrieves camera state updates every second
8. **Monitors nearby aircraft** - Requests detailed data for all aircraft within 10km radius
9. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see data updates)

## Running the Example

```bash
cd examples/simconnect-subscribe
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
📬 Message subscription started, waiting for messages...
📬 State subscription started, waiting for state changes...
🔄 State changed: disconnected -> connecting
📡 [Subscription] State changed: Disconnected -> Connecting
⏳ Connecting to simulator...
🔄 State changed: connecting -> connected
📡 [Subscription] State changed: Connecting -> Connected
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
📡 [Subscription] State changed: Connected -> Available
🚀 Simulator connection is AVAILABLE. Ready to process messages...
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1001, Data: 1
  🏁 Simulator SIM STARTED
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA
  => Received SimObject data event
     Camera State: 2, Camera Substate: 0, Category: Airplane
📨 Message received - SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
     Aircraft Title: Boeing 747-8i Asobo, Livery Name: ...
^C
🛑 Received interrupt signal, shutting down...
📭 State subscription cancelled
📭 Subscription channel closed
👋 Goodbye!
```

## Code Explanation

### Subscribe vs Callbacks

This example uses the **Subscribe pattern** for both messages and state changes:

#### Message Subscriptions

| Feature | Subscribe (`mgr.Subscribe`) | OnMessage (`mgr.OnMessage`) |
|---------|----------------------------|----------------------------|
| Message delivery | Via Go channel | Via callback function |
| Concurrency | Consumer controls processing | Callback runs in dispatcher |
| Multiple consumers | ✅ Multiple subscriptions | ❌ Single callback |
| Backpressure | Channel buffering | None (callback must return) |
| Use case | Fan-out, isolated processing | Simple single consumer |

#### State Change Subscriptions

| Feature | SubscribeStateChange (`mgr.SubscribeStateChange`) | OnStateChange (`mgr.OnStateChange`) |
|---------|--------------------------------------------------|-----------------------------------|
| State delivery | Via Go channel | Via callback function |
| Concurrency | Consumer controls processing | Callback runs in state updater |
| Multiple consumers | ✅ Multiple subscriptions | ✅ Multiple callbacks |
| Backpressure | Channel buffering | None (callback must return) |
| Use case | Async processing, fan-out | Simple inline handling |

### Creating a Message Subscription

```go
// Create a subscription with ID and buffer size
sub := mgr.Subscribe("main-subscriber", 256)
```

Parameters:
- First parameter: Subscription ID (use empty string `""` for auto-generated UUID)
- Second parameter: Channel buffer size for message buffering

### Creating a State Change Subscription

```go
// Create a state change subscription with ID and buffer size
stateSub := mgr.SubscribeStateChange("state-subscriber", 16)
```

Parameters:
- First parameter: Subscription ID (use empty string `""` for auto-generated UUID)
- Second parameter: Channel buffer size (state changes are less frequent, smaller buffer is fine)

### Processing Messages

```go
go func() {
    for {
        select {
        case msg, ok := <-sub.Messages():
            if !ok {
                // Channel closed, subscription ended
                return
            }
            // Process the message
            handleMessage(msg)
        case <-sub.Done():
            // Subscription was cancelled
            return
        }
    }
}()
```

The subscription provides:
- `Messages()` - Returns a receive-only channel for incoming messages
- `Done()` - Returns a channel that closes when the subscription ends
- `ID()` - Returns the subscription identifier
- `Unsubscribe()` - Cancels the subscription and releases resources

### Processing State Changes

```go
go func() {
    for {
        select {
        case change, ok := <-stateSub.StateChanges():
            if !ok {
                // Channel closed, subscription ended
                return
            }
            // Process the state change
            fmt.Printf("State: %s -> %s\n", change.OldState, change.NewState)
            
            // React to specific states
            if change.NewState == manager.StateConnected {
                // Setup data definitions when connected
            }
        case <-stateSub.Done():
            // Subscription was cancelled
            return
        }
    }
}()
```

The state subscription provides:
- `StateChanges()` - Returns a receive-only channel for `StateChange` events
- `Done()` - Returns a channel that closes when the subscription ends
- `ID()` - Returns the subscription identifier
- `Unsubscribe()` - Cancels the subscription and releases resources

The `StateChange` struct contains:
- `OldState` - The previous connection state
- `NewState` - The new connection state

### Cleanup

Always call `Unsubscribe()` when done to release resources:

```go
// Unsubscribe when done (cleanup)
sub.Unsubscribe()
stateSub.Unsubscribe()
```

### Hybrid Approach: Callbacks and Subscriptions Together

You can use both callbacks and subscriptions simultaneously. This example demonstrates both patterns:

```go
// Callback-based state handling (immediate, synchronous)
mgr.OnStateChange(func(oldState, newState manager.ConnectionState) {
    switch newState {
    case manager.StateConnected:
        // Setup data definitions when connected
        if client := mgr.Client(); client != nil {
            setupDataDefinitions(client)
        }
    case manager.StateAvailable:
        // Connection fully ready
    }
})

// Channel-based state handling (asynchronous, independent goroutine)
stateSub := mgr.SubscribeStateChange("state-logger", 16)
go func() {
    for change := range stateSub.StateChanges() {
        log.Printf("State: %s -> %s", change.OldState, change.NewState)
    }
}()
```

Both callbacks and subscriptions receive the same state changes, allowing different components to handle them independently.
```

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

## When to Use Subscribe vs Callbacks

### Message Subscriptions

**Use Subscribe when:**
- You need multiple independent message consumers
- You want to process messages in isolated goroutines
- You need channel-based backpressure handling
- Building fan-out architectures where multiple components need the same messages
- You prefer channel-based concurrency patterns

**Use OnMessage when:**
- You have a single message consumer
- Simple callback-based processing is sufficient
- You don't need message buffering or backpressure
- Minimal setup is preferred

### State Change Subscriptions

**Use SubscribeStateChange when:**
- You need to monitor state changes in isolated goroutines
- You want to decouple state handling from the main flow
- You need multiple independent state change consumers
- Building reactive architectures with state-driven behavior
- You prefer channel-based concurrency patterns

**Use OnStateChange when:**
- You need immediate, synchronous reaction to state changes
- Setting up resources (data definitions) when connection is ready
- Simple callback-based handling is sufficient
- Minimal setup is preferred

## Multiple Subscriptions

You can create multiple subscriptions for fan-out patterns:

### Message Subscriptions

```go
// Create multiple message subscriptions
sub1 := mgr.Subscribe("logger", 100)
sub2 := mgr.Subscribe("analytics", 100)
sub3 := mgr.Subscribe("ui-updates", 50)

// Each subscription receives all messages independently
go processLogs(sub1)
go processAnalytics(sub2)
go updateUI(sub3)
```

### State Change Subscriptions

```go
// Create multiple state subscriptions
stateSub1 := mgr.SubscribeStateChange("connection-monitor", 16)
stateSub2 := mgr.SubscribeStateChange("metrics-collector", 16)

// Each subscription receives all state changes independently
go monitorConnection(stateSub1)
go collectMetrics(stateSub2)
```

Each subscription receives a copy of every event, allowing independent processing at different rates.
