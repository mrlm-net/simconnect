---
title: "Manager API Reference"
description: "Auto-reconnect manager API: state subscriptions, custom events, and ID allocation."
order: 3
section: "manager"
---

# Manager API Reference

This document is the API reference for the `manager` package. It covers the features that go beyond basic connection management: the Manager vs Engine trade-off, auto-reconnect lifecycle, SimState subscriptions, connection event subscriptions, custom system events, ID allocation, and state accessors.

> **See also:** [Manager Usage](usage-manager.md) for full usage examples including data requests, channel-based subscriptions, and system event handlers.

## Manager vs Engine

The library exposes two distinct layers for connecting to SimConnect.

| Concern | `engine` (direct) | `manager` (recommended) |
|---|---|---|
| Connection lifecycle | Manual `Connect` / `Disconnect` | Automatic with reconnect loop |
| Auto-reconnect | No | Yes (configurable) |
| SimState tracking | No | Yes (camera, pause, position, environment) |
| Built-in system event subscriptions | No | Yes (Pause, Sim, Crash, View, FlightLoaded, ...) |
| ID allocation helpers | No | Yes (`IsValidUserID`, `IsManagerID`) |
| Concurrent subscriptions | No | Yes (channel-based, callback-based) |

**Use `engine` when:**

- You are writing a short-lived script or one-shot tool.
- You need full control over the connection and message loop.
- You do not need reconnection or SimState tracking.

**Use `manager` when:**

- You are building a long-running add-on that must survive simulator restarts.
- You want automatic reconnection without writing a retry loop yourself.
- You need SimState (camera mode, pause state, position, environment) delivered as a unified struct.
- You want typed, channel-based subscriptions for system events.

The `manager` wraps an `engine` instance internally (a new one per connection) and exposes most of its API through the `Manager` interface; the rest is reachable through `Client()`.

```go
//go:build windows

package main

import (
    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    // Manager: auto-reconnect, SimState, typed subscriptions
    mgr := manager.New("MyApp")
    _ = mgr

    // Engine: single connection, manual lifecycle
    // client := engine.New("MyApp")
}
```

## Auto-Reconnect Lifecycle

### Start and Stop

`Start()` blocks and runs the connection loop. `Stop()` cancels it and waits for all subscriptions to drain.

```go
//go:build windows

package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())

    mgr := manager.New("MyApp", manager.WithContext(ctx))

    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    go func() {
        <-sig
        mgr.Stop()
        cancel()
    }()

    if err := mgr.Start(); err != nil {
        fmt.Println("Manager stopped:", err)
    }
}
```

### Reconnect Loop Behaviour

Each call to `Start()` runs the following loop:

1. Enter `StateConnecting` and attempt `engine.Connect()` with a per-attempt timeout (`ConnectionTimeout`, default 30s).
2. If the attempt fails, wait `RetryInterval` (default 15s) and retry, up to `MaxRetries` attempts in all (default 0 = unlimited). After a lost connection the limit is `ReconnectMaxRetries` when set (0 or more), otherwise `MaxRetries` again. Reaching the limit enters `StateDisconnected` and `Start()` returns an error; the manager does not try again.
3. On success, enter `StateConnected` and begin dispatching messages. When the simulator's OPEN message arrives, the manager registers its SimState request and system events, subscribes again the kept custom system events (and, with `ResubscribeOnReconnect`, the application's pass-through subscriptions), then enters `StateAvailable` and fires `OnOpen`.
4. If the simulator closes the connection (stream channel closes), reset `SimState` to defaults, close the old engine, clear the request registry and enter `StateDisconnected`.
5. If `AutoReconnect` is `true` (default), enter `StateReconnecting`, wait `ReconnectDelay` (default 30s), and restart from step 1. With `AutoReconnect` off, `Start()` returns `nil`.
6. If the context is cancelled at any point, `Start()` disconnects and returns `ctx.Err()` (`context.Canceled`). Call `Stop()` to wait for subscriptions to drain.

A connection attempt that runs past `ConnectionTimeout` is abandoned on its own engine and the next attempt uses a new engine; if the abandoned attempt succeeds late, the manager disconnects it.

### Subscription Behaviour on Reconnect

Subscriptions created before `Start()` (or while connected) survive reconnections. The manager does **not** destroy and recreate channels on reconnect — the same `Subscription`, `SimStateSubscription`, `ConnectionOpenSubscription`, and `ConnectionQuitSubscription` instances remain valid across connection cycles.

