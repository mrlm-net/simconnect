# Subscribe Facilities Example

## Overview

This example shows how to subscribe to facility data streams (airport list) from the simulator and handle live facility list updates.

## What It Does

1. Connects to the simulator and waits until the session is open.
2. Subscribes to facility updates using `SubscribeToFacilities` for airport data.
3. Listens on the client's message stream and decodes incoming facility list packets.
4. Prints header information and each airport entry (ICAO, region, coordinates, altitude).
5. Runs a reconnect loop to tolerate simulator disconnects.

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK (bundled with MSFS)

## Running the Example

```bash
go run examples/subscribe-facilities/main.go
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

The example uses `simconnect.NewClient` and calls `SubscribeToFacilities` with `types.SIMCONNECT_FACILITY_LIST_AIRPORT`. Messages are consumed from the client's stream and parsed as facility list packets. The example demonstrates reading raw facility entries using `unsafe` to extract packed C-structured fields (ICAO, region, lat/lon/alt) and shows a robust reconnect loop so the example keeps working if the simulator restarts.
