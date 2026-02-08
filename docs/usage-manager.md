# Manager Usage

The `manager` package provides automatic connection lifecycle management with reconnection support. This document covers the complete API for robust, long-running SimConnect applications.

> **See also:** [Configuration Options](config-manager.md) for all available options when creating a manager.

## Creating a Manager

### Using the Root Package (Recommended)

```go
import "github.com/mrlm-net/simconnect"

mgr := simconnect.New("MyApp")
```

### Using the Manager Package Directly

```go
import "github.com/mrlm-net/simconnect/pkg/manager"

mgr := manager.New("MyApp")
```

## Connection Lifecycle

### Start

Starts the manager's connection loop. This method **blocks** until the context is cancelled or the manager is stopped.

```go
if err := mgr.Start(); err != nil {
    log.Fatal("Manager stopped with error:", err)
}
```

The manager will:
1. Attempt to connect to SimConnect
2. Dispatch messages while connected
3. Automatically reconnect if the connection drops (when enabled)
4. Continue until `Stop()` is called or context is cancelled

### Stop

Gracefully stops the manager and closes the connection.

```go
mgr.Stop()
```

### ConnectionState

Returns the current connection state.

```go
state := mgr.ConnectionState()
switch state {
case manager.StateDisconnected:
    fmt.Println("Not connected")
case manager.StateConnecting:
    fmt.Println("Connecting...")
case manager.StateConnected:
    fmt.Println("Connected")
case manager.StateReconnecting:
    fmt.Println("Reconnecting...")
case manager.StateStopped:
    fmt.Println("Stopped")
}
```

### Client

Returns the underlying engine client when connected. Returns `nil` when disconnected.

```go
if client := mgr.Client(); client != nil {
    // Use client for SimConnect operations
    client.AddToDataDefinition(...)
}
```

## Callback-Based Event Handlers

The manager provides callback-style handlers for reacting to events. Each returns an ID that can be used to remove the handler.

### OnConnectionStateChange

Called when the connection state changes.

```go
handlerID := mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
    fmt.Printf("State: %v → %v\n", oldState, newState)
    
    if newState == manager.StateConnected {
        // Set up data definitions when connected
        setupDataDefinitions(mgr.Client())
    }
})

// Remove handler when no longer needed
mgr.RemoveConnectionStateChange(handlerID)
```

### OnSimStateChange

Called when the simulator state changes (camera, pause/sim running state, crash and sound flags).

```go
handlerID := mgr.OnSimStateChange(func(oldState, newState manager.SimState) {
    if oldState.Paused != newState.Paused {
        if newState.Paused {
            fmt.Println("Simulator paused")
        } else {
            fmt.Println("Simulator resumed")
        }
    }

    if oldState.SimRunning != newState.SimRunning {
        if newState.SimRunning {
            fmt.Println("Simulator started")
        } else {
            fmt.Println("Simulator stopped")
        }
    }

    if oldState.Camera != newState.Camera {
        fmt.Printf("Camera changed: %v → %v\n", oldState.Camera, newState.Camera)
    }

    if oldState.Crashed != newState.Crashed {
        if newState.Crashed {
            fmt.Println("Simulator reports a crash")
        } else {
            fmt.Println("Crash state reset")
        }
    }

    if oldState.Sound != newState.Sound {
        fmt.Printf("Sound event: id=%d\n", newState.Sound)
    }
})

mgr.RemoveSimStateChange(handlerID)
```

### OnMessage

Called for every SimConnect message received.

```go
handlerID := mgr.OnMessage(func(msg engine.Message) {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
        handleObjectData(msg.AsSimObjectData())
    case types.SIMCONNECT_RECV_ID_EVENT:
        handleEvent(msg.AsEvent())
    }
})

mgr.RemoveMessage(handlerID)
```

### OnOpen

Called when the SimConnect connection opens.

