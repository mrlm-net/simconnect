---
title: "Client Data Areas"
description: "Using SimConnect Client Data Areas for inter-add-on shared memory communication."
order: 6
section: "client"
---

# Client Data Areas

Client data areas are named shared memory regions that SimConnect clients use to exchange arbitrary data with each other. One add-on creates and writes the area; any other connected client can subscribe to it and receive updates. This is the standard SimConnect mechanism for inter-add-on communication — for example, a weather injector writing current conditions that a cockpit display reads.

> **See also:** [Engine/Client Usage](usage-client.md) for the full client API reference.

## Overview

A client data area has four properties:

- **Name** — a unique string identifier visible to all SimConnect clients on the same session.
- **ID** — a numeric handle your client assigns when mapping the name.
- **Size** — the total byte size of the area, between 1 and 8192 bytes.
- **Access** — read/write (default) or read-only (only the creator can write).

Data within an area is structured using *definitions* — the same define-ID pattern used by `AddToDataDefinition`. You can map multiple fields at explicit byte offsets, or use the `SIMCONNECT_CLIENTDATATYPE_*` constants to let SimConnect handle sizing automatically.

## Workflow

Setting up a client data area follows four steps:

1. **Map name to ID** — associate a human-readable name with a numeric client data ID.
2. **Create the area** — register the area size and access flags with SimConnect.
3. **Define fields** — describe the layout of data within the area.
4. **Request or write data** — subscribe to updates with `RequestClientData`, or write data with `SetClientData`.

A *reader* performs steps 1, 3, and 4 (request). A *writer* performs all four steps, using `SetClientData` instead of `RequestClientData`.

## API Reference

### MapClientDataNameToID

Maps a string name to a numeric client data ID. Both the reader and the writer must call this with the same name to refer to the same area.

```go
err := client.MapClientDataNameToID("MyAddon.SharedData", clientDataID)
```

**Signature:** `MapClientDataNameToID(clientDataName string, clientDataID uint32) error`

| Parameter | Type | Description |
|-----------|------|-------------|
| `clientDataName` | `string` | Unique name for the area, visible to all SimConnect clients |
| `clientDataID` | `uint32` | Numeric ID your client will use to reference this area |

### CreateClientData

Registers the area with SimConnect. Only the owning client calls this; readers skip it.

```go
err := client.CreateClientData(clientDataID, 16, types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT)
```

**Signature:** `CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error`

| Parameter | Type | Description |
|-----------|------|-------------|
| `clientDataID` | `uint32` | ID previously mapped with `MapClientDataNameToID` |
| `dwSize` | `uint32` | Size of the area in bytes; must be between 1 and 8192 |
| `flags` | `SIMCONNECT_CREATE_CLIENT_DATA_FLAG` | `DEFAULT` (read/write) or `READ_ONLY` (only creator can write) |

### AddToClientDataDefinition

Adds a data field to a definition for this area. Call multiple times to build a composite layout.

```go
err := client.AddToClientDataDefinition(
    defineID,
    0,                                             // byte offset within area
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), // 8 bytes at offset 0
    0,                                             // epsilon (change threshold)
    0,                                             // datum ID
)
```

**Signature:** `AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error`

| Parameter | Type | Description |
|-----------|------|-------------|
| `defineID` | `uint32` | Definition ID used when requesting or setting data |
| `dwOffset` | `uint32` | Byte offset within the area where this field starts |
| `dwSizeOrType` | `uint32` | Either a byte count (e.g. `8`) or a `SIMCONNECT_CLIENTDATATYPE_*` constant cast to `uint32` |
| `epsilon` | `float32` | Minimum change threshold before an `ON_SET` notification fires; use `0` to notify on any change |
| `datumID` | `uint32` | User-defined datum ID for tagged-mode reads; use `0` for default mode |

> **Note:** `dwSizeOrType` accepts either an explicit byte count or one of the typed `SIMCONNECT_CLIENTDATATYPE_*` constants. The constants use values in the `0xFFFFFFFA`–`0xFFFFFFFF` range, which SimConnect distinguishes from byte counts. Always cast the constant to `uint32` when passing it.

### RequestClientData

Subscribes to updates for the defined fields. Use this on the reader side.

```go
err := client.RequestClientData(
    clientDataID,
    requestID,
    defineID,
    types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,
    types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
    0, 0, 0,
)
```

**Signature:** `RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error`

