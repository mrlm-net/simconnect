---
title: "Engine API Reference"
description: "Extended Engine client API: Client Data Areas, Input Events, and object enumeration."
order: 5
section: "client"
---

# Engine API Reference

This document covers Engine client APIs added after the core reference was written. It is a companion to [Engine/Client Usage](usage-client.md), which covers connection lifecycle, data definitions, sim object requests, events, AI traffic, and facilities. Read that document first before using this one.

The APIs documented here are:

- **Client Data Area** — named shared memory regions for inter-add-on communication
- **Input Event API** — read and write MSFS 2024 input events by hash (MSFS 2024 only)
- **SimObject and Livery Enumeration** — list available aircraft models and their liveries

---

## Client Data Area API

Client data areas are named shared memory regions that SimConnect clients use to exchange arbitrary binary data with each other. One add-on creates and owns the area; any other connected client can subscribe to updates. This is the standard SimConnect mechanism for inter-add-on communication.

> **See also:** [Client Data Areas](client-data-area.md) for a full conceptual guide, field-by-field parameter tables, types reference, and a complete annotated example. This section summarises the API surface and provides a self-contained writer + reader example.

### Setup workflow

A client data area is set up in four steps:

1. **Map name to ID** — both writer and reader call `MapClientDataNameToID` with the same string name.
2. **Create the area** — the writer calls `CreateClientData` to register the area size and access flags. Readers skip this step.
3. **Define fields** — both sides call `AddToClientDataDefinition` to describe the data layout.
4. **Exchange data** — the writer calls `SetClientData` to write; the reader calls `RequestClientData` to subscribe.

### MapClientDataNameToID

Associates a human-readable name with a numeric client data ID. The name must match on both sides.

```go
err := client.MapClientDataNameToID("com.myaddon.channel", clientDataID)
```

**Signature:** `MapClientDataNameToID(clientDataName string, clientDataID uint32) error`

### CreateClientData

Registers the area with SimConnect. Call only from the owning (writer) client.

```go
err := client.CreateClientData(
    clientDataID,
    16,                                                   // size in bytes, max 8192
    types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT,
)
```

**Signature:** `CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error`

| Flag constant | Value | Meaning |
|---|---|---|
| `SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT` | `0` | Any client can write |
| `SIMCONNECT_CREATE_CLIENT_DATA_FLAG_READ_ONLY` | `1` | Only the creator can write |

The maximum area size is **8192 bytes**. SimConnect returns an HRESULT error for any value outside the range 1–8192.

### AddToClientDataDefinition

Adds a data field to a definition. Call multiple times to build composite layouts. Both the writer and the reader call this with the same `defineID` and offsets.

```go
err := client.AddToClientDataDefinition(
    defineID,
    0,                                               // byte offset within the area
    uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), // 8-byte float64
    0,                                               // epsilon: 0 = notify on any change
    0,                                               // datum ID: 0 for default mode
)
```

**Signature:** `AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error`

The `dwSizeOrType` parameter accepts either a raw byte count or one of the `SIMCONNECT_CLIENTDATATYPE_*` typed constants. The constants occupy the `0xFFFFFFFA`–`0xFFFFFFFF` range; SimConnect distinguishes them from byte counts. Always cast the constant to `uint32`.

| Constant | Go type |
|---|---|
| `SIMCONNECT_CLIENTDATATYPE_INT8` | `int8` |
| `SIMCONNECT_CLIENTDATATYPE_INT16` | `int16` |
| `SIMCONNECT_CLIENTDATATYPE_INT32` | `int32` |
| `SIMCONNECT_CLIENTDATATYPE_INT64` | `int64` |
| `SIMCONNECT_CLIENTDATATYPE_FLOAT32` | `float32` |
| `SIMCONNECT_CLIENTDATATYPE_FLOAT64` | `float64` |

### RequestClientData

Subscribes to updates. Use this on the reader side.

```go
err := client.RequestClientData(
    clientDataID,
    requestID,
    defineID,
    types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,
    types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
    0, 0, 0, // origin, interval, limit — 0 for defaults
)
```

**Signature:** `RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error`

Incoming data arrives as `SIMCONNECT_RECV_ID_CLIENT_DATA`. Cast with `msg.AsClientData()` and read the payload with `engine.CastDataAs`.

### SetClientData

Writes data to the area. Use this on the writer side.

