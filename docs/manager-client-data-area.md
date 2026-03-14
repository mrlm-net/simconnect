---
title: "Client Data Areas (Manager)"
section: "manager"
order: 6
---

# Client Data Areas (Manager)

The `manager` package exposes the full SimConnect Client Data Area (CDA) API as direct methods on the `Manager` interface. This guide explains how to use CDAs through the manager's lifecycle-managed connection, covering both the reader and writer roles.

> **See also:** [Client Data Areas](client-data-area.md) for the engine-layer reference covering types, constants, and low-level usage.

## Overview

Client data areas are named shared memory regions that SimConnect add-ons use to exchange arbitrary structured data. One add-on creates the area and writes to it; any other connected client can map the same name, define the layout, and subscribe to updates.

Typical use cases:

- A weather injector writes current conditions; a cockpit display reads and renders them.
- A flight data recorder writes position and attitude; a telemetry exporter reads in real time.
- Two independent add-ons share a control channel without a network connection.

The manager wraps the same underlying SimConnect calls as the engine client. The key difference is that each method checks whether the manager is currently connected and returns `ErrNotConnected` if not. There is no automatic retry or queuing — callers are responsible for registering CDAs at the right time in the connection lifecycle.

`MapClientDataNameToID` is also part of the manager interface and is the prerequisite for all CDA operations. Call it first after connection before calling any other CDA method.

## Prerequisites

Before calling any CDA method:

1. The manager must be connected to the simulator. Use `OnConnectionStateChange` or `SubscribeOnOpen` to detect when the connection is ready.
2. Choose define IDs and request IDs in the user range. See [Request and ID Management](manager-requests-ids.md) for the ID ranges. All user IDs must be between 1 and 999,999,849.
3. The `requestID` passed to `RequestClientData` identifies responses in the dispatch loop. It must be unique within your application and within the user range.

## Workflow

The following steps apply to any CDA integration through the manager.

### Step 1 — Map the area name to an ID

Both the writer and the reader call `MapClientDataNameToID` with the same name string. This is what links the two clients together.

```go
const (
    WeatherAreaID uint32 = 1000
    WeatherDefID  uint32 = 1001
    WeatherReqID  uint32 = 1002
)

mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    if new != manager.StateConnected {
        return
    }
    if err := mgr.MapClientDataNameToID("MyAddon.Weather", WeatherAreaID); err != nil {
        log.Printf("MapClientDataNameToID failed: %v", err)
    }
})
```

### Step 2 — Create the area (writer only)

The client that owns the area calls `CreateClientData`. Readers skip this step.

```go
// dwSize must be between 1 and 8192 bytes
if err := mgr.CreateClientData(WeatherAreaID, 16, types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT); err != nil {
    log.Printf("CreateClientData failed: %v", err)
}
```

### Step 3 — Define the struct layout

Call `AddToClientDataDefinition` once per field to describe the memory layout. Each call maps a byte offset and size to a definition ID.

```go
// Field 1: float64 at offset 0 (8 bytes)
mgr.AddToClientDataDefinition(
    WeatherDefID,
    0,                                               // byte offset
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), // typed size constant
    0,                                               // epsilon
    0,                                               // datum ID
)
// Field 2: float64 at offset 8 (8 bytes)
mgr.AddToClientDataDefinition(
    WeatherDefID,
    8,
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64),
    0,
    0,
)
```

### Step 4 — Subscribe to updates (reader)

Call `RequestClientData` to register a periodic delivery subscription. Responses arrive as `SIMCONNECT_RECV_ID_CLIENT_DATA` messages.

```go
if err := mgr.RequestClientData(
    WeatherAreaID,
    WeatherReqID,
    WeatherDefID,
    types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,         // deliver on each write
    types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
    0, 0, 0,
); err != nil {
    log.Printf("RequestClientData failed: %v", err)
}
```

### Step 5 — Receive updates

Use `Subscribe` or `SubscribeWithFilter` to receive `SIMCONNECT_RECV_ID_CLIENT_DATA` messages:

```go
sub := mgr.SubscribeWithFilter("weather-cda", 20, func(msg engine.Message) bool {
    return types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_CLIENT_DATA
})
defer sub.Unsubscribe()

go func() {
    for {
        select {
        case msg := <-sub.Messages():
            cd := msg.AsClientData()
            if cd == nil || cd.DwRequestID != WeatherReqID {
                continue
            }
            weather := engine.CastDataAs[SharedWeather](&cd.DwData)
            log.Printf("Temperature: %.1f°C  Pressure: %.2f hPa",
                weather.TemperatureC, weather.PressureHPa)
        case <-sub.Done():
            return
        }
    }
}()
```

### Step 6 — Write data (writer)

The owning add-on calls `SetClientData` whenever the data changes:

```go
data := SharedWeather{TemperatureC: 15.5, PressureHPa: 1013.25}
if err := mgr.SetClientData(
    WeatherAreaID,
    WeatherDefID,
    0,                               // flags — always 0
    0,                               // dwReserved — must be 0
    uint32(unsafe.Sizeof(data)),
    unsafe.Pointer(&data),
); err != nil {
    log.Printf("SetClientData failed: %v", err)
}
```

### Step 7 — Clean up

When a definition is no longer needed, call `ClearClientDataDefinition` to release it:

```go
if err := mgr.ClearClientDataDefinition(WeatherDefID); err != nil {
    log.Printf("ClearClientDataDefinition failed: %v", err)
}
```

> **Note:** There is no `DeleteClientData` call in the SimConnect SDK. The area itself is released automatically when the owning client disconnects from the simulator.

