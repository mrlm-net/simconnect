---
title: "Manager Configuration"
description: "Configuration options for the connection manager."
order: 3
section: "manager"
---

# Manager Configuration

The `manager` package provides automatic connection lifecycle management with reconnection support. Configuration is done via functional options passed to `manager.New()` or via the root `simconnect` package.

## Quick Start

### Using the Root Package (Recommended)

```go
import (
    "time"
    "github.com/mrlm-net/simconnect"
)

mgr := simconnect.New("MyApp",
    simconnect.WithAutoReconnect(true),
    simconnect.WithRetryInterval(10 * time.Second),
)
```

### Using the Manager Package Directly

```go
import (
    "time"
    "github.com/mrlm-net/simconnect/pkg/manager"
    "github.com/mrlm-net/simconnect/pkg/types"
)

mgr := manager.New("MyApp",
    manager.WithAutoReconnect(true),
    manager.WithRetryInterval(10 * time.Second),
)
```

## Configuration Options

All manager options are available both via the root `simconnect` package (unprefixed) and the `manager` subpackage.

### Manager-Specific Options

| Root Package | Manager Package | Type | Default | Description |
|--------------|-----------------|------|---------|-------------|
| `WithContext(ctx)` | `manager.WithContext(ctx)` | `context.Context` | `context.Background()` | Context for manager lifecycle |
| `WithLogger(logger)` | `manager.WithLogger(logger)` | `*slog.Logger` | Text handler, INFO level | Logger for manager operations |
| `WithLogLevel(level)` | `manager.WithLogLevel(level)` | `slog.Level` | `slog.LevelInfo` | Minimum level for default logger (use `WithLogger` to provide a custom logger) |
| `WithRetryInterval(d)` | `manager.WithRetryInterval(d)` | `time.Duration` | `15s` | Delay between connection attempts |
| `WithConnectionTimeout(d)` | `manager.WithConnectionTimeout(d)` | `time.Duration` | `30s` | Timeout for each connection attempt |
| `WithReconnectDelay(d)` | `manager.WithReconnectDelay(d)` | `time.Duration` | `30s` | Delay before reconnecting after disconnect |
| `WithShutdownTimeout(d)` | `manager.WithShutdownTimeout(d)` | `time.Duration` | `10s` | Timeout for graceful shutdown of subscriptions |
| `WithMaxRetries(n)` | `manager.WithMaxRetries(n)` | `int` | `0` (unlimited) | Maximum connection retries before giving up |
| `WithAutoReconnect(enabled)` | `manager.WithAutoReconnect(enabled)` | `bool` | `true` | Enable automatic reconnection on disconnect |
| `WithSimStatePeriod(period)` | `manager.WithSimStatePeriod(period)` | `types.SIMCONNECT_PERIOD` | `SIMCONNECT_PERIOD_SIM_FRAME` | SimState data request frequency |

### Engine Pass-Through Options

These options configure the underlying engine client:

| Root Package | Manager Package | Type | Default | Description |
|--------------|-----------------|------|---------|-------------|
| `WithBufferSize(size)` | `manager.WithBufferSize(size)` | `int` | `256` | Message buffer size for SimConnect |
| `WithDLLPath(path)` | `manager.WithDLLPath(path)` | `string` | `C:/MSFS 2024 SDK/...` | Path to SimConnect DLL |
| `WithHeartbeat(freq)` | `manager.WithHeartbeat(freq)` | `engine.HeartbeatFrequency` | `engine.HEARTBEAT_6HZ` | Heartbeat frequency |
| `WithEngineOptions(opts...)` | `manager.WithEngineOptions(opts...)` | `...engine.Option` | - | Pass any engine options directly |
| `WithAutoDetect()` | `manager.WithAutoDetect()` | - | disabled | Enable automatic DLL path detection (engine pass-through) |
| `WithLogLevelFromString(level)` | `manager.WithLogLevelFromString(level)` | `string` | - | Set log level from string (engine pass-through) |

> **Note:** `Context` and `Logger` passed via `WithEngineOptions()` will be ignored. The manager controls these settings—use `WithContext()` and `WithLogger()` on the manager instead.

## Option Details

### WithContext

Provides a context for the manager lifecycle. When cancelled, the manager will gracefully shut down.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