```go
value := MyStruct{Field1: 15.5, Field2: 1013.25}
err := client.SetClientData(
    clientDataID,
    defineID,
    0,                            // flags — always 0; SimConnect defines no enum for this
    0,                            // dwReserved — must be 0
    uint32(unsafe.Sizeof(value)),
    unsafe.Pointer(&value),
)
```

**Signature:** `SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error`

> **Note:** The `flags` parameter is a plain `uint32`. SimConnect does not define a typed enum for it. Always pass `0`. The `dwReserved` parameter must also always be `0`.

### Complete example

The example below shows both sides of a shared data channel. The writer creates and populates a 16-byte area; the reader subscribes and prints values when they change.

```go
//go:build windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SharedData is the layout both sides agree on.
// 16 bytes total: two float64 fields at explicit offsets 0 and 8.
type SharedData struct {
	TemperatureC float64 // offset 0
	PressureHPa  float64 // offset 8
}

const (
	AreaID  uint32 = 1000
	DefID   uint32 = 1001
	ReqID   uint32 = 1002
)

// writerSetup creates and writes the area.
// Call this from the add-on that owns the data.
func writerSetup(client *engine.Engine) {
	client.MapClientDataNameToID("com.example.weather", AreaID)
	client.CreateClientData(AreaID, uint32(unsafe.Sizeof(SharedData{})),
		types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT)

	client.AddToClientDataDefinition(DefID, 0,
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)
	client.AddToClientDataDefinition(DefID, 8,
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)

	data := SharedData{TemperatureC: 15.5, PressureHPa: 1013.25}
	client.SetClientData(AreaID, DefID, 0, 0,
		uint32(unsafe.Sizeof(data)), unsafe.Pointer(&data))
}

// readerSetup subscribes to the area.
// Call this from the add-on that reads the data.
func readerSetup(client *engine.Engine) {
	client.MapClientDataNameToID("com.example.weather", AreaID)

	client.AddToClientDataDefinition(DefID, 0,
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)
	client.AddToClientDataDefinition(DefID, 8,
		uint32(types.SIMCONNECT_CLIENTDATATYPE_FLOAT64), 0, 0)

	client.RequestClientData(
		AreaID, ReqID, DefID,
		types.SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET,
		types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG_DEFAULT,
		0, 0, 0,
	)
}

func main() {
	client := engine.New("WeatherReader")
	if err := client.Connect(); err != nil {
		panic(err)
	}
	defer client.Disconnect()

	readerSetup(client)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	for {
		select {
		case <-sigChan:
			return
		case msg, ok := <-client.Stream():
			if !ok {
				return
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_CLIENT_DATA:
				cd := msg.AsClientData()
				if cd == nil || cd.DwRequestID != ReqID {
					continue
				}
				data := engine.CastDataAs[SharedData](&cd.DwData)
				fmt.Printf("Temperature: %.1f°C  Pressure: %.1f hPa\n",
					data.TemperatureC, data.PressureHPa)
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				if ex := msg.AsException(); ex != nil {
					fmt.Printf("SimConnect exception: %d\n", ex.DwException)
				}
			}
		}
	}
}
```

---

## Input Event API (MSFS 2024 only)

> **MSFS 2024 exclusive.** The Input Event API requires Microsoft Flight Simulator 2024. All functions in this section will return HRESULT errors or produce undefined behaviour when called against MSFS 2020.

Input events are named simulator actions identified by a 64-bit hash. They replace the legacy key event model for cockpit and aircraft interactions in MSFS 2024. You query available events by name, then read or write their values using the hash.

Input events carry one of two value types, identified by `SIMCONNECT_INPUT_EVENT_TYPE`:

| Constant | Meaning |
|---|---|
| `SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE` | Numeric value; read and written as `float64` |
| `SIMCONNECT_INPUT_EVENT_TYPE_STRING` | Text value; up to 259 bytes, null-terminated |

### EnumerateInputEvents

Requests a list of all available input events. Responses arrive as one or more `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` messages; cast with `msg.AsEnumerateInputEvents()`.

```go
err := client.EnumerateInputEvents(requestID)
```

**Signature:** `EnumerateInputEvents(requestID uint32) error`

Each response message contains a `RgData` array of `SIMCONNECT_INPUT_EVENT_DESCRIPTOR` elements:

```go
if recv := msg.AsEnumerateInputEvents(); recv != nil {
    // DwArraySize tells you how many descriptors are in this batch
    count := recv.DwArraySize
    _ = count
    // Access descriptors at &recv.RgData[0] through &recv.RgData[count-1]
    name := engine.BytesToString(recv.RgData[0].Name[:])
    fmt.Println("Event:", name)
}
```

