# All Facilities Example

## Overview

This example demonstrates how to request and read the full set of facilities (airports) from the simulator using a direct request API.

## What It Does

1. Connects to the simulator and waits until the session is open.
2. Issues a `RequestAllFacilities` request for airport data.
3. Receives facility list packets and prints header information and each airport entry (ICAO, region, coordinates, altitude).
4. Handles multi-packet responses and reconnects if needed.

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK (bundled with MSFS)

## Running the Example

```bash
go run examples/all-facilities/main.go
```

## Expected Output

Typical output contains connection notices and per-packet facility information, for example:

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
📡 Received SIMCONNECT_RECV_OPEN message!
🏢 Received facility list:
  📋 Request ID: 1
  📊 Array Size: 200
  📦 Packet: 1 of 3
  ✈️  Airport #1: LKPR (CZE) | 🌍 Lat: 49.01083, Lon: 13.39861 | 📏 Alt: 364.00m
...
```

## Code Explanation

The example uses `simconnect.NewClient` to create a direct engine client and then calls `RequestAllFacilities` with `types.SIMCONNECT_FACILITY_LIST_AIRPORT`. Received messages are read from the client's stream and parsed as `SIMCONNECT_RECV_FACILITIES_LIST` packets. Because the C layout for facility entries may differ from Go's struct packing, the example reads individual fields from the raw message bytes using `unsafe` and prints values reliably. The program implements a reconnect loop so it can tolerate simulator restarts.