```go
handlerID := mgr.OnOpen(func(data *types.SIMCONNECT_RECV_OPEN) {
    fmt.Printf("Connected to: %s v%d.%d\n",
        engine.ParseNullTerminatedString(data.SzApplicationName[:]),
        data.DwApplicationVersionMajor,
        data.DwApplicationVersionMinor)
})

mgr.RemoveOpen(handlerID)
```

### OnQuit

Called when the simulator closes the connection.

```go
handlerID := mgr.OnQuit(func() {
    fmt.Println("Simulator disconnected")
})

mgr.RemoveQuit(handlerID)
```

## Channel-Based Subscriptions

For more control over message handling, use channel-based subscriptions. These are ideal for goroutine-based architectures.

### Subscribe

Creates a subscription that receives all SimConnect messages.

```go
sub := mgr.Subscribe("my-subscription", 10)  // Buffer size of 10
defer sub.Unsubscribe()

for {
    select {
    case msg := <-sub.Messages():
        handleMessage(msg)
    case <-sub.Done():
        return
    }
}
```

### SubscribeWithFilter

Creates a subscription with a custom filter function.

```go
// Only receive data messages for specific request IDs
sub := mgr.SubscribeWithFilter("position-data", 10, func(msg engine.Message) bool {
    if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
        return false
    }
    data := msg.AsSimObjectData()
    return data.DwRequestID == PositionReqID
})
defer sub.Unsubscribe()
```

### SubscribeWithType

Creates a subscription filtered by message types.

```go
// Only receive object data and event messages
sub := mgr.SubscribeWithType("game-events", 10,
    types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA,
    types.SIMCONNECT_RECV_ID_EVENT,
)
defer sub.Unsubscribe()
```


### Callback-Style System Event Handlers

For simple event handling, the manager provides callback-style handlers that register functions to be invoked when specific events occur:

```go
// Crash event handler
crashID := mgr.OnCrashed(func() {
    fmt.Println("Aircraft crashed!")
})
// Remove handler when no longer needed
mgr.RemoveCrashed(crashID)

// Crash reset handler
resetID := mgr.OnCrashReset(func() {
    fmt.Println("Crash reset")
})
mgr.RemoveCrashReset(resetID)

// Sound event handler
soundID := mgr.OnSoundEvent(func(soundEventID uint32) {
    fmt.Printf("Sound event: %d\n", soundEventID)
})
mgr.RemoveSoundEvent(soundID)

// Flight loaded handler
flightID := mgr.OnFlightLoaded(func(filename string) {
    fmt.Printf("Flight loaded: %s\n", filename)
})
mgr.RemoveFlightLoaded(flightID)

// Aircraft loaded handler
aircraftID := mgr.OnAircraftLoaded(func(filename string) {
    fmt.Printf("Aircraft loaded: %s\n", filename)
})
mgr.RemoveAircraftLoaded(aircraftID)

// Flight plan activated handler
planID := mgr.OnFlightPlanActivated(func(filename string) {
    fmt.Printf("Flight plan activated: %s\n", filename)
})
mgr.RemoveFlightPlanActivated(planID)

// Object added handler
addID := mgr.OnObjectAdded(func(objectID uint32, objType uint32) {
    fmt.Printf("Object added: id=%d type=%d\n", objectID, objType)
})
mgr.RemoveObjectAdded(addID)

// Object removed handler
remID := mgr.OnObjectRemoved(func(objectID uint32, objType uint32) {
    fmt.Printf("Object removed: id=%d type=%d\n", objectID, objType)
})
mgr.RemoveObjectRemoved(remID)
```

### Typed System Event Subscriptions

For more control over event handling (e.g., goroutine-based processing), use channel-based subscriptions. The manager exposes convenience subscriptions for common system events (wrapping message subscriptions and delivering typed payloads):

