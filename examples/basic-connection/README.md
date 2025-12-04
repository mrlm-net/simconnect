# Basic Connection Example

## Overview

This example demonstrates the simplest way to establish a connection to Microsoft Flight Simulator using the SimConnect SDK through the Go wrapper library.

## What It Does

1. **Creates a SimConnect client** - Initializes a new client with an application name
2. **Connects to the simulator** - Establishes a connection to a running instance of MSFS
3. **Maintains connection** - Keeps the connection alive for 2 seconds
4. **Gracefully disconnects** - Properly closes the connection on exit

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
go run examples/basic-connection/main.go
```

## Expected Output

```
✅ Connected to SimConnect...
⏳ Sleeping for 2 seconds...
✈️  Ready for takeoff!
👋 Disconnected from SimConnect...
```

## Code Explanation

The example uses a deferred function to ensure the connection is always properly closed, even if an error occurs during execution.