However, **data definitions and data requests are not automatically re-registered** after a reconnect. Re-register them inside an `OnConnectionStateChange` handler that fires when the state transitions to `StateConnected`.

The same goes for subscriptions made through the manager's pass-through calls (`SubscribeToSystemEvent`, `SubscribeInputEvent`, `SubscribeToFlowEvent`, `SubscribeToFacilities`, `SubscribeToFacilitiesEX1`): they belong to the connection they were made on. Create the manager with `WithResubscribeOnReconnect(true)` to have them subscribed again, with the same IDs, on every new connection; the matching unsubscribe calls remove them from that list, and `Stop()` clears it.

```go
//go:build windows

package main

import (
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    DataDefID = 1000
    DataReqID = 1001
)

func setupOnConnect(mgr manager.Manager) {
    mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
        if new != manager.StateConnected {
            return
        }
        // Re-register data definitions on every (re)connect
        mgr.AddToDataDefinition(DataDefID, "PLANE LATITUDE", "degrees",
            types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
        mgr.RequestDataOnSimObject(
            DataReqID, DataDefID,
            types.SIMCONNECT_OBJECT_ID_USER,
            types.SIMCONNECT_PERIOD_SECOND,
            types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
            0, 0, 0,
        )
    })
}
```

### Custom System Events on Reconnect

Custom system events registered with `SubscribeToCustomSystemEvent` (with their `OnCustomSystemEvent` handlers and channel subscriptions) are **kept over a lost connection** and subscribed again with the same IDs when the next connection opens; the application need not register them again. `Stop()` clears them and resets the custom event ID allocator.

### Stop and Shutdown Timeout

`Stop()` cancels the manager context and waits for all subscriptions to call `Unsubscribe()`. If subscriptions do not drain within `ShutdownTimeout` (default 10s), `Stop()` proceeds and logs a warning.

### Connection Timing Defaults

| Option | Default | Description |
|---|---|---|
| `WithRetryInterval` | 15s | Delay between failed connection attempts |
| `WithConnectionTimeout` | 30s | Timeout per individual connection attempt |
| `WithReconnectDelay` | 30s | Delay before reconnecting after a disconnect |
| `WithShutdownTimeout` | 10s | Maximum wait for subscriptions to close on stop |
| `WithMaxRetries` | 0 (unlimited) | Maximum connection attempts before giving up |
| `WithReconnectMaxRetries` | -1 (use `MaxRetries`) | Attempt limit after a lost connection (0 = unlimited) |
| `WithAutoReconnect` | true | Whether to reconnect after a disconnect |
| `WithResubscribeOnReconnect` | false | Subscribe pass-through subscriptions again on each new connection |

## SimState Subscriptions

The manager continuously polls SimConnect for a large set of simulator variables and aggregates them into a `SimState` struct. Changes to significant fields (discrete state, not continuous telemetry) are broadcast to all SimState subscribers.

### SimState Struct

`SimState` is a flat value struct (no pointers). Compare two values with `==` or use the `Equal` method, which ignores continuously changing fields (time, position, weather, speed) and only compares discrete state fields.

Key field groups:

| Group | Fields |
|---|---|
| Camera | `Camera` (`CameraState`), `Substate` (`CameraSubstate`), `SmartCameraActive` |
| Simulation control | `Paused`, `SimRunning`, `SimDisabled`, `SimulationRate`, `UserInputEnabled` |
| Crash and sound | `Crashed`, `CrashReset`, `Sound` |
| Realism | `Realism`, `RealismCrashDetection`, `RealismCrashWithOthers` |
| Rendering mode | `IsInVR`, `IsUsingMotionControllers`, `IsUsingJoystickThrottle`, `TrackIREnabled` |
| Session type | `IsInRTC`, `IsAvatar`, `IsAircraft`, `SimOnGround` |
| Avatar | `HandAnimState`, `HideAvatarInAircraft`, `ParachuteOpen` |
| Mission | `MissionScore` |
| Time (sim) | `SimulationTime`, `LocalTime`, `ZuluTime` |
| Date (local/Zulu) | `LocalDay`, `LocalMonth`, `LocalYear`, `ZuluDay`, `ZuluMonth`, `ZuluYear` |
| Position | `Latitude`, `Longitude`, `Altitude`, `IndicatedAltitude` |
| Attitude | `TrueHeading`, `MagneticHeading`, `Pitch`, `Bank` |
| Speed | `GroundSpeed`, `IndicatedAirspeed`, `TrueAirspeed`, `VerticalSpeed` |
| Environment | `AmbientTemperature`, `AmbientPressure`, `AmbientWindVelocity`, `AmbientWindDirection`, `AmbientVisibility`, `AmbientInCloud`, `AmbientPrecipState`, `AmbientInSmoke`, `EnvSmokeDensity`, `EnvCloudDensity` |
| Pressure and ground | `BarometerPressure`, `SeaLevelPressure`, `SeaLevelAmbientTemperature`, `GroundAltitude`, `MagVar`, `SurfaceType`, `DensityAltitude` |
| Sun and timezone | `ZuluSunriseTime`, `ZuluSunsetTime`, `TimeZoneOffset` |
| Units | `TooltipUnits`, `UnitsOfMeasure`, `VisualModelRadius` |