## Complete Example

The following snippet shows a reader that connects via manager, maps an existing shared area, defines a two-field layout, and prints every update.

```go
//go:build windows

package main

import (
    "errors"
    "log"
    "os"
    "os/signal"
    "unsafe"

    "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

type SharedWeather struct {
    TemperatureC float64 // offset 0, 8 bytes
    PressureHPa  float64 // offset 8, 8 bytes
}

const (
    WeatherAreaID uint32 = 1000
    WeatherDefID  uint32 = 1001
    WeatherReqID  uint32 = 1002
)

func main() {
    mgr := simconnect.New("WeatherReader",
        manager.WithAutoReconnect(true),
    )

    mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
        if new != manager.StateConnected {
            return
        }
        setupCDA(mgr)
    })

    sub := mgr.SubscribeWithFilter("weather-cda", 20, func(msg engine.Message) bool {
        return types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_CLIENT_DATA
    })
    defer sub.Unsubscribe()

    go func() {
        for {
            select {
            case msg := <-sub.Messages():
                cd := msg.AsClientData()
                if cd == nil || cd.DwRequestID != WeatherReqID {
                    continue
                }
                weather := engine.CastDataAs[SharedWeather](&cd.DwData)
                log.Printf("Temperature: %.1f°C  Pressure: %.2f hPa",
                    weather.TemperatureC, weather.PressureHPa)
            case <-sub.Done():
                return
            }
        }
    }()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)
    go func() {
        <-sigChan
        mgr.Stop()
    }()

    if err := mgr.Start(); err != nil {
        log.Printf("Manager stopped: %v", err)
    }
}

func setupCDA(mgr manager.Manager) {
    if err := mgr.MapClientDataNameToID("MyAddon.Weather", WeatherAreaID); err != nil {
        log.Printf("MapClientDataNameToID: %v", err)
        return
    }
    // Reader skips CreateClientData — only the writer calls it.
    if err := mgr.AddToClientDataDefinition(WeatherDefID, 0,
        uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0); err != nil {
        log.Printf("AddToClientDataDefinition [0]: %v", err)
    }
    if err := mgr.AddToClientDataDefinition(WeatherDefID, 8,
        uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0); err != nil {
        log.Printf("AddToClientDataDefinition [8]: %v", err)
    }
    if err := mgr.RequestClientData(
        WeatherAreaID, WeatherReqID, WeatherDefID,
        types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,
        types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
        0, 0, 0,
    ); err != nil {
        if errors.Is(err, manager.ErrNotConnected) {
            log.Println("Not connected, skipping RequestClientData")
        } else {
            log.Printf("RequestClientData: %v", err)
        }
    }
}
```

## Method Reference

| Method | Parameters | Returns | Notes |
|--------|-----------|---------|-------|
| `MapClientDataNameToID` | `clientDataName string, clientDataID uint32` | `error` | Called by both reader and writer; must be called first after connection |
| `CreateClientData` | `clientDataID uint32, dwSize uint32, flags SIMCONNECT_CREATE_CLIENT_DATA_FLAG` | `error` | Writer only; `dwSize` must be 1–8192 bytes |
| `AddToClientDataDefinition` | `defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32` | `error` | Call once per field; use `SIMCONNECT_CLIENTDATATYPE_*` constants for `dwSizeOrType` |
| `RequestClientData` | `clientDataID uint32, requestID uint32, defineID uint32, period SIMCONNECT_CLIENT_DATA_PERIOD, flags SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32` | `error` | Reader only; `requestID` must be in user range |
| `SetClientData` | `clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer` | `error` | Writer only; `flags` and `dwReserved` must both be `0` |
| `ClearClientDataDefinition` | `defineID uint32` | `error` | Removes all field definitions for the given define ID |

`MapClientDataNameToID` is defined directly on the `Manager` interface alongside the CDA methods. All six methods are available without calling `mgr.Client()`.

## ErrNotConnected

Every method in this table returns `manager.ErrNotConnected` if called when the manager has no active connection. This includes the period between startup and the first successful connection, and any reconnection gap when auto-reconnect is enabled.

```go
if err := mgr.CreateClientData(...); err != nil {
    if errors.Is(err, manager.ErrNotConnected) {
        // Not yet connected — defer setup to the OnConnectionStateChange handler
        return
    }
    log.Printf("CreateClientData failed: %v", err)
}
```

The manager does not queue or retry failed calls. Register your CDA setup inside `OnConnectionStateChange` or `SubscribeOnOpen` so it runs automatically on each (re)connection.

## ID Range

The `requestID` parameter in `RequestClientData` must be within the user range: **1 to 999,999,849**. The manager reserves 999,999,850–999,999,999 for internal use.

Use `manager.IsValidUserID(id)` to validate an ID before use:

```go
const WeatherReqID uint32 = 1002

if !manager.IsValidUserID(WeatherReqID) {
    log.Fatal("ID conflicts with manager reserved range")
}
```

See [Request and ID Management](manager-requests-ids.md) for the complete ID allocation table and validation helpers.

## See Also

- [Client Data Areas](client-data-area.md) — Engine-layer reference: types, constants (`SIMCONNECT_CLIENTDATATYPE_*`, `SIMCONNECT_CLIENT_DATA_PERIOD`, `SIMCONNECT_CLIENT_DATA_REQUEST_FLAG`), and complete writer/reader examples
- [Manager Usage](usage-manager.md) — Full manager API reference including subscriptions and connection lifecycle
- [Request and ID Management](manager-requests-ids.md) — ID allocation strategy and conflict prevention
