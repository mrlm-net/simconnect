# Await Connection Example

## Overview

This example demonstrates how to establish a connection to Microsoft Flight Simulator using the SimConnect SDK with automatic retry logic and graceful shutdown handling.

## What It Does

1. **Creates a cancellable context** - Sets up context for graceful shutdown on Ctrl+C
2. **Retries connection** - Continuously attempts to connect until the simulator is running
3. **Streams messages** - Listens for incoming SimConnect messages
4. **Handles disconnection** - Properly handles stream closure when simulator exits
5. **Graceful shutdown** - Responds to interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
go run examples/await-connection/main.go
```

## Expected Output

```
⏳ Waiting for simulator to start...
🔄 Connection attempt failed: ..., retrying in 2 seconds...
✅ Connected to SimConnect, listening for messages...
ℹ️  (Press Ctrl+C to exit)
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📨 Message received - ID: 1, Size: 304 bytes
...
🛑 Received interrupt signal, shutting down...
🔌 Context cancelled, disconnecting...
👋 Disconnected from SimConnect
```

## Code Explanation

The example uses a retry loop to wait for the simulator to start, then streams messages from SimConnect. It handles graceful shutdown via context cancellation when the user presses Ctrl+C, and properly closes the connection when the stream ends.
