# Airplane Doors Example

A demonstration of controlling aircraft doors using SimConnect events. This example uses the `TOGGLE_AIRCRAFT_EXIT` event to control individual passenger doors on an aircraft.

## Features

- Toggle individual aircraft doors (doors 1-4)
- Interactive command-line interface
- Real-time door control with immediate feedback
- Proper error handling and graceful shutdown

## Usage

```bash
go run main.go
```

**Requirements:**
- Windows OS
- Microsoft Flight Simulator or compatible simulator running
- Aircraft loaded in the simulator

## Commands

- `D1` - Toggle door #1 (front left passenger door)
- `D2` - Toggle door #2 (front right passenger door)
- `D3` - Toggle door #3 (rear left passenger door)
- `D4` - Toggle door #4 (rear right passenger door)
- `H` - Show help
- `Q` - Quit application

## What it demonstrates

- Creating a SimConnect client connection
- Mapping and transmitting events with parameters to the simulator
- Event notification group setup and prioritization
- Interactive command processing in a separate goroutine
- Processing different SimConnect message types (events, exceptions)
- Proper resource cleanup and shutdown handling

## Key Concepts

- **Event Mapping**: Maps client events to simulator events (`TOGGLE_AIRCRAFT_EXIT`)
- **Event Parameters**: Passes door number as parameter to the event
- **Event Groups**: Organizes events into notification groups for prioritization
- **Interactive Control**: Demonstrates user input handling for real-time control
- **Message Processing**: Handles different SimConnect message types in a unified loop

## Code Structure

The example follows this pattern:
1. Initialize SimConnect client
2. Map events for controlling aircraft doors
3. Configure event notification groups
4. Start interactive input handler in a goroutine
5. Process messages in a loop until interrupted
6. Clean shutdown on signal or error

## Note

Since aircraft door states are not available as readable simulation variables, this demo only sends toggle commands. Check the aircraft visually to see the actual door states after sending commands.