- `SubscribeOnFlightLoaded(id, bufferSize)` — delivers `FilenameEvent` with the loaded flight filename.
- `SubscribeOnAircraftLoaded(id, bufferSize)` — delivers `FilenameEvent` with the loaded aircraft `.AIR` filename.
- `SubscribeOnFlightPlanActivated(id, bufferSize)` — delivers `FilenameEvent` with the activated flight plan filename.
- `SubscribeOnObjectAdded(id, bufferSize)` — delivers `ObjectEvent` when an AI object is added (contains `ObjectID` and `ObjType`).
- `SubscribeOnObjectRemoved(id, bufferSize)` — delivers `ObjectEvent` when an AI object is removed (contains `ObjectID` and `ObjType`).
- `SubscribeOnCrashed(id, bufferSize)` — delivers raw `engine.Message` for the `Crashed` system event (filter pre-applied).
- `SubscribeOnCrashReset(id, bufferSize)` — delivers raw `engine.Message` for the `Crash Reset` system event.
- `SubscribeOnSoundEvent(id, bufferSize)` — delivers raw `engine.Message` for the `Sound` system event (sound ID available in `DwData`).

Example — receive flight-loaded notifications:

```go
sub := mgr.SubscribeOnFlightLoaded("flight-load", 5)
defer sub.Unsubscribe()

for {
    select {
    case ev := <-sub.Events():
        fmt.Printf("Flight loaded: %s\n", ev.Filename)
    case <-sub.Done():
        return
    }
}
```

Example — subscribe to crash/sound events (raw message subscription delivered by helpers):

```go
subCrash := mgr.SubscribeOnCrashed("crash-sub", 4)
defer subCrash.Unsubscribe()

go func() {
    for msg := range subCrash.Messages() {
        ev := msg.AsEvent()
        if ev != nil {
            fmt.Printf("[sub] Crashed event: data=%d\n", ev.DwData)
        }
    }
}()

subSound := mgr.SubscribeOnSoundEvent("sound-sub", 4)
defer subSound.Unsubscribe()

go func() {
    for msg := range subSound.Messages() {
        ev := msg.AsEvent()
        if ev != nil {
            fmt.Printf("[sub] Sound event id=%d data=%d\n", ev.UEventID, ev.DwData)
        }
    }
}()
```

Example — monitor AI object add/remove events:

```go
subAdd := mgr.SubscribeOnObjectAdded("obj-add", 20)
defer subAdd.Unsubscribe()
subRem := mgr.SubscribeOnObjectRemoved("obj-rem", 20)
defer subRem.Unsubscribe()

go func() {
    for ev := range subAdd.Events() {
        fmt.Printf("Object added: id=%d type=%d\n", ev.ObjectID, ev.ObjType)
    }
}()

for ev := range subRem.Events() {
    fmt.Printf("Object removed: id=%d type=%d\n", ev.ObjectID, ev.ObjType)
}
```

Notes:
- These helpers filter and forward the appropriate SimConnect message types. They are safe to use concurrently and will automatically cancel when the manager stops.
- The manager already subscribes to the corresponding SimConnect system events on connection open; these helpers simply provide typed channels for consumers.

### GetSubscription

Retrieves an existing subscription by ID.

```go
if sub := mgr.GetSubscription("my-subscription"); sub != nil {
    // Use existing subscription
}
```

### Subscription Interface

All subscriptions implement the `Subscription` interface:

| Method | Returns | Description |
|--------|---------|-------------|
| `Messages()` | `<-chan engine.Message` | Channel receiving messages |
| `Done()` | `<-chan struct{}` | Closed when subscription ends |
| `Unsubscribe()` | - | Stops and removes the subscription |
| `ID()` | `string` | Subscription identifier |

## State Subscriptions

Specialized subscriptions for state changes.

### SubscribeConnectionStateChange

```go
sub := mgr.SubscribeConnectionStateChange("conn-monitor", 5)
defer sub.Unsubscribe()

for {
    select {
    case change := <-sub.Changes():
        fmt.Printf("Connection: %v → %v\n", change.Old, change.New)
    case <-sub.Done():
        return
    }
}
```

