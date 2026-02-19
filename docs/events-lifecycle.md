---
title: "Event Lifecycle"
description: "System events dispatch pipeline and consumption patterns."
order: 5
section: "events"
---

# Event Lifecycle

This document describes every system event the manager subscribes to, how events flow through the dispatch pipeline, and the two consumption patterns available to users: callbacks and channel-based subscriptions.

> **See also:** [Manager Usage](usage-manager.md) for the complete manager API reference, and [ID Management](manager-requests-ids.md) for reserved ID ranges.

## Event Architecture

When the manager receives a SimConnect `OPEN` message, it automatically subscribes to all built-in system events via `SubscribeToSystemEvent`. Each event is assigned a manager-reserved ID (999,999,987 - 999,999,998) for internal tracking. Users never need to subscribe to these events manually.

```
SimConnect DLL
    │
    ▼
  Engine (dispatch loop)
    │
    ▼
  Manager (processMessage)
    ├── SIMCONNECT_RECV_ID_EVENT           → Pause, Sim, Crashed, CrashReset, Sound, View, FlightPlanDeactivated, Custom
    ├── SIMCONNECT_RECV_ID_EVENT_FILENAME  → FlightLoaded, AircraftLoaded, FlightPlanActivated
    ├── SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE → ObjectAdded, ObjectRemoved
    └── SIMCONNECT_RECV_ID_SIMOBJECT_DATA  → SimState polling (Camera, environment, telemetry)
```

Each event is forwarded to:
1. **SimState** — events that modify sim state (Pause, Sim, Crashed, CrashReset, Sound) update the internal `SimState` struct and trigger `OnSimStateChange` notifications.
2. **Typed callback handlers** — `On<Event>` callbacks registered by the user.
3. **Channel subscriptions** — `SubscribeOn<Event>` channels for goroutine-based consumers.
4. **Generic message handlers** — `OnMessage` callbacks and `Subscribe`/`SubscribeWithFilter`/`SubscribeWithType` channels always receive the raw message.

## Built-In System Events

The manager subscribes to **12 system events** on every connection. These are registered in `registerSimStateSubscriptions` and cannot be duplicated by user code.

### State-Modifying Events

These events update `SimState` fields and trigger `OnSimStateChange` notifications in addition to their own typed handlers.

| Event | SimConnect Name | SimState Field | Handler Signature | Data |
|-------|----------------|----------------|-------------------|------|
| Pause | `"Pause"` | `Paused` | `func(paused bool)` | `DwData == 1` means paused |
| Sim | `"Sim"` | `SimRunning` | `func(running bool)` | `DwData == 1` means running |
| Crashed | `"Crashed"` | `Crashed` | `func()` | `DwData == 1` means crashed |
| CrashReset | `"CrashReset"` | `CrashReset` | `func()` | `DwData == 1` means reset |
| Sound | `"Sound"` | `Sound` | `func(soundID uint32)` | `DwData` is the sound event ID |

**Dispatch behavior:** The manager acquires a write lock (`mu.Lock`), compares the new value against the current `SimState` field, and only fires handlers if the value changed. Handlers are copied under the lock and invoked outside it.

### Non-State Events

These events fire handlers directly without modifying `SimState`.

| Event | SimConnect Name | Recv Type | Handler Signature | Data |
|-------|----------------|-----------|-------------------|------|
| View | `"View"` | `RECV_ID_EVENT` | `func(viewID uint32)` | `DwData` is the camera view ID |
| FlightPlanDeactivated | `"FlightPlanDeactivated"` | `RECV_ID_EVENT` | `func()` | Void event (no data payload) |
| FlightLoaded | `"FlightLoaded"` | `RECV_ID_EVENT_FILENAME` | `func(filename string)` | Flight file path |
| AircraftLoaded | `"AircraftLoaded"` | `RECV_ID_EVENT_FILENAME` | `func(filename string)` | Aircraft `.AIR` file path |
| FlightPlanActivated | `"FlightPlanActivated"` | `RECV_ID_EVENT_FILENAME` | `func(filename string)` | Flight plan file path |
| ObjectAdded | `"ObjectAdded"` | `RECV_ID_EVENT_OBJECT_ADDREMOVE` | `func(objectID uint32, objType SIMCONNECT_SIMOBJECT_TYPE)` | AI object info |
| ObjectRemoved | `"ObjectRemoved"` | `RECV_ID_EVENT_OBJECT_ADDREMOVE` | `func(objectID uint32, objType SIMCONNECT_SIMOBJECT_TYPE)` | AI object info |

