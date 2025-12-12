# SimConnect Subscribe Example

## Overview

This example demonstrates how to use the **Subscribe pattern** for receiving messages from the Manager interface. Instead of using the `OnMessage` callback, this example shows how to create a channel-based subscription for message handling, which is useful for isolating message processing in separate goroutines or implementing fan-out patterns.

## What It Does

1. **Channel-based message delivery** - Uses `Subscribe()` to receive messages via Go channels
2. **Automatic connection management** - Manager handles connect/disconnect lifecycle automatically
3. **State-based setup** - Registers data definitions and subscriptions when connection becomes available
4. **Automatic reconnection** - Reconnects automatically if the simulator disconnects or restarts
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
cd examples/simconnect-subscribe
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
📬 Message subscription started, waiting for messages...
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
^C
🛑 Received interrupt signal, shutting down...
📭 Subscription channel closed
👋 Goodbye!
```

## Code Explanation

### Subscribe vs OnMessage

This example uses the **Subscribe pattern** instead of the `OnMessage` callback:

| Feature | Subscribe (`mgr.Subscribe`) | OnMessage (`mgr.OnMessage`) |
|---------|----------------------------|----------------------------|
| Message delivery | Via Go channel | Via callback function |
| Concurrency | Consumer controls processing | Callback runs in dispatcher |
| Multiple consumers | ✅ Multiple subscriptions | ❌ Single callback |
| Backpressure | Channel buffering | None (callback must return) |
| Use case | Fan-out, isolated processing | Simple single consumer |

### Creating a Subscription

```go
// Create a subscription with ID and buffer size
sub := mgr.Subscribe("main-subscriber", 256)
```

Parameters:
- First parameter: Subscription ID (use empty string `""` for auto-generated UUID)
- Second parameter: Channel buffer size for message buffering

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

### Cleanup

Always call `Unsubscribe()` when done to release resources:

```go
// Unsubscribe when done (cleanup)
sub.Unsubscribe()
```

### State Change Handler

The state change handler works the same as in the Manager example:

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
    }
})
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

## When to Use Subscribe vs OnMessage

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

## Multiple Subscriptions

You can create multiple subscriptions for fan-out patterns:

```go
// Create multiple subscriptions
sub1 := mgr.Subscribe("logger", 100)
sub2 := mgr.Subscribe("analytics", 100)
sub3 := mgr.Subscribe("ui-updates", 50)

// Each subscription receives all messages independently
go processLogs(sub1)
go processAnalytics(sub2)
go updateUI(sub3)
```

Each subscription receives a copy of every message, allowing independent processing at different rates.