`SIMCONNECT_INPUT_EVENT_DESCRIPTOR` fields:

| Field | Type | Description |
|---|---|---|
| `Name` | `[64]byte` | Human-readable event name, null-terminated |
| `Hash` | `DWORD` | 32-bit hash (use `GetInputEvent`/`SetInputEvent` with the full `uint64` hash from subscription) |
| `Type` | `SIMCONNECT_DATATYPE` | Data type of the event value |
| `NodeNames` | `[1024]byte` | Associated node names, null-separated |

### GetInputEvent

Requests the current value of a single input event identified by its 64-bit hash. The response arrives as `SIMCONNECT_RECV_ID_GET_INPUT_EVENT`; cast with `msg.AsGetInputEvent()`.

```go
err := client.GetInputEvent(requestID, hash)
```

**Signature:** `GetInputEvent(requestID uint32, hash uint64) error`

Read the response value using the engine helpers:

```go
if recv := msg.AsGetInputEvent(); recv != nil && recv.RequestID == requestID {
    if f, ok := engine.InputEventValueAsFloat64(recv); ok {
        fmt.Printf("Value: %f\n", f)
    }
    if s, ok := engine.InputEventValueAsString(recv); ok {
        fmt.Printf("Value: %s\n", s)
    }
}
```

| Helper | Signature | Returns |
|---|---|---|
| `InputEventValueAsFloat64` | `(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (float64, bool)` | Value and `true` when type is `DOUBLE`; `(0, false)` otherwise |
| `InputEventValueAsString` | `(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (string, bool)` | Value and `true` when type is `STRING`; `("", false)` otherwise |

### SetInputEvent

Writes a value to an input event. Two typed variants exist:

```go
// Write a numeric value
err := client.SetInputEventDouble(hash, 1.0)

// Write a string value (truncated to 259 bytes if longer)
err := client.SetInputEventString(hash, "some-value")
```

**Signatures:**

- `SetInputEventDouble(hash uint64, value float64) error`
- `SetInputEventString(hash uint64, value string) error`

There is no response message for `SetInputEvent`. HRESULT errors are returned directly.

### SubscribeInputEvent

Subscribes to change notifications for a specific input event identified by hash. Notifications arrive as `SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT` messages; cast with `msg.AsSubscribeInputEvent()`.

```go
err := client.SubscribeInputEvent(hash)
```

**Signature:** `SubscribeInputEvent(hash uint64) error`

Read subscription notifications using the engine helpers:

```go
if recv := msg.AsSubscribeInputEvent(); recv != nil {
    eventHash := engine.SubscribeInputEventHash(recv)
    if f, ok := engine.SubscribeInputEventValueAsFloat64(recv); ok {
        fmt.Printf("Hash %d changed to %f\n", eventHash, f)
    }
    if s, ok := engine.SubscribeInputEventValueAsString(recv); ok {
        fmt.Printf("Hash %d changed to %s\n", eventHash, s)
    }
}
```

> **Note:** `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.HashBytes` is stored as `[8]byte` rather than `uint64` to avoid Go alignment padding at wire offset 12. Use `engine.SubscribeInputEventHash(recv)` rather than reading `HashBytes` directly.

| Helper | Signature | Returns |
|---|---|---|
| `SubscribeInputEventHash` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) uint64` | Event hash |
| `SubscribeInputEventValueAsFloat64` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (float64, bool)` | Value and `true` when type is `DOUBLE` |
| `SubscribeInputEventValueAsString` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (string, bool)` | Value and `true` when type is `STRING` |

### UnsubscribeInputEvent

Cancels a previous subscription.

```go
err := client.UnsubscribeInputEvent(hash)
```

**Signature:** `UnsubscribeInputEvent(hash uint64) error`

### Input Event example

```go
//go:build windows

package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const EnumReqID uint32 = 2000