mgr := manager.New("MyApp", manager.WithContext(ctx))
```

### WithLogger

Sets a custom structured logger. Both manager and engine default to INFO level.

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

mgr := manager.New("MyApp", manager.WithLogger(logger))
```

### WithLogLevel

Set the minimum level used when the manager constructs a default logger. Manager-level logger overrides engine-level logger when the manager creates engine instances.

```go
mgr := manager.New("MyApp", manager.WithLogLevel(slog.LevelDebug))
// Or from root package
mgr := simconnect.New("MyApp", simconnect.WithLogLevel(slog.LevelDebug))
```

### WithRetryInterval

Sets the fixed delay between connection attempts when the simulator is not available.

```go
manager.WithRetryInterval(10 * time.Second)
```

### WithConnectionTimeout

Sets the timeout for each individual connection attempt.

```go
manager.WithConnectionTimeout(20 * time.Second)
```

### WithReconnectDelay

Sets the delay before attempting to reconnect after the simulator disconnects.

```go
manager.WithReconnectDelay(15 * time.Second)
```

### WithShutdownTimeout

Sets the maximum time to wait for subscriptions to close during shutdown.

```go
manager.WithShutdownTimeout(5 * time.Second)
```

### WithMaxRetries

Limits the number of connection attempts. Set to `0` for unlimited retries.

```go
manager.WithMaxRetries(5)  // Give up after 5 failed attempts
manager.WithMaxRetries(0)  // Retry forever (default)
```

### WithAutoReconnect

Controls whether the manager automatically reconnects when the simulator disconnects.

```go
manager.WithAutoReconnect(true)   // Auto-reconnect (default)
manager.WithAutoReconnect(false)  // Stop after first disconnect
```

### WithSimStatePeriod

Controls how frequently the manager requests SimState data from SimConnect. Lower frequencies reduce CPU usage at the cost of less responsive state change notifications.

```go
import "github.com/mrlm-net/simconnect/pkg/types"

// Default: every simulation frame (~30-60Hz)
manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_SIM_FRAME)

// Once per second (lower CPU, suitable for dashboards)
manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_SECOND)

// Single snapshot (no periodic updates)
manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_ONCE)

// Disable SimState requests entirely
manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_NEVER)
```

| Period | Update Rate | Use Case |
|--------|-------------|----------|
| `SIMCONNECT_PERIOD_SIM_FRAME` | ~30-60Hz | Real-time instruments, HUDs (default) |
| `SIMCONNECT_PERIOD_VISUAL_FRAME` | ~30-60Hz | Visual frame-synced updates |
| `SIMCONNECT_PERIOD_SECOND` | 1Hz | Dashboards, status monitors |
| `SIMCONNECT_PERIOD_ONCE` | Once | Initial state snapshot |
| `SIMCONNECT_PERIOD_NEVER` | Never | Disable automatic state tracking |

### WithBufferSize / WithDLLPath / WithHeartbeat

Convenience wrappers for common engine options:

```go
import "github.com/mrlm-net/simconnect/pkg/engine"

mgr := manager.New("MyApp",
    manager.WithBufferSize(512),
    manager.WithDLLPath("D:/Custom/SimConnect.dll"),
    manager.WithHeartbeat(engine.HEARTBEAT_1SEC),
)
```

### WithEngineOptions

Pass any engine options directly. Useful for less common settings:

```go
import "github.com/mrlm-net/simconnect/pkg/engine"

mgr := manager.New("MyApp",
    manager.WithEngineOptions(
        engine.WithBufferSize(1024),
    ),
)
```

### WithAutoDetect