**Dispatch behavior:** The manager acquires a read lock (`mu.RLock`), copies handlers, releases the lock, then invokes handlers with `safeCallHandler` for panic recovery.

## Consumption Patterns

Every event supports two consumption patterns. Choose based on your architecture:

### Callback Pattern (`On<Event>`)

Register a function to be invoked synchronously in the dispatch goroutine. Best for simple, fast reactions.

```go
// Register
handlerID := mgr.OnPause(func(paused bool) {
    fmt.Printf("Pause state: %v\n", paused)
})

// Remove when done
mgr.RemovePause(handlerID)
```

**Characteristics:**
- Invoked on the manager's dispatch goroutine
- Wrapped in `safeCallHandler` — panics are recovered and logged
- Blocks message processing while executing — keep handlers fast
- Returns a handler ID for removal

### Channel Pattern (`SubscribeOn<Event>`)

Creates a buffered channel subscription for goroutine-based processing. Best for async consumers.

```go
sub := mgr.SubscribeOnPause("my-pause-sub", 10)
defer sub.Unsubscribe()

go func() {
    for {
        select {
        case paused := <-sub.Messages():
            // process in dedicated goroutine
        case <-sub.Done():
            return
        }
    }
}()
```

**Characteristics:**
- Non-blocking send to the channel — messages are dropped if the buffer is full
- Automatically cancelled when the manager stops
- `Done()` channel signals subscription termination
- Safe for concurrent use

### When to Use Which

| Scenario | Pattern | Reason |
|----------|---------|--------|
| Log a state change | Callback | Simple, no goroutine overhead |
| Update a UI component | Channel | Decouples event from rendering goroutine |
| Trigger a data request | Callback | Immediate response in dispatch context |
| Aggregate events over time | Channel | Buffered processing in dedicated goroutine |
| Multiple consumers for one event | Both | Register multiple callbacks or subscriptions independently |

## Complete Event Reference

### Pause

Fires when the simulator pause state changes.

```go
// Callback
id := mgr.OnPause(func(paused bool) {
    if paused {
        fmt.Println("Simulator paused")
    } else {
        fmt.Println("Simulator resumed")
    }
})
mgr.RemovePause(id)

// Channel
sub := mgr.SubscribeOnPause("pause", 5)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    ev := msg.AsEvent()
    if ev != nil {
        fmt.Printf("Pause data: %d\n", ev.DwData)
    }
}
```

### Sim (Running)

Fires when the simulator starts or stops running.

```go
// Callback
id := mgr.OnSimRunning(func(running bool) {
    if running {
        fmt.Println("Simulator started")
    } else {
        fmt.Println("Simulator stopped")
    }
})
mgr.RemoveSimRunning(id)

// Channel
sub := mgr.SubscribeOnSimRunning("sim", 5)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    ev := msg.AsEvent()
    if ev != nil {
        fmt.Printf("Sim data: %d\n", ev.DwData)
    }
}
```

### Crashed

Fires when the user aircraft crashes. Updates `SimState.Crashed`.

```go
// Callback
id := mgr.OnCrashed(func() {
    fmt.Println("Aircraft crashed!")
})
mgr.RemoveCrashed(id)

// Channel
sub := mgr.SubscribeOnCrashed("crash", 4)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    fmt.Println("Crash event received")
}
```

### CrashReset

Fires when the crash state is reset (e.g., the user selects "Restart"). Updates `SimState.CrashReset`.

```go
// Callback
id := mgr.OnCrashReset(func() {
    fmt.Println("Crash reset")
})
mgr.RemoveCrashReset(id)

// Channel
sub := mgr.SubscribeOnCrashReset("reset", 4)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    fmt.Println("Crash reset event received")
}
```

### Sound

Fires when a sound event occurs. Updates `SimState.Sound` with the sound ID.

