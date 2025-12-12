````markdown
# Using Datasets Example

## Overview

This example demonstrates how to register and use a SimConnect dataset to read structured aircraft data from the simulator using the Go wrapper library.

## What It Does

1. **Initializes a SimConnect client** with context and retry logic.
2. **Registers a dataset** definition for aircraft using the `traffic` dataset helper.
3. **Requests aircraft data** by object type (aircraft) and periodically refreshes the request.
4. **Parses dataset payloads** into a Go struct (`AircraftData`) and prints selected fields.
5. **Shows how to spawn AI traffic** from a JSON config (parked and enroute examples).

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK available/installed
- Go 1.18+ and the repository checked out

## Files

- `main.go` — Example application that registers the dataset, requests data and processes incoming messages.
- `planes.json` — Optional JSON file used to define AI traffic to spawn (create this file locally if you want to spawn aircraft).

## Configuration

The example may read a `planes.json` file in the current working directory to create parked and/or enroute AI aircraft. Use the same format as other AI traffic examples (see `examples/ai-traffic/README.md`). A minimal `planes.json` looks like:

```json
[
	{ "airport": "LKPR", "plane": "FSLTL A320 VLG Vueling", "number": "N12345" }
]
```

## Running the Example

Run from the repository root:

```bash
go run examples/using-datasets/main.go
```

The program continuously attempts to connect to the simulator, registers the dataset, and prints dataset fields when `SIMOBJECT_DATA_BYTYPE` messages arrive. Press `Ctrl+C` to stop and disconnect cleanly.

## Expected Output

When connected you will see messages like:

```
⏳ Waiting for simulator to start...
✅ Connected to SimConnect, listening for messages...
✈️  Ready for plane spotting???
📨 Message received -  SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE
		 Aircraft Title: Boeing 747-8i Asobo, Category: Airplane, Livery Name: ..., Lat: 49.0123, Lon: 12.3456, Alt: 1234.000000, ...
```

If you provide a `planes.json` file the example will create parked or enroute AI aircraft as defined.

## Code Notes

- The dataset shape is mapped to the `AircraftData` struct in `main.go`. Helper methods convert fixed-length byte arrays into Go strings.
- The example uses `traffic.NewAircraftDataset("AircraftDataset", 3000)` and registers it via `client.RegisterDataset(...)`.
- Periodic refreshes are implemented with a `time.Ticker` that calls `client.RequestDataOnSimObjectType(...)` every 5 seconds.
- The message processing loop reads from `client.Stream()` and switches on `types.SIMCONNECT_RECV_ID(msg.DwID)` to handle dataset messages.

## Tips & Troubleshooting

- Ensure aircraft titles in `planes.json` exactly match installed aircraft titles in MSFS.
- Flight plan files (if used) must be valid `.pln` files and accessible from the working directory.
- Adjust the data request radius (third parameter to `RequestDataOnSimObjectType`) to tune which aircraft are returned.

````

