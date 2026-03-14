---
title: "Input Events"
description: "Enumerate, read, write, and subscribe to MSFS 2024 Input Events via the SimConnect API."
order: 9
section: "client"
---

# Input Events

> **MSFS 2024 only.** The Input Event API (`EnumerateInputEvents`, `GetInputEvent`, `SetInputEvent`, `SubscribeInputEvent`, `UnsubscribeInputEvent`) is not present in the MSFS 2020 SimConnect SDK. Calling these methods against an MSFS 2020 installation will return an error. The `As*` message helpers (`AsEnumerateInputEvents()`, `AsGetInputEvent()`, `AsSubscribeInputEvent()`) will always return `nil` when connected to MSFS 2020 because the simulator never sends the corresponding `DwID` values.

> **See also:** [Engine/Client API Reference](usage-engine-api.md) for the full API surface, including the Input Event section with a compact example.

## What Are Input Events?

Input Events are hash-addressed simulator events that map to physical cockpit interactions. They represent things like button presses, switch states, and knob positions in aircraft panels. Unlike SimVars, which describe the state of the simulation world, Input Events are the raw input bindings — the same ones the simulator uses internally to trigger avionics logic.

You discover available events at runtime by enumerating them. Each event has a human-readable name, a 32-bit descriptor hash, and a value type (numeric or string). Once you have the name or hash you need, you can read the current value, write a new value, or subscribe to receive a notification every time the value changes.

Common use cases include:

- Building hardware panel integrations that reflect cockpit switch state
- Monitoring whether a specific button was pressed
- Driving cockpit state from an external source without going through SimVar writes

## Enumerate Available Events

Call `EnumerateInputEvents` to request the full list of available input events. The simulator responds with one or more `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` messages, each containing a batch of descriptors.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const EnumReqID uint32 = 1000

func main() {
    client := engine.New("InputEventEnum")
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    if err := client.EnumerateInputEvents(EnumReqID); err != nil {
        panic(err)
    }

    for msg := range client.Stream() {
        if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS {
            continue
        }
        recv := msg.AsEnumerateInputEvents()
        if recv == nil {
            continue
        }
        // recv.DwArraySize tells you how many descriptors are in this batch.
        // Access elements at recv.RgData[0] through recv.RgData[count-1].
        count := int(recv.DwArraySize)
        for i := 0; i < count; i++ {
            desc := recv.RgData[i]
            name := engine.BytesToString(desc.Name[:])
            fmt.Printf("Event: %-64s  hash=0x%08X  type=%d\n", name, desc.Hash, desc.Type)
        }
        // DwEntryNumber and DwOutOf let you track batched delivery.
        if recv.DwEntryNumber+1 >= recv.DwOutOf {
            fmt.Println("Enumeration complete.")
            break
        }
    }
}
```

**Signature:** `EnumerateInputEvents(requestID uint32) error`

`SIMCONNECT_INPUT_EVENT_DESCRIPTOR` fields:

| Field | Type | Description |
|---|---|---|
| `Name` | `[64]byte` | Human-readable event name, null-terminated. Use `engine.BytesToString(desc.Name[:])` to convert. |
| `Hash` | `DWORD` (32-bit) | Descriptor hash for this event. Cast to `uint64` when calling `GetInputEvent`, `SetInputEvent*`, or `SubscribeInputEvent`. |
| `Type` | `SIMCONNECT_DATATYPE` | Value type of the event (`SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE` or `SIMCONNECT_INPUT_EVENT_TYPE_STRING`). |
| `NodeNames` | `[1024]byte` | Null-separated list of associated node names. |

> **Note:** The `Hash` field in `SIMCONNECT_INPUT_EVENT_DESCRIPTOR` is a 32-bit `DWORD`. The DLL API calls (`GetInputEvent`, `SetInputEventDouble`, `SetInputEventString`, `SubscribeInputEvent`, `UnsubscribeInputEvent`) all take a `uint64` hash parameter. Cast explicitly: `uint64(desc.Hash)`.

## Get Event Value

To read the current value of a known event, call `GetInputEvent` with a request ID and the event's 64-bit hash. The response arrives asynchronously as a `SIMCONNECT_RECV_ID_GET_INPUT_EVENT` message.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const GetReqID uint32 = 1001

func main() {
    client := engine.New("InputEventGet")
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    // hash is a uint64 obtained from enumeration or a known constant.
    var hash uint64 = 0x00000001

    if err := client.GetInputEvent(GetReqID, hash); err != nil {
        panic(err)
    }

    for msg := range client.Stream() {
        if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_GET_INPUT_EVENT {
            continue
        }
        recv := msg.AsGetInputEvent()
        if recv == nil || uint32(recv.RequestID) != GetReqID {
            continue
        }
        if f, ok := engine.InputEventValueAsFloat64(recv); ok {
            fmt.Printf("Value (float64): %f\n", f)
        }
        if s, ok := engine.InputEventValueAsString(recv); ok {
            fmt.Printf("Value (string): %s\n", s)
        }
        break
    }
}
```