### SubscribeSimStateChange

```go
sub := mgr.SubscribeSimStateChange("sim-monitor", 5)
defer sub.Unsubscribe()

for {
    select {
    case change := <-sub.Changes():
        if change.New.Paused {
            pauseDataCollection()
        } else {
            resumeDataCollection()
        }
    case <-sub.Done():
        return
    }
}
```

### SubscribeOnOpen

```go
sub := mgr.SubscribeOnOpen("open-monitor", 5)
defer sub.Unsubscribe()

for {
    select {
    case openData := <-sub.Opens():
        initializeConnection(openData)
    case <-sub.Done():
        return
    }
}
```

### SubscribeOnQuit

```go
sub := mgr.SubscribeOnQuit("quit-monitor", 5)
defer sub.Unsubscribe()

for {
    select {
    case <-sub.Quits():
        cleanupResources()
    case <-sub.Done():
        return
    }
}
```

## Simulator State


### SimState

Returns the current simulator state when connected.

```go
state := mgr.SimState()
fmt.Printf("Camera: %v, Paused: %v, Crashed: %v\n", state.Camera, state.Paused, state.Crashed)
```

### SimState Structure

| Field | Type | Description |
|-------|------|-------------|
| `Camera` | `CameraState` | Current camera mode |
| `Substate` | `CameraSubstate` | Camera substate |
| `Paused` | `bool` | Whether simulation is paused |
| `SimRunning` | `bool` | Whether simulation is running |
| `SimulationRate` | `float64` | Current simulation rate multiplier |
| `SimulationTime` | `float64` | Seconds since simulation start |
| `LocalTime` | `float64` | Seconds since midnight (local) |
| `ZuluTime` | `float64` | Seconds since midnight (Zulu/UTC) |
| `IsInVR` | `bool` | Whether user is in VR mode |
| `IsUsingMotionControllers` | `bool` | Motion controllers active |
| `IsUsingJoystickThrottle` | `bool` | Joystick throttle active |
| `IsInRTC` | `bool` | In real-time communication |
| `IsAvatar` | `bool` | Avatar mode active |
| `IsAircraft` | `bool` | Controlling an aircraft |
| `Crashed` | `bool` | Crash state reported by sim |
| `CrashReset` | `bool` | Crash reset flag |
| `Sound` | `uint32` | Last sound event ID |
| `LocalDay` | `int` | Local day of month |
| `LocalMonth` | `int` | Local month of year |
| `LocalYear` | `int` | Local year |
| `ZuluDay` | `int` | Zulu day of month |
| `ZuluMonth` | `int` | Zulu month of year |
| `ZuluYear` | `int` | Zulu year |
| `Realism` | `float64` | Realism setting value |
| `VisualModelRadius` | `float64` | Visual model radius (meters) |
| `SimDisabled` | `bool` | Simulation disabled flag |
| `RealismCrashDetection` | `bool` | Crash detection enabled |
| `RealismCrashWithOthers` | `bool` | Crash with others enabled |
| `TrackIREnabled` | `bool` | TrackIR head tracking enabled |
| `UserInputEnabled` | `bool` | User input enabled |
| `SimOnGround` | `bool` | Aircraft is on ground |

### Camera States

Common camera state values (see `pkg/manager/state.go` for the complete list of 20+ camera states):

| Value | Constant | Description |
|-------|----------|-------------|
| 2 | `CameraStateCockpit` | Cockpit view |
| 3 | `CameraStateExternalChase` | External/chase view |
| 4 | `CameraStateDrone` | Drone camera |
| 5 | `CameraStateFixedOnPlane` | Fixed on plane |
| 6 | `CameraStateEnvironment` | Environment camera |
| 7 | `CameraStateSixDoF` | Six degrees of freedom |
| 8 | `CameraStateGameplay` | Gameplay camera |
| 9 | `CameraStateShowcase` | Showcase mode |
| 10 | `CameraStateDroneAircraft` | Drone Aircraft |
| 12 | `CameraStateWorldMap` | World Map |
| 17 | `CameraStateReplay` | Replay |

