# Subscribe Events Example

## Overview

This example demonstrates how to subscribe to and handle SimConnect system events from Microsoft Flight Simulator. It shows event subscription patterns for monitoring simulator state changes like pause, sim start/stop, and sound toggle.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Subscribes to system events** - Monitors Pause, Sim, and Sound events
3. **Processes event messages** - Handles `SIMCONNECT_RECV_ID_EVENT` messages with state information
4. **Handles reconnection** - Automatically reconnects if the simulator disconnects
5. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
cd examples/subscribe-events
go run main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
📨 Message received - SIMCONNECT_RECV_ID_OPEN
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator'
  Application Version: 1.0
  Application Build: 1.0
  SimConnect Version: 12.0
  SimConnect Build: 61259.0
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1001, Data: 1
  🏁 Simulator SIM STARTED
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1000, Data: 1
  ⏸️  Simulator is PAUSED
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1000, Data: 0
  ▶️  Simulator is UNPAUSED
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1002, Data: 0
  🔇 Simulator SOUND OFF
📨 Message received - SIMCONNECT_RECV_ID_EVENT
  Event ID: 1002, Data: 1
  🔊 Simulator SOUND ON
📴 Stream closed (simulator disconnected)
⏳ Waiting 5 seconds before reconnecting...
```

## Code Explanation

### System Event Subscriptions

The example subscribes to three system events:

```go
client.SubscribeToSystemEvent(1000, "Pause")  // Pause/unpause events
client.SubscribeToSystemEvent(1001, "Sim")    // Sim start/stop events
client.SubscribeToSystemEvent(1002, "Sound")  // Sound on/off events
```

Each subscription assigns a unique event ID that will be returned in the `UEventID` field of received event messages.

### The Events This Example Subscribes To

| Event ID | Event Name | Description |
|----------|------------|-------------|
| 1000 | `Pause` | Simulator paused/unpaused (0=unpaused, 1=paused) |
| 1001 | `Sim` | Simulation started/stopped (0=stopped, 1=started) |
| 1002 | `Sound` | Master sound toggled (0=off, 1=on) |

SimConnect has more system events (timers, aircraft/flight loaded, crashes and others); see [Events Lifecycle](../../docs/events-lifecycle.md) and the SDK's `SubscribeToSystemEvent` reference.

### Event Processing

Events are received as `SIMCONNECT_RECV_ID_EVENT` messages:

```go
case types.SIMCONNECT_RECV_ID_EVENT:
    eventMsg := msg.AsEvent()
    fmt.Printf("  Event ID: %d, Data: %d\n", eventMsg.UEventID, eventMsg.DwData)
    
    // Check event ID and handle accordingly
    if eventMsg.UEventID == 1000 {
        if eventMsg.DwData == 1 {
            fmt.Println("  ⏸️  Simulator is PAUSED")
        } else {
            fmt.Println("  ▶️  Simulator is UNPAUSED")
        }
    }
```

The `UEventID` field contains the ID you assigned during subscription, and `DwData` contains the event state (typically 0 or 1 for binary states).

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic
2. Sets up event subscriptions
3. Processes messages in a loop
4. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

## Use Cases

This pattern is useful for:
- Monitoring simulator state changes
- Building event-driven applications
- Triggering actions based on simulator events
- Creating automation tools that respond to pause/unpause
- Implementing flight logging that tracks simulation sessions
- Building tools that respond to aircraft/flight loading events

## Notes

- Event IDs must be unique within your application
- Event IDs must match between subscription and handling
- System events provide binary state (0/1) in the `dwData` field
- String events (like `AircraftLoaded`) return the filename in a different message format
- Events are delivered asynchronously via the message stream
- Unsubscribe from events using `client.UnsubscribeFromSystemEvent(eventID)` when no longer needed