```go
// Callback
id := mgr.OnSoundEvent(func(soundID uint32) {
    fmt.Printf("Sound event: %d\n", soundID)
})
mgr.RemoveSoundEvent(id)

// Channel
sub := mgr.SubscribeOnSoundEvent("sound", 8)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    ev := msg.AsEvent()
    if ev != nil {
        fmt.Printf("Sound ID: %d\n", ev.DwData)
    }
}
```

### View

Fires when the camera view changes. Does not modify `SimState` — camera state is tracked separately via SimVar polling.

```go
// Callback
id := mgr.OnView(func(viewID uint32) {
    fmt.Printf("View changed to: %d\n", viewID)
})
mgr.RemoveView(id)

// Channel
sub := mgr.SubscribeOnView("view", 8)
defer sub.Unsubscribe()
for msg := range sub.Messages() {
    ev := msg.AsEvent()
    if ev != nil {
        fmt.Printf("View ID: %d\n", ev.DwData)
    }
}
```

### FlightLoaded

Fires when a flight file (`.FLT`) is loaded.

```go
// Callback
id := mgr.OnFlightLoaded(func(filename string) {
    fmt.Printf("Flight loaded: %s\n", filename)
})
mgr.RemoveFlightLoaded(id)

// Channel
sub := mgr.SubscribeOnFlightLoaded("flight", 4)
defer sub.Unsubscribe()
for ev := range sub.Events() {
    fmt.Printf("Flight file: %s\n", ev.Filename)
}
```

### AircraftLoaded

Fires when an aircraft (`.AIR`) file is loaded or changed.

```go
// Callback
id := mgr.OnAircraftLoaded(func(filename string) {
    fmt.Printf("Aircraft loaded: %s\n", filename)
})
mgr.RemoveAircraftLoaded(id)

// Channel
sub := mgr.SubscribeOnAircraftLoaded("aircraft", 4)
defer sub.Unsubscribe()
for ev := range sub.Events() {
    fmt.Printf("Aircraft file: %s\n", ev.Filename)
}
```

### FlightPlanActivated

Fires when a flight plan is activated.

```go
// Callback
id := mgr.OnFlightPlanActivated(func(filename string) {
    fmt.Printf("Flight plan activated: %s\n", filename)
})
mgr.RemoveFlightPlanActivated(id)

// Channel
sub := mgr.SubscribeOnFlightPlanActivated("plan", 4)
defer sub.Unsubscribe()
for ev := range sub.Events() {
    fmt.Printf("Flight plan file: %s\n", ev.Filename)
}
```

### FlightPlanDeactivated

Fires when the active flight plan is deactivated. This is a void event with no data payload.

```go
// Callback
id := mgr.OnFlightPlanDeactivated(func() {
    fmt.Println("Flight plan deactivated")
})
mgr.RemoveFlightPlanDeactivated(id)

// Channel
sub := mgr.SubscribeOnFlightPlanDeactivated("deactivate", 4)
defer sub.Unsubscribe()
for range sub.Messages() {
    fmt.Println("Flight plan deactivated")
}
```

### ObjectAdded

Fires when an AI object (traffic, ground vehicle, etc.) is added to the simulation.

```go
// Callback
id := mgr.OnObjectAdded(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
    fmt.Printf("Object added: id=%d type=%d\n", objectID, objType)
})
mgr.RemoveObjectAdded(id)

// Channel
sub := mgr.SubscribeOnObjectAdded("add", 32)
defer sub.Unsubscribe()
for ev := range sub.Events() {
    fmt.Printf("Added: id=%d type=%d\n", ev.ObjectID, ev.ObjType)
}
```

### ObjectRemoved

Fires when an AI object is removed from the simulation.

```go
// Callback
id := mgr.OnObjectRemoved(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
    fmt.Printf("Object removed: id=%d type=%d\n", objectID, objType)
})
mgr.RemoveObjectRemoved(id)

// Channel
sub := mgr.SubscribeOnObjectRemoved("rem", 32)
defer sub.Unsubscribe()
for ev := range sub.Events() {
    fmt.Printf("Removed: id=%d type=%d\n", ev.ObjectID, ev.ObjType)
}
```

## Custom System Events

Beyond the 12 built-in events, users can subscribe to any SimConnect system event by name. Custom events use a dynamic ID pool (999,999,850 - 999,999,886, 37 slots) allocated at runtime.

### Subscribing

