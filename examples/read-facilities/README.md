# Read Facilities Example

## Overview

This example demonstrates how to retrieve lists of facilities from Microsoft Flight Simulator using SimConnect's facility list API. It showcases how to request facility lists, handle paginated responses, and parse facility data with proper memory alignment handling.

## What It Does

1. **Auto-reconnection** - Continuously attempts to connect to the simulator with retry logic
2. **Requests facility list** - Retrieves a list of airports using `RequestFacilitiesListEX1`
3. **Handles paginated responses** - Processes facility list messages that may be split across multiple packets
4. **Dynamic memory alignment handling** - Detects and adapts to different struct packing/alignment (33, 36, or 40 bytes)
5. **Parses facility data** - Extracts ICAO code, region, latitude, longitude, and altitude for each facility
6. **Handles reconnection** - Automatically reconnects if the simulator disconnects
7. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- The simulator can be at any state (facility data is available regardless of loaded scenery)

## Running the Example

```bash
cd examples/read-facilities
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
  SimConnect Build: 62651.0
📨 Message received - SIMCONNECT_RECV_ID_AIRPORT_LIST
🏢 Received facility list:
  📋 Request ID: 2000
  📊 Array Size: 50
  📦 Packet: 1 of 3
  Actual entry size: 36 bytes
  ✈️  Airport #1: LKPR (CZ) | 🌍 Lat: 50.100833, Lon: 14.260000 | 📏 Alt: 1247.38m
  ✈️  Airport #2: LKVO (CZ) | 🌍 Lat: 49.151667, Lon: 17.438889 | 📏 Alt: 794.50m
  ✈️  Airport #3: LKTB (CZ) | 🌍 Lat: 49.151250, Lon: 16.694444 | 📏 Alt: 528.17m
  ...
📨 Message received - SIMCONNECT_RECV_ID_AIRPORT_LIST
🏢 Received facility list:
  📋 Request ID: 2000
  📊 Array Size: 50
  📦 Packet: 2 of 3
  ...
```

## Code Explanation

### Requesting Facility List

```go
client.RequestFacilitiesListEX1(2000, types.SIMCONNECT_FACILITY_LIST_AIRPORT)
```

Parameters:
- **2000** - User Request ID (used to identify responses)
- **SIMCONNECT_FACILITY_LIST_AIRPORT** - Facility type to request

Available facility types:
- `SIMCONNECT_FACILITY_LIST_AIRPORT`
- `SIMCONNECT_FACILITY_LIST_WAYPOINT`
- `SIMCONNECT_FACILITY_LIST_NDB`
- `SIMCONNECT_FACILITY_LIST_VOR`

### Message Structure

**SIMCONNECT_RECV_ID_AIRPORT_LIST**:
```go
msg := msg.AsAirportList()
// Contains:
// - dwRequestID: The request ID you specified (2000)
// - dwArraySize: Number of entries in this message
// - dwEntryNumber: Which packet this is (1-based)
// - dwOutOf: Total number of packets
```

### Memory Alignment Handling

**Critical Implementation Detail**: The facility list entries have different sizes depending on compiler alignment:

- **33 bytes** - Packed (1-byte alignment)
  - Ident(6) + Region(3) + Lat(8) + Lon(8) + Alt(8) = 33
- **36 bytes** - 4-byte alignment (most common)
  - Ident(6) + Region(3) + Padding(3) + Lat(8) + Lon(8) + Alt(8) = 36
- **40 bytes** - 8-byte alignment
  - Ident(6) + Region(3) + Padding(7) + Lat(8) + Lon(8) + Alt(8) = 40

The code dynamically calculates the entry size:

```go
headerSize := types.DWORD(28) // SIMCONNECT_RECV_FACILITIES_LIST header
actualDataSize := msg.DwSize - headerSize
actualEntrySize := actualDataSize / types.DWORD(list.DwArraySize)
```

Then determines field offsets based on the detected size:

