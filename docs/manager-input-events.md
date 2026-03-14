---
title: "Input Events (Manager)"
section: "manager"
order: 7
---

# Input Events (Manager)

The `manager` package exposes the full SimConnect Input Event API as direct methods on the `Manager` interface. This guide covers enumeration, value reads and writes, subscriptions, and cleanup through the manager's lifecycle-managed connection.

> **MSFS 2024 only.** The Input Event API is not present in the MSFS 2020 SimConnect SDK. All six methods (`EnumerateInputEvents`, `GetInputEvent`, `SetInputEventDouble`, `SetInputEventString`, `SubscribeInputEvent`, `UnsubscribeInputEvent`) will return an error when called against an MSFS 2020 installation.

> **See also:** [Input Events](guide-input-events.md) for the engine-layer reference covering message types, descriptor fields, wire layout details, and value extraction helpers.

## Overview

Input Events are hash-addressed simulator bindings that map to physical cockpit interactions — button states, switch positions, knob values. They differ from system events (which signal lifecycle changes like pause or crash) and from SimVars (which describe simulation world state). Input Events represent the raw input layer that the simulator uses internally to trigger avionics logic.

You cannot assume a fixed set of Input Events. The available set depends on the aircraft loaded and the simulator build. Enumerate at runtime to discover what is present, then use the hash to read, write, or subscribe.

## How Input Event Hashes Work

Each input event is identified by a hash. There are two representations:

- **Descriptor hash** (`SIMCONNECT_INPUT_EVENT_DESCRIPTOR.Hash`) — a 32-bit value returned during enumeration. Cast it to `uint64` when passing to any method: `uint64(desc.Hash)`.
- **Subscription notification hash** — the full 64-bit hash carried in `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT`. Extract it with `engine.SubscribeInputEventHash(recv)` rather than reading the raw bytes directly.

Hashes are stable for the duration of a simulator session. They may change between sessions or after loading a different aircraft. Enumerate again whenever the aircraft or simulator state changes if you need to maintain an accurate hash map.

## Workflow

### Step 1 — Enumerate available input events

Call `EnumerateInputEvents` with a request ID. The simulator responds with one or more `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` messages, each carrying a batch of descriptors.

```go
const EnumReqID uint32 = 1000

mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    if new != manager.StateConnected {
        return
    }
    if err := mgr.EnumerateInputEvents(EnumReqID); err != nil {
        log.Printf("EnumerateInputEvents failed: %v", err)
    }
})
```

### Step 2 — Handle enumeration responses

Subscribe to the `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` message type and read the descriptor batch:

```go
sub := mgr.SubscribeWithFilter("input-enum", 50, func(msg engine.Message) bool {
    return types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS
})
defer sub.Unsubscribe()

go func() {
    for {
        select {
        case msg := <-sub.Messages():
            recv := msg.AsEnumerateInputEvents()
            if recv == nil {
                continue
            }
            count := int(recv.DwArraySize)
            for i := 0; i < count; i++ {
                desc := recv.RgData[i]
                name := engine.BytesToString(desc.Name[:])
                log.Printf("Event: %-64s  hash=0x%08X  type=%d", name, desc.Hash, desc.Type)
            }
        case <-sub.Done():
            return
        }
    }
}()
```

### Step 3 — Subscribe to change notifications

Once you have the hash of an event you want to track, call `SubscribeInputEvent`. The simulator will push a notification every time the event value changes.

```go
var eventHash uint64 = 0x00000001 // replace with hash from enumeration

if err := mgr.SubscribeInputEvent(eventHash); err != nil {
    log.Printf("SubscribeInputEvent failed: %v", err)
}
```

### Step 4 — Request the current value

To read the current value once, call `GetInputEvent`. The response arrives as a `SIMCONNECT_RECV_ID_GET_INPUT_EVENT` message.