The full field list is defined in `pkg/manager/state.go`.

### Polling Frequency

The manager polls SimState at `SIMCONNECT_PERIOD_SIM_FRAME` by default (every sim frame, ~30-60 Hz). Use `WithSimStatePeriod` to reduce frequency if CPU usage matters:

```go
//go:build windows

package main

import (
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
    mgr := manager.New("MyApp",
        // Poll SimState once per second instead of every frame
        manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_SECOND),
    )
    _ = mgr
}
```

Supported values: `SIMCONNECT_PERIOD_SIM_FRAME`, `SIMCONNECT_PERIOD_VISUAL_FRAME`, `SIMCONNECT_PERIOD_SECOND`, `SIMCONNECT_PERIOD_ONCE`, `SIMCONNECT_PERIOD_NEVER`.

### OnSimStateChange (Callback)

Register a callback that fires when any significant SimState field changes. The callback receives both the old and new state.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    mgr := manager.New("MyApp")

    id := mgr.OnSimStateChange(func(old, new manager.SimState) {
        if old.Paused != new.Paused {
            if new.Paused {
                fmt.Println("Simulator paused")
            } else {
                fmt.Println("Simulator resumed")
            }
        }
        if old.Camera != new.Camera {
            fmt.Printf("Camera: %v -> %v\n", old.Camera, new.Camera)
        }
        if old.Crashed != new.Crashed && new.Crashed {
            fmt.Println("Crash detected")
        }
    })

    // Remove when done
    mgr.RemoveSimStateChange(id)
    _ = mgr
}
```

### SubscribeSimStateChange (Channel)

For goroutine-based processing, use the channel subscription. Each `SimStateChange` carries both the previous and new state.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func watchSimState(mgr manager.Manager) {
    sub := mgr.SubscribeSimStateChange("sim-state-watcher", 10)
    defer sub.Unsubscribe()

    for {
        select {
        case change := <-sub.SimStateChanges():
            if change.OldState.SimRunning != change.NewState.SimRunning {
                fmt.Printf("SimRunning changed to %v\n", change.NewState.SimRunning)
            }
        case <-sub.Done():
            return
        }
    }
}
```

### SimStateSubscription Interface

| Method | Returns | Description |
|---|---|---|
| `ID()` | `string` | Subscription identifier |
| `SimStateChanges()` | `<-chan SimStateChange` | Channel receiving state transitions |
| `Done()` | `<-chan struct{}` | Closed when subscription ends |
| `Unsubscribe()` | — | Cancels the subscription and closes channels |

### GetSimStateSubscription

Retrieves an existing SimState subscription by ID. Returns `nil` if not found.

```go
//go:build windows

package main

import "github.com/mrlm-net/simconnect/pkg/manager"

func example(mgr manager.Manager) {
    if sub := mgr.GetSimStateSubscription("sim-state-watcher"); sub != nil {
        // subscription is still active
        _ = sub
    }
}
```

## Connection Event Subscriptions

The manager fires events when the underlying SimConnect connection opens and when the simulator closes it. These are distinct from the connection state transitions: `OnOpen` fires when the SimConnect handshake completes (containing simulator version info), and `OnQuit` fires when the simulator signals it is shutting down.

