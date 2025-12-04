# Lifecycle Connection Example

## Overview

This example demonstrates a robust connection lifecycle management pattern for Microsoft Flight Simulator using the SimConnect SDK. It automatically reconnects when the simulator disconnects and handles graceful shutdown.

## What It Does

1. **Creates a cancellable context** - Sets up context for graceful shutdown on Ctrl+C
2. **Manages connection lifecycle** - Encapsulates connection logic in a reusable function
3. **Auto-reconnects** - Automatically reconnects when the simulator disconnects
4. **Retries on failure** - Continuously attempts to connect until the simulator is running
5. **Streams messages** - Listens for incoming SimConnect messages
6. **Graceful shutdown** - Responds to interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
go run examples/lifecycle-connection/main.go
```

## Expected Output

```
ℹ️  (Press Ctrl+C to exit)
⏳ Waiting for simulator to start...
🔄 Connection attempt failed: ..., retrying in 2 seconds...
✅ Connected to SimConnect, listening for messages...
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📨 Message received - ID: 1, Size: 304 bytes
...
📴 Stream closed (simulator disconnected)
⏳ Waiting 5 seconds before reconnecting...
🔄 Attempting to reconnect...
⏳ Waiting for simulator to start...
...
🛑 Received interrupt signal, shutting down...
🔌 Context cancelled, disconnecting...
⚠️  Connection ended: context canceled
```

## Code Explanation

The example implements a reconnection loop pattern using the `runConnection` function that handles a single connection lifecycle. When the simulator disconnects (stream closes), it returns `nil` to signal that reconnection should be attempted. When the context is cancelled (Ctrl+C), it returns the context error to signal a complete shutdown. This pattern is ideal for long-running applications that need to maintain a persistent connection to the simulator.
