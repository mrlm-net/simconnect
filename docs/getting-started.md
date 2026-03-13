---
title: "Getting Started"
description: "Install the library and write your first Microsoft Flight Simulator connection."
order: 1
section: "client"
---

# Getting Started

This guide walks you through installing the library, connecting to Microsoft Flight Simulator, and reading your first SimVar — from zero to live data in a few minutes.

## Prerequisites

- **Windows only.** SimConnect is a Windows-native DLL. The library will not compile on other platforms.
- **Go 1.25+**
- **Microsoft Flight Simulator 2020 or 2024** installed and running when you test your add-on.
- **SimConnect.dll** — bundled with MSFS. The library auto-detects it from common SDK installation paths. You can also set the `SIMCONNECT_DLL` environment variable or pass `simconnect.ClientWithDLLPath(...)` to override detection.

## Install

```bash
go get github.com/mrlm-net/simconnect
```

## Connect and Disconnect

Every file that imports or calls SimConnect APIs must carry the `//go:build windows` build constraint. The DLL does not exist on other platforms and the Go toolchain will refuse to link without it.

```go
//go:build windows

package main

import (
    "fmt"
    "os"

    "github.com/mrlm-net/simconnect"
)

func main() {
    client := simconnect.NewClient("MyApp")

    if err := client.Connect(); err != nil {
        fmt.Fprintln(os.Stderr, "connect error:", err)
        os.Exit(1)
    }
    defer client.Disconnect()

    // Wait for the open acknowledgement before doing any work.
    for msg := range client.Stream() {
        if open := msg.AsOpen(); open != nil {
            fmt.Println("connected to SimConnect")
            break
        }
    }
}
```

`simconnect.NewClient` accepts the application name that MSFS displays in its connection list. `Connect()` loads the DLL and opens a named-pipe channel to the simulator. `Disconnect()` cancels all in-flight work and closes the connection — the `defer` form is the correct pattern.

## Read a SimVar

Reading data from the simulator requires three steps: define the data structure, request it, and consume it from the message stream.

### Step 1 — Define the data

`AddToDataDefinition` binds a SimConnect variable name and unit to a definition ID. Call it once per field; the order of calls determines the field order in the struct you will cast the response into.

```go
//go:build windows

package main

import (
    "fmt"
    "os"

    "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

// IDs can be any uint32 in the range 1–999,999,899.
const (
    PositionDefID uint32 = 1000
    PositionReqID uint32 = 1001
)

// AircraftPosition must match the definition order and types exactly.
type AircraftPosition struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}

func main() {
    client := simconnect.NewClient("MyApp")

    if err := client.Connect(); err != nil {
        fmt.Fprintln(os.Stderr, "connect error:", err)
        os.Exit(1)
    }
    defer client.Disconnect()

    // Register the three variables we want to read.
    client.AddToDataDefinition(PositionDefID, "PLANE LATITUDE",  "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    client.AddToDataDefinition(PositionDefID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    client.AddToDataDefinition(PositionDefID, "PLANE ALTITUDE",  "feet",    types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)

    // Request the data once for the user aircraft.
    client.RequestDataOnSimObject(
        PositionReqID,
        PositionDefID,
        types.SIMCONNECT_OBJECT_ID_USER,
        types.SIMCONNECT_PERIOD_ONCE,
        types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
        0, 0, 0,
    )

    // Read messages until we receive the response.
    for msg := range client.Stream() {
        switch types.SIMCONNECT_RECV_ID(msg.DwID) {

        case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
            data := msg.AsSimObjectData()
            if data.DwRequestID != PositionReqID {
                continue
            }
            pos := engine.CastDataAs[AircraftPosition](&data.DwData)
            fmt.Printf("lat=%.4f lon=%.4f alt=%.0fft\n",
                pos.Latitude, pos.Longitude, pos.Altitude)
            return

        case types.SIMCONNECT_RECV_ID_EXCEPTION:
            ex := msg.AsException()
            fmt.Fprintf(os.Stderr, "simconnect exception: %d\n", ex.DwException)
            return
        }
    }
}
```

### How the cast works

`AsSimObjectData()` reinterprets the raw message bytes as `SIMCONNECT_RECV_SIMOBJECT_DATA`. The actual payload sits at `data.DwData`. `CastDataAs[T]` performs an unsafe pointer cast from that field to your struct — no allocation, no copy.

The struct layout must match the definition order and types exactly:

| `AddToDataDefinition` type | Go field type |
|---|---|
| `SIMCONNECT_DATATYPE_FLOAT64` | `float64` |
| `SIMCONNECT_DATATYPE_FLOAT32` | `float32` |
| `SIMCONNECT_DATATYPE_INT32` | `int32` |
| `SIMCONNECT_DATATYPE_INT64` | `int64` |
| `SIMCONNECT_DATATYPE_STRING256` | `[256]byte` |

Use `engine.BytesToString(field[:])` to convert a fixed-size byte array to a Go string.

> **Note:** `SIMCONNECT_PERIOD_ONCE` delivers a single response and stops. For continuous updates, use `SIMCONNECT_PERIOD_SECOND` or `SIMCONNECT_PERIOD_SIM_FRAME` with `SIMCONNECT_DATA_REQUEST_FLAG_CHANGED` to receive data only when a value changes.

## Next Steps

- [Configuration](config-client.md) — DLL path, heartbeat frequency, buffer size, logging
- [Engine/Client Usage](usage-client.md) — Full API reference: events, facilities, AI traffic, flight plans
- [Datasets](usage-datasets.md) — Pre-built variable sets for aircraft position, engine state, weather, and more
- [Manager](config-manager.md) — Production-ready wrapper with auto-reconnect and structured state tracking