```go
const GetReqID uint32 = 1001

if err := mgr.GetInputEvent(GetReqID, eventHash); err != nil {
    log.Printf("GetInputEvent failed: %v", err)
}
```

Receive the response:

```go
sub := mgr.SubscribeWithFilter("input-get", 10, func(msg engine.Message) bool {
    return types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_GET_INPUT_EVENT
})
defer sub.Unsubscribe()

go func() {
    for msg := range sub.Messages() {
        recv := msg.AsGetInputEvent()
        if recv == nil || uint32(recv.RequestID) != GetReqID {
            continue
        }
        if f, ok := engine.InputEventValueAsFloat64(recv); ok {
            log.Printf("Value (float64): %f", f)
        }
        if s, ok := engine.InputEventValueAsString(recv); ok {
            log.Printf("Value (string): %s", s)
        }
    }
}()
```

### Step 5 — Set a value

Write a new value using `SetInputEventDouble` for numeric events or `SetInputEventString` for string-typed events. Both are fire-and-forget — no response message is sent.

```go
// Write a numeric value
if err := mgr.SetInputEventDouble(eventHash, 1.0); err != nil {
    log.Printf("SetInputEventDouble failed: %v", err)
}

// Write a string value (strings longer than 259 bytes are silently truncated)
if err := mgr.SetInputEventString(eventHash, "AUTOPILOT_ON"); err != nil {
    log.Printf("SetInputEventString failed: %v", err)
}
```

### Step 6 — Unsubscribe

When you no longer need change notifications for an event, call `UnsubscribeInputEvent` with the same hash.

```go
if err := mgr.UnsubscribeInputEvent(eventHash); err != nil {
    log.Printf("UnsubscribeInputEvent failed: %v", err)
}
```

## Complete Example

The following example connects via manager, enumerates input events on each connection, subscribes to the first event found, and prints change notifications.

```go
//go:build windows

package main

import (
    "errors"
    "log"
    "os"
    "os/signal"
    "sync/atomic"

    "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    EnumReqID uint32 = 1000
    GetReqID  uint32 = 1001
)

var subscribedHash atomic.Uint64

func main() {
    mgr := simconnect.New("InputEventDemo",
        manager.WithAutoReconnect(true),
    )

    // Enumerate on every (re)connection
    mgr.OnOpen(func(data *types.SIMCONNECT_RECV_OPEN) {
        subscribedHash.Store(0)
        if err := mgr.EnumerateInputEvents(EnumReqID); err != nil {
            if errors.Is(err, manager.ErrNotConnected) {
                return
            }
            log.Printf("EnumerateInputEvents: %v", err)
        }
    })

    // Handle enumeration and subscription change messages
    sub := mgr.Subscribe("input-events", 50)
    defer sub.Unsubscribe()

    go func() {
        for {
            select {
            case msg := <-sub.Messages():
                handleMessage(mgr, msg)
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

func handleMessage(mgr manager.Manager, msg engine.Message) {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {

    case types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS:
        recv := msg.AsEnumerateInputEvents()
        if recv == nil {
            return
        }
        count := int(recv.DwArraySize)
        for i := 0; i < count; i++ {
            desc := recv.RgData[i]
            name := engine.BytesToString(desc.Name[:])
            log.Printf("Event: %-64s  hash=0x%08X  type=%d", name, desc.Hash, desc.Type)
            // Subscribe to the first event found in the first batch
            if i == 0 && recv.DwEntryNumber == 0 && subscribedHash.Load() == 0 {
                h := uint64(desc.Hash)
                if err := mgr.SubscribeInputEvent(h); err == nil {
                    subscribedHash.Store(h)
                    log.Printf("Subscribed to %s (0x%016X)", name, h)
                }
            }
        }

    case types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT:
        recv := msg.AsSubscribeInputEvent()
        if recv == nil {
            return
        }
        hash := engine.SubscribeInputEventHash(recv)
        if f, ok := engine.SubscribeInputEventValueAsFloat64(recv); ok {
            log.Printf("Event 0x%016X changed → %f", hash, f)
        }
        if s, ok := engine.SubscribeInputEventValueAsString(recv); ok {
            log.Printf("Event 0x%016X changed → %s", hash, s)
        }

    case types.SIMCONNECT_RECV_ID_EXCEPTION:
        recv := msg.AsException()
        if recv != nil {
            log.Printf("SimConnect exception: %d", recv.DwException)
        }
    }
}
```

