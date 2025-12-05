# Read Messages Example

## Overview

This example demonstrates how to read and process SimConnect messages from Microsoft Flight Simulator, including subscribing to system events and handling specific message types.

## What It Does

1. **Creates a cancellable context** - Sets up context for graceful shutdown on Ctrl+C
2. **Manages connection lifecycle** - Encapsulates connection logic in a reusable function
3. **Auto-reconnects** - Automatically reconnects when the simulator disconnects
4. **Subscribes to system events** - Demonstrates subscribing to the Pause event
5. **Processes messages** - Shows how to handle different message types
6. **Graceful shutdown** - Responds to interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
go run examples/read-messages/main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
🔄 Connection attempt failed: ..., retrying in 2 seconds...
✅ Connected to SimConnect, listening for messages...
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📨 Message received - ID: 1, Size: 304 bytes
📨 Message received - ID: 5, Size: 24 bytes
   Event ID: 1000, Data: 1
   >> Simulator is PAUSED
📨 Message received - ID: 5, Size: 24 bytes
   Event ID: 1000, Data: 0
   >> Simulator is UNPAUSED
...
🛑 Received interrupt signal, shutting down...
🔌 Context cancelled, disconnecting...
⚠️  Connection ended: context canceled
```

## Code Explanation

The example implements a reconnection loop pattern using the `runConnection` function that handles a single connection lifecycle. It subscribes to the `Pause` system event and demonstrates how to process incoming messages by type using a switch statement on `SIMCONNECT_RECV_ID`. When a Pause event is received, it displays whether the simulator is paused or unpaused. This pattern is ideal for applications that need to react to simulator state changes.