| Parameter | Type | Description |
|-----------|------|-------------|
| `clientDataID` | `uint32` | Area to subscribe to |
| `requestID` | `uint32` | ID used to identify the response in the dispatch loop |
| `defineID` | `uint32` | Definition describing the fields to receive |
| `period` | `SIMCONNECT_CLIENT_DATA_PERIOD` | How often to receive data (see [Periods](#simconnect_client_data_period)) |
| `flags` | `SIMCONNECT_CLIENT_DATA_REQUEST_FLAG` | Delivery mode (see [Request Flags](#simconnect_client_data_request_flag)) |
| `origin` | `uint32` | Number of periods to skip before first delivery; use `0` |
| `interval` | `uint32` | Number of periods between deliveries; use `0` for every period |
| `limit` | `uint32` | Maximum number of deliveries; use `0` for unlimited |

### SetClientData

Writes data to a client data area. Use this on the writer side.

```go
value := MyStruct{Temperature: 15.5, Pressure: 1013.25}
err := client.SetClientData(
    clientDataID,
    defineID,
    0,              // flags — plain uint32, always 0
    0,              // dwReserved — must be 0
    uint32(unsafe.Sizeof(value)),
    unsafe.Pointer(&value),
)
```

**Signature:** `SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error`

| Parameter | Type | Description |
|-----------|------|-------------|
| `clientDataID` | `uint32` | Area to write to |
| `defineID` | `uint32` | Definition describing the fields being written |
| `flags` | `uint32` | Plain `uint32`; SimConnect does not define a typed enum for this parameter — pass `0` |
| `dwReserved` | `uint32` | Reserved; must always be `0` |
| `cbUnitSize` | `uint32` | Size of the data being written in bytes |
| `data` | `unsafe.Pointer` | Pointer to the data struct |

### message.AsClientData

Casts an incoming `Message` to `*types.SIMCONNECT_RECV_CLIENT_DATA`. Returns `nil` if the message is not a client data notification.

```go
case types.SIMCONNECT_RECV_ID_CLIENT_DATA:
    cd := msg.AsClientData()
    if cd != nil && cd.DwRequestID == myRequestID {
        value := engine.CastDataAs[MyStruct](&cd.DwData)
    }
```

`SIMCONNECT_RECV_CLIENT_DATA` embeds `SIMCONNECT_RECV_SIMOBJECT_DATA`, so the same `DwRequestID`, `DwDefineID`, and `DwData` fields are available.

## Types Reference

### SIMCONNECT_CREATE_CLIENT_DATA_FLAG

Controls area access when calling `CreateClientData`.

| Constant | Value | Description |
|----------|-------|-------------|
| `SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT` | `0` | Area is readable and writable by any client |
| `SIMCONNECT_CREATE_CLIENT_DATA_FLAG_READ_ONLY` | `1` | Only the client that created the area can write to it |

### SIMCONNECT_CLIENT_DATA_PERIOD

Controls how frequently `RequestClientData` delivers updates.

| Constant | Description |
|----------|-------------|
| `SIMCONNECT_CLIENT_DATA_PERIOD_NEVER` | No automatic delivery; use `ONCE` to trigger a one-shot request |
| `SIMCONNECT_CLIENT_DATA_PERIOD_ONCE` | Deliver once immediately, then stop |
| `SIMCONNECT_CLIENT_DATA_PERIOD_VISUAL_FRAME` | Deliver once per rendered frame |
| `SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET` | Deliver whenever the data is updated by the writer |
| `SIMCONNECT_CLIENT_DATA_PERIOD_SECOND` | Deliver once per second |

### SIMCONNECT_CLIENT_DATA_REQUEST_FLAG

Controls the delivery mode for `RequestClientData`.

| Constant | Value | Description |
|----------|-------|-------------|
| `SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT` | `0` | Deliver every time the period fires |
| `SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_CHANGED` | `1` | Deliver only when the value has changed since the last delivery |
| `SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_TAGGED` | `2` | Deliver data in tagged format using datum IDs |

### SIMCONNECT_CLIENTDATATYPE

Typed size constants for `AddToClientDataDefinition`. Use these instead of a raw byte count when you want SimConnect to infer the field size from its type.

| Constant | Value | Go type to use in struct |
|----------|-------|--------------------------|
| `SIMCONNECT_CLIENTDATATYPE_INT8` | `0xFFFFFFFF` | `int8` |
| `SIMCONNECT_CLIENTDATATYPE_INT16` | `0xFFFFFFFE` | `int16` |
| `SIMCONNECT_CLIENTDATATYPE_INT32` | `0xFFFFFFFD` | `int32` |
| `SIMCONNECT_CLIENTDATATYPE_INT64` | `0xFFFFFFFC` | `int64` |
| `SIMCONNECT_CLIENTDATATYPE_FLOAT32` | `0xFFFFFFFB` | `float32` |
| `SIMCONNECT_CLIENTDATATYPE_FLOAT64` | `0xFFFFFFFA` | `float64` |

## Complete Example

The example below shows a reader client that maps an existing shared data area, defines a two-field layout, and receives updates whenever the writer changes the data.

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SharedWeather matches the layout written by the owning add-on.
type SharedWeather struct {
	TemperatureC float64 // offset 0, 8 bytes
	PressureHPa  float64 // offset 8, 8 bytes
}

const (
	WeatherAreaID  = 1000
	WeatherDefID   = 1001
	WeatherReqID   = 1002
)

func main() {
	client := engine.New("WeatherReader", engine.WithAutoDetect())

	if err := client.Connect(); err != nil {
		panic(err)
	}
	defer client.Disconnect()

	// Step 1: map the area name to a local ID
	if err := client.MapClientDataNameToID("MyAddon.Weather", WeatherAreaID); err != nil {
		panic(err)
	}

	// Step 2: (writer only) CreateClientData — skipped by this reader

	// Step 3: define the two fields at explicit offsets
	client.AddToClientDataDefinition(
		WeatherDefID,
		0,                                              // offset 0
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), // 8 bytes
		0, 0,
	)
	client.AddToClientDataDefinition(
		WeatherDefID,
		8,                                              // offset 8
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64),
		0, 0,
	)

	// Step 4: subscribe — deliver whenever the writer calls SetClientData
	if err := client.RequestClientData(
		WeatherAreaID,
		WeatherReqID,
		WeatherDefID,
		types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,
		types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
		0, 0, 0,
	); err != nil {
		panic(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	stream := client.Stream()
	for {
		select {
		case <-sigChan:
			fmt.Println("Shutting down...")
			return
		case msg, ok := <-stream:
			if !ok {
				return
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_CLIENT_DATA:
				cd := msg.AsClientData()
				if cd == nil || cd.DwRequestID != WeatherReqID {
					continue
				}
				weather := engine.CastDataAs[SharedWeather](&cd.DwData)
				fmt.Printf("Temperature: %.1f°C  Pressure: %.1f hPa\n",
					weather.TemperatureC, weather.PressureHPa)
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				ex := msg.AsException()
				if ex != nil {
					fmt.Printf("SimConnect exception: %d\n", ex.DwException)
				}
			}
		}
	}
}
```

### Writer Side

To write the same area from the owning add-on, call `CreateClientData` after mapping the name, then use `SetClientData` whenever the data changes:

```go
// Writer: create and populate the area
client.MapClientDataNameToID("MyAddon.Weather", WeatherAreaID)
client.CreateClientData(WeatherAreaID, uint32(unsafe.Sizeof(SharedWeather{})),
    types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT)

client.AddToClientDataDefinition(WeatherDefID, 0,
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)
client.AddToClientDataDefinition(WeatherDefID, 8,
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)

data := SharedWeather{TemperatureC: 15.5, PressureHPa: 1013.25}
client.SetClientData(WeatherAreaID, WeatherDefID, 0, 0,
    uint32(unsafe.Sizeof(data)), unsafe.Pointer(&data))
```

## Notes

- **Size limit.** `dwSize` passed to `CreateClientData` must be between 1 and 8192 bytes. SimConnect returns an HRESULT error for values outside this range.
- **dwReserved.** The `dwReserved` parameter of `SetClientData` is not optional — it must always be `0`. Passing any other value is undefined behaviour.
- **flags in SetClientData.** SimConnect does not define a typed enum for the `flags` parameter of `SetClientData` (see source comment in `pkg/engine/clientdata.go`). Always pass `0`.
- **Area lifetime.** Client data areas persist for the duration of the SimConnect session. There is no `DeleteClientData` call — the area is released automatically when the owning client disconnects.
- **READ_ONLY enforcement.** If a client data area is created with `SIMCONNECT_CREATE_CLIENT_DATA_FLAG_READ_ONLY`, only the client that called `CreateClientData` can write to it. Any other client that calls `SetClientData` on the area will receive a SimConnect exception.
- **Name uniqueness.** Area names are global within the simulator process. Use a reverse-DNS style prefix (e.g., `"com.mycompany.myaddon.channel"`) to avoid collisions with other add-ons.
- **Struct alignment.** Go may insert padding bytes in structs that does not exist in the SimConnect wire format. When mapping multi-field layouts, always verify field offsets match what you pass to `AddToClientDataDefinition`. Using `SIMCONNECT_CLIENTDATATYPE_*` constants with explicit offsets is safer than relying on `unsafe.Sizeof` for individual fields.

## See Also

- [Engine/Client Usage](usage-client.md) — Full client API reference including data definitions and sim object data
- [Manager Usage](usage-manager.md) — Automatic connection lifecycle management
- [Examples](../examples) — Working code samples