## Method Reference

| Method | Parameters | Returns | Notes |
|--------|-----------|---------|-------|
| `EnumerateInputEvents` | `requestID uint32` | `error` | Triggers one or more `SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS` response messages |
| `GetInputEvent` | `requestID uint32, hash uint64` | `error` | Async — response arrives as `SIMCONNECT_RECV_ID_GET_INPUT_EVENT` |
| `SetInputEventDouble` | `hash uint64, value float64` | `error` | Fire-and-forget; no response message |
| `SetInputEventString` | `hash uint64, value string` | `error` | Fire-and-forget; strings over 259 bytes are silently truncated |
| `SubscribeInputEvent` | `hash uint64` | `error` | Change notifications arrive as `SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT` |
| `UnsubscribeInputEvent` | `hash uint64` | `error` | Cancels the active subscription for the given hash |

## Double vs String

Use `SetInputEventDouble` for the vast majority of sim controls. Most Input Events are numeric — switch states (0.0 or 1.0), throttle positions (0.0–1.0), heading values, and similar. If `SIMCONNECT_INPUT_EVENT_DESCRIPTOR.Type` is `SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE`, use the double setter.

Use `SetInputEventString` only when `Type` is `SIMCONNECT_INPUT_EVENT_TYPE_STRING`. String-typed events are rare and typically represent text-mode commands or named state identifiers in specialised aircraft implementations. Strings longer than 259 bytes are silently truncated on the DLL side to preserve the null terminator.

When in doubt, check the `Type` field from the enumeration descriptor before setting a value.

## No Auto-Resubscribe on Reconnect

Input Event subscriptions are not restored automatically when the manager reconnects to the simulator. The SimConnect session is fully reset on each connection — all previously registered subscriptions, enumerations, and hash-to-event mappings are gone.

You must resubscribe in your `OnOpen` handler:

```go
mgr.OnOpen(func(data *types.SIMCONNECT_RECV_OPEN) {
    // Re-enumerate to rediscover hashes for the current session
    if err := mgr.EnumerateInputEvents(EnumReqID); err != nil {
        log.Printf("EnumerateInputEvents: %v", err)
    }
    // Re-subscribe once you have valid hashes from the enumeration response
})
```

Do not cache hashes across reconnections. Hashes are session-scoped and may differ after an aircraft reload or simulator restart.

## ErrNotConnected

Every method returns `manager.ErrNotConnected` when the manager has no active connection. This covers the startup window before the first successful connection and any reconnection gap.

```go
if err := mgr.EnumerateInputEvents(EnumReqID); err != nil {
    if errors.Is(err, manager.ErrNotConnected) {
        // Not yet connected — will retry from OnOpen handler
        return
    }
    log.Printf("EnumerateInputEvents failed: %v", err)
}
```

The manager does not queue or retry failed calls. Register your Input Event setup inside `OnOpen` so it runs automatically on each connection.

## See Also

- [Input Events](guide-input-events.md) — Engine-layer reference: descriptor fields, wire layout notes, hash extraction helpers, and complete enumeration/subscribe examples using the raw client
- [Manager Usage](usage-manager.md) — Full manager API reference including subscriptions and connection lifecycle
- [Request and ID Management](manager-requests-ids.md) — ID allocation strategy; `requestID` in `EnumerateInputEvents` and `GetInputEvent` must be in the user range (1–999,999,849)