## Configuration Getters

Inspect the manager's configuration at runtime:

```go
fmt.Printf("Auto-reconnect: %v\n", mgr.IsAutoReconnect())
fmt.Printf("Retry interval: %v\n", mgr.RetryInterval())
fmt.Printf("Connection timeout: %v\n", mgr.ConnectionTimeout())
fmt.Printf("Reconnect delay: %v\n", mgr.ReconnectDelay())
fmt.Printf("Shutdown timeout: %v\n", mgr.ShutdownTimeout())
fmt.Printf("Max retries: %d\n", mgr.MaxRetries())
```

## ID Management

The manager reserves IDs 999,000-999,999 for internal use. See [Request ID Management](manager-requests-ids.md) for details.

### Validating User IDs

```go
const MyDataDefID = 1000

if !manager.IsValidUserID(MyDataDefID) {
    log.Fatal("ID conflicts with manager reserved range")
}
```

## Dataset Registration

The manager exposes dataset registration methods directly, eliminating the need to access the underlying client for common data definition operations. All methods return `manager.ErrNotConnected` if called when not connected.

### RegisterDataset

Registers a complete pre-built dataset with SimConnect.

```go
import "github.com/mrlm-net/simconnect/pkg/datasets/traffic"

mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    if new == manager.StateConnected {
        // Register a pre-built dataset directly on the manager
        if err := mgr.RegisterDataset(DataDefID, traffic.NewAircraftDataset()); err != nil {
            log.Printf("Failed to register dataset: %v", err)
        }
    }
})
```

### AddToDataDefinition

Adds individual data definitions to a definition group.

```go
mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
    if new == manager.StateConnected {
        mgr.AddToDataDefinition(DataDefID, "PLANE LATITUDE", "degrees", 
            types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
        mgr.AddToDataDefinition(DataDefID, "PLANE LONGITUDE", "degrees", 
            types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
        mgr.AddToDataDefinition(DataDefID, "PLANE ALTITUDE", "feet", 
            types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
    }
})
```

### RequestDataOnSimObject

Requests periodic data for a specific object (e.g., user aircraft).

```go
err := mgr.RequestDataOnSimObject(
    DataReqID,                              // Request ID
    DataDefID,                              // Definition ID
    types.SIMCONNECT_OBJECT_ID_USER,        // User aircraft
    types.SIMCONNECT_PERIOD_SECOND,         // Update every second
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, // Only when changed
    0, 0, 0,                                // origin, interval, limit
)
if err != nil {
    if errors.Is(err, manager.ErrNotConnected) {
        log.Println("Not connected, will retry later")
    }
}
```

### RequestDataOnSimObjectType

Requests data for all objects of a type within a radius.

```go
// Request data for all aircraft within 25km
err := mgr.RequestDataOnSimObjectType(
    TrafficReqID,
    TrafficDefID,
    25000, // radius in meters
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT,
)
```

### ClearDataDefinition

Clears all definitions from a definition group.

```go
err := mgr.ClearDataDefinition(DataDefID)
```

### SetDataOnSimObject

Sets data values on a simulation object.

```go
type AircraftPosition struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}

pos := AircraftPosition{
    Latitude:  47.4502,
    Longitude: -122.3088,
    Altitude:  5000,
}

err := mgr.SetDataOnSimObject(
    DataDefID,
    types.SIMCONNECT_OBJECT_ID_USER,
    0, // flags
    0, // array count (0 for single object)
    uint32(unsafe.Sizeof(pos)),
    unsafe.Pointer(&pos),
)
```

### Dataset Methods Summary

