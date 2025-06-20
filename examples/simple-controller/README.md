# Simple Controller Example

A basic example demonstrating how to monitor and control aircraft external power using SimConnect events and data requests.

## Features

- Monitor external power state via simulation variable
- Toggle external power using SimConnect events
- Real-time display of power status changes
- Proper error handling and graceful shutdown

## Usage

```bash
go run main.go
```

**Requirements:**
- Windows OS
- Microsoft Flight Simulator or compatible simulator running
- Aircraft loaded in the simulator

## What it demonstrates

- Creating a SimConnect client connection
- Setting up data definitions for simulation variables
- Requesting periodic data updates
- Mapping and transmitting events to the simulator
- Processing different message types (data, events, exceptions)
- Proper resource cleanup and shutdown handling

## Key Concepts

- **Data Definitions**: Defines what simulation data to monitor (`EXTERNAL POWER ON`)
- **Event Mapping**: Maps client events to simulator events (`TOGGLE_EXTERNAL_POWER`)
- **Event Groups**: Organizes events into notification groups for prioritization
- **Message Processing**: Handles different SimConnect message types in a unified loop

## Code Structure

The example follows this pattern:
1. Initialize SimConnect client
2. Set up data definitions for monitoring external power state
3. Map events for controlling external power
4. Configure event notification groups
5. Process messages in a loop until interrupted
6. Clean shutdown on signal or error