```go
switch actualEntrySize {
case 33: // Packed
    latOffset, lonOffset, altOffset = 9, 17, 25
case 36: // 4-byte alignment
    latOffset, lonOffset, altOffset = 12, 20, 28
case 40: // 8-byte alignment
    latOffset, lonOffset, altOffset = 16, 24, 32
}
```

### Parsing Facility Entries

The code uses `unsafe.Pointer` to manually read facility data at calculated offsets:

```go
dataStart := unsafe.Pointer(uintptr(unsafe.Pointer(list)) + uintptr(headerSize))

for i := uint32(0); i < uint32(list.DwArraySize); i++ {
    entryOffset := uintptr(i) * uintptr(actualEntrySize)
    entryPtr := unsafe.Pointer(uintptr(dataStart) + entryOffset)
    
    // Read fixed fields (Ident at offset 0, Region at offset 6)
    // Read variable fields (Lat/Lon/Alt at calculated offsets)
}
```

### Pagination Handling

Facility lists may be split across multiple messages when there are many results:

```go
fmt.Printf("  dwEntryNumber: %d\n", list.DwEntryNumber)  // Current packet
fmt.Printf("  dwOutOf: %d\n", list.DwOutOf)              // Total packets
```

You should continue processing messages until `dwEntryNumber == dwOutOf`.

### Connection Lifecycle

The `runConnection()` function:
1. Connects with retry logic until simulator is available
2. Requests facility list immediately after connection
3. Processes messages in a loop, handling facility list responses
4. Returns `nil` on disconnect (triggers reconnection) or error on cancellation

## Use Cases

This pattern demonstrates:
- Building comprehensive facility databases from simulator data
- Creating navigation tools with facility search capabilities
- Analyzing scenery coverage and facility distribution
- Building airport/waypoint/navaid browsers
- Exporting simulator facility data for external tools
- Validating flight plans against available facilities

## Important Notes

- **No facility definition needed** - Unlike `RequestFacilityData`, list requests don't require `AddToFacilityDefinition`
- **Paginated responses** - Large facility lists may span multiple messages
- **Memory alignment varies** - Always calculate actual entry size from message data
- **Raw data parsing** - Use `unsafe.Pointer` carefully to read facility entries
- **Simulator must be running** - But doesn't need a loaded flight
- **All facilities available** - Not limited to facilities near aircraft position

## Comparison with RequestFacilityData

| Feature | RequestFacilitiesListEX1 | RequestFacilityData |
|---------|-------------------------|---------------------|
| Purpose | Get list of facilities | Get detailed facility data |
| Definition Required | No | Yes (AddToFacilityDefinition) |
| Data Returned | Basic info (ICAO, region, position) | Customizable fields |
| Response Type | AIRPORT_LIST/WAYPOINT_LIST/etc. | FACILITY_DATA |
| Pagination | Yes (multiple messages) | No (single facility) |
| Use Case | Discovery, browsing | Detailed lookup |

## Facility List Types

The example uses airports, but other facility types are available:

- **Airports** - `SIMCONNECT_FACILITY_LIST_AIRPORT` → `SIMCONNECT_RECV_ID_AIRPORT_LIST`
- **Waypoints** - `SIMCONNECT_FACILITY_LIST_WAYPOINT` → `SIMCONNECT_RECV_ID_WAYPOINT_LIST`
- **NDBs** - `SIMCONNECT_FACILITY_LIST_NDB` → `SIMCONNECT_RECV_ID_NDB_LIST`
- **VORs** - `SIMCONNECT_FACILITY_LIST_VOR` → `SIMCONNECT_RECV_ID_VOR_LIST`

Each type returns similar structure but may have type-specific fields in the future.

## Memory Safety Considerations

This example uses `unsafe.Pointer` extensively for performance and to handle varying memory layouts. In production code, consider:

- Validating message sizes before parsing
- Adding bounds checking for array access
- Handling unknown entry sizes gracefully
- Adding comprehensive error handling for malformed data

## Further Reading

- [SimConnect_RequestFacilitiesListEx1 docs](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilitiesListEx1.htm)
- [SimConnect_RequestFacilityData docs](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilityData.htm)
- Read Facility Example (for detailed facility data requests)