**Signature:** `GetInputEvent(requestID uint32, hash uint64) error`

Value extraction helpers for `*types.SIMCONNECT_RECV_GET_INPUT_EVENT`:

| Helper | Signature | Returns |
|---|---|---|
| `engine.InputEventValueAsFloat64` | `(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (float64, bool)` | Value and `true` when type is `DOUBLE`; `(0, false)` otherwise |
| `engine.InputEventValueAsString` | `(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (string, bool)` | Value and `true` when type is `STRING`; `("", false)` otherwise |

## Set Event Value

To write a value to an input event, use `SetInputEventDouble` for numeric events or `SetInputEventString` for string-typed events. Both calls are fire-and-forget — there is no response message. Any HRESULT error is returned directly from the call.

```go
//go:build windows

// Write a numeric value to a DOUBLE-typed event.
err := client.SetInputEventDouble(hash, 1.0)
if err != nil {
    // HRESULT error returned directly — no response message.
    panic(err)
}

// Write a string value to a STRING-typed event.
// Strings longer than 259 bytes are silently truncated to preserve the null terminator.
err = client.SetInputEventString(hash, "AUTOPILOT_ON")
if err != nil {
    panic(err)
}
```

**Signatures:**

- `SetInputEventDouble(hash uint64, value float64) error`
- `SetInputEventString(hash uint64, value string) error`

## Subscribe to Changes

Subscribing to an input event causes the simulator to push a notification message every time the event value changes. Use this instead of polling `GetInputEvent` in a loop.

```go
//go:build windows

err := client.SubscribeInputEvent(hash)
```

**Signature:** `SubscribeInputEvent(hash uint64) error`

Change notifications arrive as `SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT` messages. Cast with `msg.AsSubscribeInputEvent()` and use the engine helpers to extract the hash and value:

```go
//go:build windows

if recv := msg.AsSubscribeInputEvent(); recv != nil {
    // Read the hash from the wire struct (stored as [8]byte due to alignment).
    eventHash := engine.SubscribeInputEventHash(recv)

    if f, ok := engine.SubscribeInputEventValueAsFloat64(recv); ok {
        fmt.Printf("Event %d changed → %f\n", eventHash, f)
    }
    if s, ok := engine.SubscribeInputEventValueAsString(recv); ok {
        fmt.Printf("Event %d changed → %s\n", eventHash, s)
    }
}
```

> **Note:** `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.HashBytes` is stored as `[8]byte` at wire offset 12 rather than `uint64`. This avoids the 4-byte alignment padding Go would otherwise insert, which would shift the field to the wrong offset. Always use `engine.SubscribeInputEventHash(recv)` rather than reading `HashBytes` directly with `binary.LittleEndian.Uint64`.