func main() {
	client := engine.New("InputEventDemo")
	if err := client.Connect(); err != nil {
		panic(err)
	}
	defer client.Disconnect()

	// Step 1: enumerate all available input events
	if err := client.EnumerateInputEvents(EnumReqID); err != nil {
		panic(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	var subscribedHash uint64

	for {
		select {
		case <-sigChan:
			if subscribedHash != 0 {
				client.UnsubscribeInputEvent(subscribedHash)
			}
			return
		case msg, ok := <-client.Stream():
			if !ok {
				return
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {

			case types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS:
				recv := msg.AsEnumerateInputEvents()
				if recv == nil {
					continue
				}
				name := engine.BytesToString(recv.RgData[0].Name[:])
				fmt.Printf("Found input event: %s\n", name)
				// Subscribe to the first event found
				if subscribedHash == 0 {
					// hash stored as uint32 in descriptor; cast for subscribe call
					subscribedHash = uint64(recv.RgData[0].Hash)
					client.SubscribeInputEvent(subscribedHash)
				}

			case types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT:
				recv := msg.AsSubscribeInputEvent()
				if recv == nil {
					continue
				}
				hash := engine.SubscribeInputEventHash(recv)
				if f, ok := engine.SubscribeInputEventValueAsFloat64(recv); ok {
					fmt.Printf("Event hash %d changed: %f\n", hash, f)
				}
				if s, ok := engine.SubscribeInputEventValueAsString(recv); ok {
					fmt.Printf("Event hash %d changed: %s\n", hash, s)
				}
			}
		}
	}
}
```

---

## SimObject and Livery Enumeration

`EnumerateSimObjectsAndLiveries` requests a list of available aircraft models (or other object types) and their installed liveries. This is useful for building traffic injection tools or aircraft selection UIs that need to know what models the simulator has loaded.

### EnumerateSimObjectsAndLiveries

```go
err := client.EnumerateSimObjectsAndLiveries(requestID, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

**Signature:** `EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error`

| `objectType` constant | Description |
|---|---|
| `SIMCONNECT_SIMOBJECT_TYPE_USER` | The user aircraft |
| `SIMCONNECT_SIMOBJECT_TYPE_ALL` | All object types |
| `SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT` | Fixed-wing aircraft |
| `SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER` | Helicopters and rotorcraft |
| `SIMCONNECT_SIMOBJECT_TYPE_BOAT` | Boats |
| `SIMCONNECT_SIMOBJECT_TYPE_GROUND` | Ground vehicles |

Responses arrive as one or more `SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST` messages. Cast each message with `msg.AsSimObjectAndLiveryEnumeration()`.

### AsSimObjectAndLiveryEnumeration

```go
if recv := msg.AsSimObjectAndLiveryEnumeration(); recv != nil {
    for _, item := range recv.RgData {
        title := engine.BytesToString(item.AircraftTitle[:])
        livery := engine.BytesToString(item.LiveryName[:])
        fmt.Printf("Model: %s  Livery: %s\n", title, livery)
    }
}
```

`SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST` embeds `SIMCONNECT_RECV_LIST_TEMPLATE`, which provides pagination fields:

| Field | Type | Description |
|---|---|---|
| `DwRequestID` | `DWORD` | Matches the `requestID` passed to `EnumerateSimObjectsAndLiveries` |
| `DwArraySize` | `DWORD` | Number of entries in this response batch |
| `DwEntryNumber` | `DWORD` | Zero-based index of the first entry in this batch |
| `DwOutOf` | `DWORD` | Total number of entries across all batches |
| `RgData` | `[]SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY` | Entry slice for this batch |

Each `SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY` element has two fields:

| Field | Type | Description |
|---|---|---|
| `AircraftTitle` | `[256]byte` | Container title (matches the title in `aircraft.cfg`) |
| `LiveryName` | `[256]byte` | Livery name, or empty if the model has no separate liveries |

Use `engine.BytesToString` to convert the null-terminated byte arrays to Go strings.

### Enumeration example

```go
//go:build windows

package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const EnumReqID uint32 = 3000

func main() {
	client := engine.New("LiveryEnum")
	if err := client.Connect(); err != nil {
		panic(err)
	}
	defer client.Disconnect()

	if err := client.EnumerateSimObjectsAndLiveries(
		EnumReqID,
		types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT,
	); err != nil {
		panic(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	for {
		select {
		case <-sigChan:
			return
		case msg, ok := <-client.Stream():
			if !ok {
				return
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
				continue
			}
			recv := msg.AsSimObjectAndLiveryEnumeration()
			if recv == nil || recv.DwRequestID != EnumReqID {
				continue
			}
			for _, item := range recv.RgData {
				title := engine.BytesToString(item.AircraftTitle[:])
				livery := engine.BytesToString(item.LiveryName[:])
				fmt.Printf("%-60s  %s\n", title, livery)
			}
			// Stop after receiving the last batch
			if recv.DwEntryNumber+recv.DwArraySize >= recv.DwOutOf {
				fmt.Printf("Total: %d entries\n", recv.DwOutOf)
				return
			}
		}
	}
}
```

---

## Message Helpers Reference

Every incoming `Message` from `client.Stream()` carries a `DwID` field identifying the message type. The helper methods on `Message` cast the raw pointer to a typed struct. All helpers return `nil` when `DwID` does not match the expected `SIMCONNECT_RECV_ID`.

| Method | `SIMCONNECT_RECV_ID` checked | Return type |
|---|---|---|
| `AsOpen()` | `SIMCONNECT_RECV_ID_OPEN` | `*types.SIMCONNECT_RECV_OPEN` |
| `AsException()` | `SIMCONNECT_RECV_ID_EXCEPTION` | `*types.SIMCONNECT_RECV_EXCEPTION` |
| `AsEvent()` | `SIMCONNECT_RECV_ID_EVENT` | `*types.SIMCONNECT_RECV_EVENT` |
| `AsEventFrame()` | `SIMCONNECT_RECV_ID_EVENT_FRAME` | `*types.SIMCONNECT_RECV_EVENT_FRAME` |
| `AsEventFilename()` | `SIMCONNECT_RECV_ID_EVENT_FILENAME` | `*types.SIMCONNECT_RECV_EVENT_FILENAME` |
| `AsEventObjectAddRemove()` | `SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE` | `*types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE` |
| `AsSimObjectData()` | `SIMCONNECT_RECV_ID_SIMOBJECT_DATA` | `*types.SIMCONNECT_RECV_SIMOBJECT_DATA` |
| `AsSimObjectDataBType()` | `SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE` | `*types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE` |
| `AsClientData()` | `SIMCONNECT_RECV_ID_CLIENT_DATA` | `*types.SIMCONNECT_RECV_CLIENT_DATA` |
| `AsAssignedObjectID()` | `SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID` | `*types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID` |
| `AsFacilityData()` | `SIMCONNECT_RECV_ID_FACILITY_DATA` | `*types.SIMCONNECT_RECV_FACILITY_DATA` |
| `AsFacilityDataEnd()` | `SIMCONNECT_RECV_ID_FACILITY_DATA_END` | `*types.SIMCONNECT_RECV_FACILITY_DATA_END` |
| `AsFacilityList()` | `AIRPORT_LIST`, `VOR_LIST`, `NDB_LIST`, or `WAYPOINT_LIST` | `*types.SIMCONNECT_RECV_FACILITIES_LIST` |
| `AsAirportList()` | `SIMCONNECT_RECV_ID_AIRPORT_LIST` | `*types.SIMCONNECT_RECV_AIRPORT_LIST` |
| `AsNDBList()` | `SIMCONNECT_RECV_ID_NDB_LIST` | `*types.SIMCONNECT_RECV_NDB_LIST` |
| `AsVORList()` | `SIMCONNECT_RECV_ID_VOR_LIST` | `*types.SIMCONNECT_RECV_VOR_LIST` |
| `AsWaypointList()` | `SIMCONNECT_RECV_ID_WAYPOINT_LIST` | `*types.SIMCONNECT_RECV_WAYPOINT_LIST` |
| `AsSimObjectAndLiveryEnumeration()` | `SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST` | `*types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST` |
| `AsEnumerateInputEvents()` | `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` | `*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` |
| `AsGetInputEvent()` | `SIMCONNECT_RECV_ID_GET_INPUT_EVENT` | `*types.SIMCONNECT_RECV_GET_INPUT_EVENT` |
| `AsSubscribeInputEvent()` | `SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT` | `*types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` |
| `AsFlowEvent()` | `SIMCONNECT_RECV_ID_FLOW_EVENT` | `*types.SIMCONNECT_RECV_FLOW_EVENT` |

> **Note:** `AsEnumerateInputEvents()`, `AsGetInputEvent()`, `AsSubscribeInputEvent()`, and `AsFlowEvent()` are MSFS 2024 only. Calling them against MSFS 2020 will always return `nil` because the simulator never sends the corresponding `DwID` values.

---

## See Also

- [Engine/Client Usage](usage-client.md) — Core client API: connection, data definitions, sim object requests, events, facilities, and AI traffic
- [Client Data Areas](client-data-area.md) — Full conceptual guide and parameter reference for the Client Data Area API
- [Manager Usage](usage-manager.md) — Automatic connection lifecycle management with auto-reconnect
