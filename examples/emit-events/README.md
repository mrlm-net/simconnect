# Emit Events Example

## Overview

This example demonstrates how to trigger simulator events in Microsoft Flight Simulator using the SimConnect SDK. It shows an **interactive door toggle system** where you can enter door/exit indices and toggle them in real-time. The example illustrates event mapping, notification groups, and event transmission to control aircraft doors.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Maps client events** - Maps custom event IDs to SimConnect key events (`TOGGLE_AIRCRAFT_EXIT`)
3. **Creates notification groups** - Organizes events into groups for better management
4. **Sets group priority** - Assigns priority levels to notification groups
5. **Interactive user input** - Prompts for door indices and toggles them on Enter key
6. **Real-time event transmission** - Sends events immediately to open/close aircraft doors
7. **Input validation** - Validates user input and provides helpful error messages
8. **Handles reconnection** - Automatically reconnects if the simulator disconnects
9. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

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

🚪 Interactive Door Toggle
──────────────────────────────────────────────
Enter a door/exit index number and press Enter to toggle it.
Common indices: 1 (main door), 2-25 (additional doors/exits)
Note: Valid door indices vary by aircraft model.
Press Ctrl+C to exit.
──────────────────────────────────────────────

Door index: 1
✅ Toggled door/exit index 1

Door index: 3
✅ Toggled door/exit index 3

Door index: abc
❌ Invalid input 'abc'. Please enter a positive integer.

Door index: 
```

## Code Explanation

### Mapping Client Events

```go
client.MapClientEventToSimEvent(2010, "TOGGLE_AIRCRAFT_EXIT")
```

This maps a custom client event ID (2010) to the SimConnect key event `TOGGLE_AIRCRAFT_EXIT`, which opens/closes aircraft doors/exits.

### Creating Notification Groups

```go
client.AddClientEventToNotificationGroup(3000, 2010, false)
```

Adds the client event (2010) to notification group (3000). The `false` parameter indicates the event should not be masked.

### Setting Priority

```go
client.SetNotificationGroupPriority(3000, 1000)
```

Sets the priority of the notification group to 1000 (higher values = higher priority).

### Interactive Input Loop

After connection is established, a goroutine reads user input:

```go
scanner := bufio.NewScanner(os.Stdin)
for {
    fmt.Print("Door index: ")
    scanner.Scan()
    input := strings.TrimSpace(scanner.Text())
    
    doorIndex, err := strconv.ParseUint(input, 10, 32)
    if err != nil {
        fmt.Printf("❌ Invalid input. Please enter a positive integer.\n")
        continue
    }
    
    client.TransmitClientEvent(
        types.SIMCONNECT_OBJECT_ID_USER,
        2010,
        uint32(doorIndex),
        3000,
        0,
    )
}
```

### Transmitting Events

```go
client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, 2010, doorIndex, 3000, 0)
```

Transmits the event to the simulator:
- `SIMCONNECT_OBJECT_ID_USER` - Targets the user's aircraft
- `2010` - The client event ID (mapped to TOGGLE_AIRCRAFT_EXIT)
- `doorIndex` - The door/exit index number (1, 2, 3, etc.) entered by user
- `3000` - The notification group
- `0` - Flags (no special flags)

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic (2-second intervals)
2. Maps events and sets up notification groups
3. Waits for `SIMCONNECT_RECV_ID_OPEN` to confirm connection is ready
4. Starts interactive input goroutine for user commands
5. Processes incoming messages from the simulator
6. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

### Input Validation

The example validates user input to ensure safe event transmission:
- **Empty input** - Ignored, prompts again
- **Non-numeric input** - Shows error message and prompts again
- **Zero or negative** - Rejects with error (door indices must be positive)
- **Valid integer** - Transmits event with the specified door index

### Door Indices

Different aircraft have different door/exit configurations:
- **Index 1** - Usually the main passenger door
- **Index 2-10** - Additional passenger doors, cargo doors, emergency exits
- **Index 11-25+** - Specialized exits, service doors, or aircraft-specific exits

The simulator will only respond to valid indices for the current aircraft. Invalid indices are safely ignored by the simulator. Larger aircraft (like the A380 or 747) may have 15+ doors/exits, while smaller aircraft typically have 2-4.

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
2. `SIMCONNECT_RECV_ID_OPEN` message received, confirming connection is ready
3. Interactive prompt appears, waiting for user input
4. User enters door index and presses Enter
5. Event is transmitted to the simulator
6. Simulator processes the event and toggles the specified door
7. Application continues listening for messages and user input
8. Process repeats for each user input until Ctrl+C is pressed

## Notes

- **Interactive Mode** - This example uses stdin for interactive input, making it a great demonstration of real-time event control
- **Immediate Execution** - Events are transmitted and executed immediately when you press Enter
- **Aircraft State** - Make sure the simulator has an aircraft loaded to see door animations
- **Door Indices** - Valid indices depend on the aircraft model; experiment with different numbers to discover available doors
- **Context Awareness** - The input goroutine monitors the context for cancellation, ensuring clean shutdown
- **Error Handling** - Invalid inputs are caught and reported without crashing the application
- **Reconnection** - If the simulator disconnects, the app will reconnect and prompt for input again

## Tips

- Try different door indices (1-25) to discover all doors on your aircraft
- Some aircraft have cargo doors, passenger doors, and emergency exits with different indices
- Larger aircraft (A380, 747) may have 15-25+ doors/exits to discover
- Watch the simulator window to see doors opening/closing in real-time
- Press the same index twice to toggle a door closed then open again