### SubscribeOnOpen (Channel)

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func watchOpen(mgr manager.Manager) {
    sub := mgr.SubscribeOnOpen("open-watcher", 5)
    defer sub.Unsubscribe()

    for {
        select {
        case data := <-sub.Opens():
            fmt.Printf("Connected to: %s v%d.%d\n",
                data.ApplicationName,
                data.ApplicationVersionMajor,
                data.ApplicationVersionMinor,
            )
        case <-sub.Done():
            return
        }
    }
}
```

### OnOpen (Callback)

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func registerOpenHandler(mgr manager.Manager) string {
    return mgr.OnOpen(func(data types.ConnectionOpenData) {
        fmt.Printf("SimConnect open: %s v%d.%d\n",
            data.ApplicationName,
            data.ApplicationVersionMajor,
            data.ApplicationVersionMinor,
        )
    })
}
```

Remove with `mgr.RemoveOpen(id)`.

### SubscribeOnQuit (Channel)

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func watchQuit(mgr manager.Manager) {
    sub := mgr.SubscribeOnQuit("quit-watcher", 5)
    defer sub.Unsubscribe()

    for {
        select {
        case <-sub.Quits():
            fmt.Println("Simulator quit")
        case <-sub.Done():
            return
        }
    }
}
```

### OnQuit (Callback)

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func registerQuitHandler(mgr manager.Manager) string {
    return mgr.OnQuit(func(data types.ConnectionQuitData) {
        fmt.Println("Simulator quit")
    })
}
```

Remove with `mgr.RemoveQuit(id)`.

### ConnectionOpenSubscription Interface

| Method | Returns | Description |
|---|---|---|
| `ID()` | `string` | Subscription identifier |
| `Opens()` | `<-chan types.ConnectionOpenData` | Channel receiving open events |
| `Done()` | `<-chan struct{}` | Closed when subscription ends |
| `Unsubscribe()` | — | Cancels the subscription |

### ConnectionQuitSubscription Interface

| Method | Returns | Description |
|---|---|---|
| `ID()` | `string` | Subscription identifier |
| `Quits()` | `<-chan types.ConnectionQuitData` | Channel receiving quit events |
| `Done()` | `<-chan struct{}` | Closed when subscription ends |
| `Unsubscribe()` | — | Cancels the subscription |

### GetOpenSubscription / GetQuitSubscription

Retrieve an existing subscription by ID, or `nil` if not found.

```go
//go:build windows

package main

import "github.com/mrlm-net/simconnect/pkg/manager"

func example(mgr manager.Manager) {
    openSub := mgr.GetOpenSubscription("open-watcher")
    quitSub := mgr.GetQuitSubscription("quit-watcher")
    _, _ = openSub, quitSub
}
```

## Custom System Events

The manager handles a fixed set of system events internally (Pause, Sim, Crashed, CrashReset, Sound, View, FlightLoaded, AircraftLoaded, FlightPlanActivated, FlightPlanDeactivated, ObjectAdded, ObjectRemoved). For any other SimConnect system event not on that list, use the custom system event API.

### Reserved Event Names

The following names cannot be used with the custom event API — they are managed internally:

`Pause`, `Sim`, `Crashed`, `CrashReset`, `Sound`, `View`, `FlightLoaded`, `AircraftLoaded`, `FlightPlanActivated`, `FlightPlanDeactivated`, `ObjectAdded`, `ObjectRemoved`

Attempting to subscribe to a reserved name returns `ErrReservedEventName`.

### SubscribeToCustomSystemEvent

Creates a channel subscription for a named SimConnect system event. Returns a `Subscription` that delivers raw `engine.Message` values. Calling this for the same event name a second time reuses the already-registered SimConnect event and returns a new, separate subscription; earlier ones keep working. A new event name needs a connection (`ErrNotConnected` otherwise).

```go
//go:build windows

package main

import (
    "fmt"
    "log"

    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func subscribe6Hz(mgr manager.Manager) {
    sub, err := mgr.SubscribeToCustomSystemEvent("6Hz", 10)
    if err != nil {
        log.Printf("subscribe failed: %v", err)
        return
    }
    defer sub.Unsubscribe()

    for {
        select {
        case msg := <-sub.Messages():
            if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT {
                ev := msg.AsEvent()
                fmt.Printf("6Hz tick: data=%d\n", ev.DwData)
            }
        case <-sub.Done():
            return
        }
    }
}
```