| Method | Description |
|--------|-------------|
| `RegisterDataset(defID, dataset)` | Register a complete dataset |
| `AddToDataDefinition(...)` | Add single data definition |
| `RequestDataOnSimObject(...)` | Request data for specific object |
| `RequestDataOnSimObjectType(...)` | Request data for object type |
| `ClearDataDefinition(defID)` | Clear all definitions |
| `SetDataOnSimObject(...)` | Set data on an object |

All methods return `manager.ErrNotConnected` when not connected. Use `errors.Is()` to check:

```go
if errors.Is(err, manager.ErrNotConnected) {
    // Handle not connected state
}
```

## Example: Complete Application

```go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "time"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

type AircraftData struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}

const (
    DataDefID = 1000
    DataReqID = 1001
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())

    // Handle Ctrl+C
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)
    go func() {
        <-sigChan
        fmt.Println("\nShutting down...")
        cancel()
    }()

    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    mgr := manager.New("MyApp",
        manager.WithContext(ctx),
        manager.WithLogger(logger),
        manager.WithAutoReconnect(true),
        manager.WithRetryInterval(10*time.Second),
    )

    // Set up data definitions when connected
    mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
        if new == manager.StateConnected {
            setupDataDefinitions(mgr)
        }
    })

    // Handle incoming data
    mgr.OnMessage(func(msg engine.Message) {
        if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
            data := msg.AsSimObjectData()
            if data.DwRequestID == DataReqID {
                aircraft := engine.CastDataAs[AircraftData](&data.DwData)
                fmt.Printf("Position: %.4f, %.4f @ %.0fft\n",
                    aircraft.Latitude, aircraft.Longitude, aircraft.Altitude)
            }
        }
    })

    // React to pause state
    mgr.OnSimStateChange(func(old, new manager.SimState) {
        if old.Paused != new.Paused {
            if new.Paused {
                fmt.Println("⏸ Paused")
            } else {
                fmt.Println("▶ Resumed")
            }
        }
    })

    // Start the manager (blocks until context cancelled)
    if err := mgr.Start(); err != nil {
        logger.Error("Manager stopped", "error", err)
    }
}

func setupDataDefinitions(mgr manager.Manager) {
    mgr.AddToDataDefinition(DataDefID, "PLANE LATITUDE", "degrees", 
        types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    mgr.AddToDataDefinition(DataDefID, "PLANE LONGITUDE", "degrees", 
        types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
    mgr.AddToDataDefinition(DataDefID, "PLANE ALTITUDE", "feet", 
        types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)

    mgr.RequestDataOnSimObject(
        DataReqID, DataDefID,
        types.SIMCONNECT_OBJECT_ID_USER,
        types.SIMCONNECT_PERIOD_SECOND,
        types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
        0, 0, 0,
    )
}
```

## Example: Channel-Based Processing

```go
package main

import (
    "context"
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    mgr := manager.New("ChannelApp", manager.WithContext(ctx))

    // Use channel-based subscriptions
    go processPositionData(mgr)
    go monitorConnectionState(mgr)

    mgr.Start()
}

func processPositionData(mgr manager.Manager) {
    sub := mgr.SubscribeWithType("position", 100,
        types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA,
    )
    defer sub.Unsubscribe()

    for {
        select {
        case msg := <-sub.Messages():
            data := msg.AsSimObjectData()
            // Process position data...
            fmt.Printf("Received data for request %d\n", data.DwRequestID)
        case <-sub.Done():
            return
        }
    }
}

func monitorConnectionState(mgr manager.Manager) {
    sub := mgr.SubscribeConnectionStateChange("state-monitor", 10)
    defer sub.Unsubscribe()

    for {
        select {
        case change := <-sub.Changes():
            fmt.Printf("Connection state: %v → %v\n", change.Old, change.New)
        case <-sub.Done():
            return
        }
    }
}
```

## See Also

- [Manager Configuration](config-manager.md) — All configuration options
- [Client Usage](usage-client.md) — Direct engine client API
- [Request ID Management](manager-requests-ids.md) — ID allocation strategy
- [Examples](../examples/simconnect-manager) — Working code samples