Value extraction helpers for `*types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT`:

| Helper | Signature | Returns |
|---|---|---|
| `engine.SubscribeInputEventHash` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) uint64` | The 64-bit event hash |
| `engine.SubscribeInputEventValueAsFloat64` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (float64, bool)` | Value and `true` when type is `DOUBLE` |
| `engine.SubscribeInputEventValueAsString` | `(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (string, bool)` | Value and `true` when type is `STRING` |

## Unsubscribe

Cancel a subscription by passing the same hash you used to subscribe:

```go
//go:build windows

err := client.UnsubscribeInputEvent(hash)
```

**Signature:** `UnsubscribeInputEvent(hash uint64) error`

Call this before disconnecting if you want to be explicit, or when you no longer need change notifications for a particular event.

## Hash Note

There are two hash representations in this API, and it is important not to conflate them:

- **`SIMCONNECT_INPUT_EVENT_DESCRIPTOR.Hash`** — a `DWORD` (32-bit unsigned integer) returned in the enumeration response. Cast it to `uint64` when passing it to any DLL call: `uint64(desc.Hash)`.
- **`SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.HashBytes`** — a `[8]byte` field at wire offset 12 in the subscribe notification struct. This is a full 64-bit hash stored as raw bytes to work around Go's alignment rules. Use `engine.SubscribeInputEventHash(recv)` to decode it.

The two hashes are not necessarily the same value. The descriptor hash is a compact identifier used at enumeration time. The subscription notification carries the full 64-bit hash the simulator uses internally. Use the value from the subscribe notification if you need to correlate events back to subscriptions in your own bookkeeping.

## Complete Example

This example enumerates all available input events, subscribes to the first one found, then prints every change notification until interrupted.

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

    // Step 1: request the full list of available input events.
    if err := client.EnumerateInputEvents(EnumReqID); err != nil {
        panic(err)
    }

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)

    var subscribedHash uint64

    for {
        select {
        case <-sigChan:
            // Clean up subscription before exit.
            if subscribedHash != 0 {
                client.UnsubscribeInputEvent(subscribedHash)
            }
            fmt.Println("Shutting down.")
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
                // Print the first descriptor in this batch.
                name := engine.BytesToString(recv.RgData[0].Name[:])
                fmt.Printf("Found event: %s\n", name)

                // Subscribe to the first event we see.
                if subscribedHash == 0 {
                    subscribedHash = uint64(recv.RgData[0].Hash)
                    if err := client.SubscribeInputEvent(subscribedHash); err != nil {
                        fmt.Printf("SubscribeInputEvent failed: %v\n", err)
                    } else {
                        fmt.Printf("Subscribed to hash 0x%016X (%s)\n", subscribedHash, name)
                    }
                }

            case types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT:
                recv := msg.AsSubscribeInputEvent()
                if recv == nil {
                    continue
                }
                hash := engine.SubscribeInputEventHash(recv)
                if f, ok := engine.SubscribeInputEventValueAsFloat64(recv); ok {
                    fmt.Printf("Event 0x%016X changed → %f\n", hash, f)
                }
                if s, ok := engine.SubscribeInputEventValueAsString(recv); ok {
                    fmt.Printf("Event 0x%016X changed → %s\n", hash, s)
                }

            case types.SIMCONNECT_RECV_ID_EXCEPTION:
                recv := msg.AsException()
                if recv != nil {
                    fmt.Printf("SimConnect exception: %d\n", recv.DwException)
                }
            }
        }
    }
}
```

## See Also

- [Engine/Client API Reference](usage-engine-api.md) — Full Input Event API section with the complete method reference table
- [Engine/Client Usage](usage-client.md) — General dispatch loop patterns and message handling
- [Input Events (Manager)](manager-input-events.md) — Using Input Events through the manager with lifecycle-managed reconnection and auto-reconnect considerations
- [Client Configuration](config-client.md) — Connection options