### OnCustomSystemEvent

Registers a callback handler for a named system event. The event must be subscribed first via `SubscribeToCustomSystemEvent`; otherwise it returns `ErrCustomEventNotSubscribed`. Returns a handler ID that can be used to remove the handler.

```go
//go:build windows

package main

import (
    "fmt"
    "log"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

// CustomSystemEventHandler is: func(eventName string, data uint32)
type CustomSystemEventHandler = manager.CustomSystemEventHandler

func register6HzCallback(mgr manager.Manager) {
    // SubscribeToCustomSystemEvent registers the SimConnect subscription;
    // OnCustomSystemEvent registers a typed callback on top of it.
    _, err := mgr.SubscribeToCustomSystemEvent("6Hz", 4)
    if err != nil {
        log.Printf("subscribe failed: %v", err)
        return
    }

    id, err := mgr.OnCustomSystemEvent("6Hz", func(eventName string, data uint32) {
        fmt.Printf("[%s] fired: data=%d\n", eventName, data)
    })
    if err != nil {
        log.Printf("handler registration failed: %v", err)
        return
    }

    // Remove handler by ID when done
    if err := mgr.RemoveCustomSystemEvent("6Hz", id); err != nil {
        log.Printf("remove handler failed: %v", err)
    }
}
```

### UnsubscribeFromCustomSystemEvent

Removes the SimConnect system event subscription entirely, clears all associated handlers and closes every channel subscription returned for it (their `Done` channels close). The event's ID becomes free for another event.

```go
//go:build windows

package main

import (
    "log"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func unsubscribe6Hz(mgr manager.Manager) {
    if err := mgr.UnsubscribeFromCustomSystemEvent("6Hz"); err != nil {
        log.Printf("unsubscribe failed: %v", err)
    }
}
```

### Custom Event ID Limit

The manager allocates IDs for custom events from a sub-range of its reserved range: 999,999,910 to 999,999,979 (70 slots, `CustomEventIDMin`–`CustomEventIDMax`). Subscribing to more than 70 distinct custom event names at once returns `ErrCustomEventIDExhausted`. IDs of unsubscribed events are used again; `Stop()` resets the allocator.

### Error Values

| Error | Meaning |
|---|---|
| `ErrReservedEventName` | Event name is reserved for internal use |
| `ErrCustomEventNotFound` | Event was not subscribed |
| `ErrCustomEventIDExhausted` | All 70 custom event ID slots are in use |
| `ErrNotConnected` | A new custom event was subscribed without a connection |
| `ErrCustomEventNotSubscribed` | Tried to add a callback before subscribing |
| `ErrCustomEventHandlerNotFound` | Handler ID not found for removal |

## ID Allocation

SimConnect requires every data definition, data request, and system event subscription to carry a numeric ID. The manager reserves the top of the `uint32` space for its own operations so that application code can start from 1 without any coordination.

### ID Ranges

| Range | Owner | Slots |
|---|---|---|
| 1 — 999,999,899 | User application | 999,999,899 |
| 999,999,900 — 999,999,999 | Manager (internal) | 100 |
| 999,999,910 — 999,999,979 | Manager (custom events, inside the internal range) | 70 |

### Validation Helpers

```go
//go:build windows

package main

import (
    "fmt"
    "log"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

const MyDataDefID uint32 = 1000

func validateIDs() {
    if !manager.IsValidUserID(MyDataDefID) {
        log.Fatalf("ID %d conflicts with the manager's reserved range", MyDataDefID)
    }

    // Distinguish user vs manager IDs at runtime
    if manager.IsManagerID(999999900) {
        fmt.Println("ID 999999900 is reserved for the manager")
    }
}
```

`IDRange` exposes the boundaries as a variable if you need them programmatically:

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func printRanges() {
    fmt.Printf("User range: %d — %d\n", manager.IDRange.UserMin, manager.IDRange.UserMax)
    fmt.Printf("Manager range: %d — %d\n", manager.IDRange.ManagerMin, manager.IDRange.ManagerMax)
}
```

> **Note:** `IDRange.UserMax` is `999,999,899`, the upper bound of `IsValidUserID`. Every ID it accepts is clear of the manager's IDs, custom events included.

### Organising Application IDs

Pick non-overlapping sub-ranges for each concern in your application:

```go
//go:build windows

package main

