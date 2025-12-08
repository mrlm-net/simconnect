# Read Objects Example

## Overview

This example demonstrates how to enumerate and retrieve information about available aircraft and liveries in Microsoft Flight Simulator using the SimConnect SDK. It shows how to handle the `EnumerateSimObjectsAndLiveries` API and parse the resulting data structures.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Enumerates aircraft** - Requests a list of all aircraft models and liveries available in the simulator
3. **Parses enumeration data** - Processes raw binary data to extract aircraft titles and livery names
4. **Filters results** - Demonstrates filtering entries (example: FSLTL prefix for AI traffic aircraft)
5. **Handles reconnection** - Automatically reconnects if the simulator disconnects
6. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed

## Running the Example

```bash
cd examples/read-objects
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
📨 Message received - SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST
  Enumerated 512 entries (request ID: 1000, entry number: 1, out of: 2)
  Message size: 262176 bytes
  Actual entry size: 512 bytes

  Entry #1:
    Title: FSLTL A320 Air France
    Livery: Air France

  Entry #2:
    Title: FSLTL B737 Ryanair
    Livery: Ryanair

  Summary: 150 entries with data, 362 empty entries
```

## Code Explanation

### Enumeration Request

```go
client.EnumerateSimObjectsAndLiveries(1000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

This requests an enumeration of all aircraft objects in the simulator with request ID 1000.

### Data Structure

Each enumeration entry is a `SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY` structure containing:
- `AircraftTitle` - The aircraft model title (e.g., "Boeing 747-8i")
- `LiveryName` - The livery/skin name (e.g., "British Airways")

### Parsing Binary Data

The example demonstrates low-level parsing of SimConnect message data:

```go
headerSize := types.DWORD(32)
actualDataSize := msg.DwSize - headerSize
actualEntrySize := actualDataSize / types.DWORD(enumMsg.DwArraySize)

dataStart := unsafe.Pointer(uintptr(unsafe.Pointer(enumMsg)) + uintptr(headerSize))

for i := uint32(0); i < uint32(enumMsg.DwArraySize); i++ {
    offset := uintptr(i) * uintptr(actualEntrySize)
    entryPtr := unsafe.Pointer(uintptr(dataStart) + offset)
    entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(entryPtr)
    
    title := engine.BytesToString(entry.AircraftTitle[:])
    livery := engine.BytesToString(entry.LiveryName[:])
}
```

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic
2. Sends enumeration request
3. Processes incoming messages
4. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

### Filtering

The example includes filtering logic to focus on specific aircraft:

```go
if strings.HasPrefix(title, "FSLTL") || strings.HasPrefix(livery, "FSLTL") {
    validEntries++
    fmt.Printf("    Title: %s\n", title)
    fmt.Printf("    Livery: %s\n", livery)
}
```

This is useful for:
- Finding AI traffic aircraft (FSLTL models)
- Locating specific airline liveries
- Building aircraft selection menus
- Validating aircraft availability before spawning AI traffic

## Use Cases

This pattern can be used to:
- Build aircraft/livery selection UIs
- Validate aircraft titles before spawning AI traffic
- Generate reports of installed aircraft
- Filter and search available models
- Verify addon aircraft installation
- Create dynamic aircraft spawn systems

## Notes

- Enumeration may return multiple message chunks for large aircraft collections
- Empty entries are common and should be filtered
- Aircraft titles must exactly match for AI spawning operations
- The `DwOutOf` field indicates total number of message chunks
- Byte arrays need conversion using `BytesToString()` helper function
- Use `unsafe.Pointer` for low-level binary data parsing
