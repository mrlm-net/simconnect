# Emit Events Example

## Overview

This example demonstrates how to trigger simulator events in Microsoft Flight Simulator using the SimConnect SDK. It shows how to map client events to simulator events, organize them into notification groups, and transmit events to control the aircraft or simulator state.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Maps client events** - Maps custom event IDs to SimConnect key events (e.g., `TOGGLE_AIRCRAFT_EXIT`)
3. **Creates notification groups** - Organizes events into groups for better management
4. **Sets group priority** - Assigns priority levels to notification groups
5. **Transmits events** - Sends events to the simulator to trigger actions (e.g., opening/closing aircraft door)
6. **Handles reconnection** - Automatically reconnects if the simulator disconnects
7. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see event effects)

## Running the Example

```bash
cd examples/emit-events
go run main.go
```

## Expected Output

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
ℹ️  (Press Ctrl+C to exit)
📨 Message received - SIMCONNECT_RECV_ID_OPEN
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  Application Name: 'Microsoft Flight Simulator'
  Application Version: 1.0
  Application Build: 1.0
  SimConnect Version: 12.0
  SimConnect Build: 61259.0
📨 Message received - [other message types]
```

## Code Explanation

### Mapping Client Events

```go
client.MapClientEventToSimEvent(2001, "TOGGLE_AIRCRAFT_EXIT")
```

This maps a custom client event ID (2001) to the SimConnect key event `TOGGLE_AIRCRAFT_EXIT`, which opens/closes the aircraft door.

### Creating Notification Groups

```go
client.AddClientEventToNotificationGroup(3000, 2001, false)
```

Adds the client event (2001) to notification group (3000). The `false` parameter indicates the event should not be masked.

### Setting Priority

```go
client.SetNotificationGroupPriority(3000, 1)
```

Sets the priority of the notification group to 1 (higher priority).

### Transmitting Events

```go
client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, 2001, 1, 3000, 0)
```

Transmits the event to the simulator:
- `SIMCONNECT_OBJECT_ID_USER` - Targets the user's aircraft
- `2001` - The client event ID
- `1` - Event data/parameter value
- `3000` - The notification group
- `0` - Flags

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic (2-second intervals)
2. Maps events and sets up notification groups
3. Transmits the event
4. Processes incoming messages
5. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

### Available Key Events

Common SimConnect key events you can use:
- `TOGGLE_AIRCRAFT_EXIT` - Open/close aircraft door
- `TOGGLE_MASTER_BATTERY` - Toggle battery master switch
- `TOGGLE_MASTER_ALTERNATOR` - Toggle alternator
- `TOGGLE_BEACON_LIGHTS` - Toggle beacon lights
- `TOGGLE_NAV_LIGHTS` - Toggle navigation lights
- `TOGGLE_LOGO_LIGHTS` - Toggle logo lights
- `TOGGLE_TAXI_LIGHTS` - Toggle taxi lights
- `TOGGLE_LANDING_LIGHTS` - Toggle landing lights
- `PARKING_BRAKES` - Toggle parking brake
- `GEAR_TOGGLE` - Toggle landing gear

For a full list, refer to the SimConnect SDK documentation.

## Message Flow

1. Application connects to SimConnect
2. `SIMCONNECT_RECV_ID_OPEN` message received
3. Events are mapped and transmitted
4. Simulator processes the event and executes the action
5. Application continues listening for messages

## Notes

- Events are executed immediately when transmitted
- Make sure the simulator is in a state where the event makes sense (e.g., aircraft loaded for door toggle)
- Multiple events can be mapped and organized into different notification groups
- Event parameters can be used to pass additional data (e.g., setting specific values)