const (
    // Aircraft telemetry — 1000-1099
    AircraftPositionDefID uint32 = 1000
    AircraftPositionReqID uint32 = 1001
    AircraftVelocityDefID uint32 = 1002
    AircraftVelocityReqID uint32 = 1003

    // Environment — 2000-2099
    WeatherDefID uint32 = 2000
    WeatherReqID uint32 = 2001

    // Traffic — 3000-3099
    TrafficDefID uint32 = 3000
    TrafficReqID uint32 = 3001
)
```

> **Note:** Do not use IDs in the range 999,999,900 — 999,999,999. The manager uses those ranges internally; overlapping with them will silently corrupt your data definitions or event subscriptions.

## State Accessors

The manager exposes several read-only accessors for querying current state without subscribing to change events.

### ConnectionState

Returns the current connection state as a `ConnectionState` value.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func printConnectionState(mgr manager.Manager) {
    switch mgr.ConnectionState() {
    case manager.StateDisconnected:
        fmt.Println("Disconnected")
    case manager.StateConnecting:
        fmt.Println("Connecting...")
    case manager.StateConnected:
        fmt.Println("Connected")
    case manager.StateAvailable:
        fmt.Println("Available (OPEN received)")
    case manager.StateReconnecting:
        fmt.Println("Reconnecting...")
    }
}
```

### SimState

Returns a snapshot of the current `SimState`. The snapshot is a copy; it does not update as the simulator changes.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func printSimState(mgr manager.Manager) {
    s := mgr.SimState()
    fmt.Printf("Paused: %v, SimRunning: %v, Camera: %v\n",
        s.Paused, s.SimRunning, s.Camera)
    fmt.Printf("Position: %.4f, %.4f @ %.0f ft\n",
        s.Latitude, s.Longitude, s.Altitude)
}
```

### Client

Returns the underlying `engine.Client` of the current connection. Use this for operations not exposed directly on the `Manager` interface. Without an engine it returns an interface holding a nil `*engine.Engine`, which does not compare equal to `nil`, so check the connection state instead. A new engine is made per connection: do not keep the client across reconnects.

```go
//go:build windows

package main

import "github.com/mrlm-net/simconnect/pkg/manager"

func useClient(mgr manager.Manager) {
    if mgr.ConnectionState() == manager.StateAvailable {
        client := mgr.Client()
        // Direct engine access when needed
        _ = client
    }
}
```

### Configuration Getters

Inspect the manager's configuration at runtime.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func printConfig(mgr manager.Manager) {
    fmt.Printf("AutoReconnect:     %v\n", mgr.IsAutoReconnect())
    fmt.Printf("RetryInterval:     %v\n", mgr.RetryInterval())
    fmt.Printf("ConnectionTimeout: %v\n", mgr.ConnectionTimeout())
    fmt.Printf("ReconnectDelay:    %v\n", mgr.ReconnectDelay())
    fmt.Printf("ShutdownTimeout:   %v\n", mgr.ShutdownTimeout())
    fmt.Printf("MaxRetries:        %d\n", mgr.MaxRetries())
    fmt.Printf("SimStatePeriod:    %v\n", mgr.SimStatePeriod())
}
```

| Accessor | Returns | Description |
|---|---|---|
| `IsAutoReconnect()` | `bool` | Whether auto-reconnect is enabled |
| `RetryInterval()` | `time.Duration` | Delay between connection attempts |
| `ConnectionTimeout()` | `time.Duration` | Timeout per connection attempt |
| `ReconnectDelay()` | `time.Duration` | Delay before reconnecting after disconnect |
| `ShutdownTimeout()` | `time.Duration` | Maximum wait for subscription drain on stop |
| `MaxRetries()` | `int` | Connection attempt limit (0 = unlimited) |
| `SimStatePeriod()` | `types.SIMCONNECT_PERIOD` | Configured SimState poll frequency |

## See Also

- [Manager Usage](usage-manager.md) — Full usage examples: data requests, system event handlers, channel subscriptions
- [Manager Configuration](config-manager.md) — All configuration options and their defaults
- [Request ID Management](manager-requests-ids.md) — Detailed ID range documentation and conflict resolution
- [Engine/Client Usage](usage-client.md) — Direct engine client API (no reconnect)
- [Event Lifecycle](events-lifecycle.md) — Full event dispatch architecture reference