```go
// Channel subscription (also registers with SimConnect)
sub, err := mgr.SubscribeToCustomSystemEvent("6Hz", 10)
if err != nil {
    log.Fatal(err)
}
defer sub.Unsubscribe()

// Callback (requires prior SubscribeToCustomSystemEvent)
handlerID, err := mgr.OnCustomSystemEvent("6Hz", func(name string, data uint32) {
    fmt.Printf("[%s] data=%d\n", name, data)
})
```

### Unsubscribing

```go
// Remove callback handler
mgr.RemoveCustomSystemEvent("6Hz", handlerID)

// Fully unsubscribe from SimConnect
mgr.UnsubscribeFromCustomSystemEvent("6Hz")
```

### Reserved Names

The following names are reserved for built-in events and will return `ErrReservedEventName`:

`Pause`, `Sim`, `Crashed`, `CrashReset`, `Sound`, `View`, `FlightLoaded`, `AircraftLoaded`, `FlightPlanActivated`, `FlightPlanDeactivated`, `ObjectAdded`, `ObjectRemoved`

### Lifecycle

- Custom events are registered with SimConnect when `SubscribeToCustomSystemEvent` is called.
- Custom event subscriptions are **cleared on disconnect** and must be re-registered after reconnection.
- The ID pool resets on disconnect, so the same 37 slots are available for each connection.

## Internal vs User-Facing Events

| Category | Events | User Access |
|----------|--------|-------------|
| **Fully user-facing** | All 12 built-in + custom events | Callback + channel subscription |
| **Internal + user-facing** | Pause, Sim, Crashed, CrashReset, Sound | Updates `SimState` internally, exposes handlers + subscriptions to users |
| **Internal only** | SimState data polling (camera, environment, telemetry) | Consumed internally; users access via `mgr.SimState()` and `OnSimStateChange` |

## Event Lifecycle During Connection

```
1. Manager.Start() called
   └── connectWithRetry() → StateConnecting
       └── engine.Connect() → StateConnected

2. OPEN message received → StateAvailable
   └── registerSimStateSubscriptions()
       ├── SubscribeToSystemEvent("Pause", ...)
       ├── SubscribeToSystemEvent("Sim", ...)
       ├── SubscribeToSystemEvent("Crashed", ...)
       ├── SubscribeToSystemEvent("CrashReset", ...)
       ├── SubscribeToSystemEvent("Sound", ...)
       ├── SubscribeToSystemEvent("View", ...)
       ├── SubscribeToSystemEvent("FlightLoaded", ...)
       ├── SubscribeToSystemEvent("AircraftLoaded", ...)
       ├── SubscribeToSystemEvent("FlightPlanActivated", ...)
       ├── SubscribeToSystemEvent("FlightPlanDeactivated", ...)
       ├── SubscribeToSystemEvent("ObjectAdded", ...)
       ├── SubscribeToSystemEvent("ObjectRemoved", ...)
       ├── AddToDataDefinition(camera/sim state, ...)
       └── RequestDataOnSimObject(periodic polling, ...)

3. Events flow through processMessage()
   ├── Typed handlers invoked (OnPause, OnCrashed, etc.)
   ├── Channel subscriptions forwarded
   └── Generic OnMessage/Subscribe always receive raw messages

4. QUIT message received → StateDisconnected
   └── SimState reset to defaults
       └── Custom events cleared

5. AutoReconnect → back to step 1
```

## Thread Safety

All handler registration and invocation is thread-safe:

- **Registration** (`On<Event>`, `Remove<Event>`) acquires `mu.Lock` / `mu.RLock`.
- **Dispatch** copies handler slices under lock using pre-allocated buffers, then invokes outside the lock.
- **Panic recovery** — every handler invocation is wrapped in `safeCallHandler`, which uses `defer/recover` to log panics without crashing the dispatch loop.
- **Channel subscriptions** use `atomic.Bool` for fast closed-state checking and `closeMu` for safe send operations.

## See Also

- [Manager Usage](usage-manager.md) — Full API reference
- [Manager Configuration](config-manager.md) — Configuration options including `SimStatePeriod`
- [ID Management](manager-requests-ids.md) — Reserved ID ranges
- [Examples: simconnect-events](../examples/simconnect-events/) — Working example demonstrating all event types