Enables automatic detection of the SimConnect DLL path for the underlying engine. See [Client Configuration — DLL Auto-Detection](config-client.md#dll-auto-detection) for the full detection strategy.

```go
mgr := manager.New("MyApp", manager.WithAutoDetect())
// Or from root package
mgr := simconnect.New("MyApp", simconnect.WithAutoDetect())
```

### WithLogLevelFromString

Convenience wrapper to set the manager's default logger level from a textual representation:

```go
mgr := manager.New("MyApp", manager.WithLogLevelFromString("debug"))
// Or from root package
mgr := simconnect.New("MyApp", simconnect.WithLogLevelFromString("debug"))
```

Accepted values: `"debug"`, `"info"`, `"warn"`, `"warning"`, `"error"`, `"err"` (case-insensitive). Unknown values default to INFO.

## Configuration Getters

The Manager interface exposes getters for inspecting configuration at runtime:

| Method | Returns | Description |
|--------|---------|-------------|
| `IsAutoReconnect()` | `bool` | Whether auto-reconnect is enabled |
| `RetryInterval()` | `time.Duration` | Delay between connection attempts |
| `ConnectionTimeout()` | `time.Duration` | Timeout per connection attempt |
| `ReconnectDelay()` | `time.Duration` | Delay before reconnecting |
| `ShutdownTimeout()` | `time.Duration` | Graceful shutdown timeout |
| `MaxRetries()` | `int` | Max retries (0 = unlimited) |
| `SimStatePeriod()` | `types.SIMCONNECT_PERIOD` | SimState data request frequency |

```go
if mgr.IsAutoReconnect() {
    fmt.Printf("Will retry every %v\n", mgr.RetryInterval())
}
```

## Simulator State Tracking

The Manager provides access to simulator state through the `SimState()` method, available when the connection is active.

### Accessing Simulator State

```go
if mgr.IsConnected() {
    state := mgr.SimState()
    fmt.Printf("Camera State: %d\n", state.CameraState)
    fmt.Printf("Is Paused: %v\n", state.Paused)
}
```

### SimState Structure

See [Manager Usage - SimState Structure](usage-manager.md#simstate-structure) for the complete field reference.

### Monitoring Simulator State Changes

Subscribe to simulator state changes using `SubscribeSimStateChange()`:

```go
mgr.SubscribeSimStateChange(func(oldState, newState manager.SimState) {
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
})
```

## Example: Full Configuration

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "time"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())

    // Handle Ctrl+C
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)
    go func() {
        <-sigChan
        cancel()
    }()

    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))

    mgr := manager.New("MyApp",
        // Lifecycle
        manager.WithContext(ctx),
        manager.WithLogger(logger),

        // Connection behavior
        manager.WithAutoReconnect(true),
        manager.WithRetryInterval(10 * time.Second),
        manager.WithConnectionTimeout(20 * time.Second),
        manager.WithReconnectDelay(15 * time.Second),
        manager.WithMaxRetries(0),  // Unlimited

        // Shutdown
        manager.WithShutdownTimeout(5 * time.Second),

        // Engine settings
        manager.WithBufferSize(512),
        manager.WithHeartbeat(engine.HEARTBEAT_6HZ),
    )

    mgr.OnStateChange(func(old, new manager.ConnectionState) {
        logger.Info("State changed", "from", old, "to", new)
    })

    if err := mgr.Start(); err != nil {
        logger.Error("Manager stopped", "error", err)
    }
}
```

## Configuration Hierarchy

```
User Application
    │
    ├─► manager.WithContext(ctx) ──────► Manager context (controls lifecycle)
    │                                           │
    │                                           ▼
    │                                    Wrapped in cancel context
    │                                           │
    │                                           ▼
    ├─► manager.WithLogger(log) ───────► Manager logger ──► Engine logger
    │
    ├─► manager.WithRetryInterval() ───► Manager only
    ├─► manager.WithConnectionTimeout() ► Manager only
    ├─► manager.WithReconnectDelay() ──► Manager only
    ├─► manager.WithShutdownTimeout() ─► Manager only
    ├─► manager.WithMaxRetries() ──────► Manager only
    ├─► manager.WithAutoReconnect() ───► Manager only
    ├─► manager.WithSimStatePeriod() ──► Manager only (SimState request period)
    │
    └─► manager.WithBufferSize() ──────► Engine.BufferSize
        manager.WithDLLPath() ─────────► Engine.DLLPath
        manager.WithHeartbeat() ───────► Engine.Heartbeat
        manager.WithAutoDetect() ──────► Engine.AutoDetect
```

## See Also

- [Manager Usage Guide](usage-manager.md) - Complete API reference for lifecycle, subscriptions, and state
- [Request ID Management](manager-requests-ids.md) - ID allocation strategy and conflict prevention
- [Client/Engine Configuration](config-client.md) - For direct engine access without manager
- [Examples](../examples/simconnect-manager) - Working manager example
