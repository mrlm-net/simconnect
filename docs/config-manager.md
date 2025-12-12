# Manager Configuration

The `manager` package provides automatic connection lifecycle management with reconnection support. Configuration is done via functional options passed to `manager.New()`.

## Quick Start

```go
import "github.com/mrlm-net/simconnect/pkg/manager"

mgr := manager.New("MyApp",
    manager.WithAutoReconnect(true),
    manager.WithRetryInterval(10 * time.Second),
)
```

## Configuration Options

### Manager-Specific Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithContext(ctx)` | `context.Context` | `context.Background()` | Context for manager lifecycle |
| `WithLogger(logger)` | `*slog.Logger` | Text handler, INFO level | Logger for manager operations |
| `WithRetryInterval(d)` | `time.Duration` | `15s` | Delay between connection attempts |
| `WithConnectionTimeout(d)` | `time.Duration` | `30s` | Timeout for each connection attempt |
| `WithReconnectDelay(d)` | `time.Duration` | `30s` | Delay before reconnecting after disconnect |
| `WithShutdownTimeout(d)` | `time.Duration` | `10s` | Timeout for graceful shutdown of subscriptions |
| `WithMaxRetries(n)` | `int` | `0` (unlimited) | Maximum connection retries before giving up |
| `WithAutoReconnect(enabled)` | `bool` | `true` | Enable automatic reconnection on disconnect |

### Engine Pass-Through Options

These options configure the underlying engine client:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithBufferSize(size)` | `int` | `256` | Message buffer size for SimConnect |
| `WithDLLPath(path)` | `string` | `C:/MSFS 2024 SDK/...` | Path to SimConnect DLL |
| `WithHeartbeat(freq)` | `string` | `"6Hz"` | Heartbeat frequency |
| `WithEngineOptions(opts...)` | `...engine.Option` | - | Pass any engine options directly |

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

Sets a custom structured logger. The manager logs at INFO level by default, which is more verbose than the engine's WARN default.

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

mgr := manager.New("MyApp", manager.WithLogger(logger))
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

### WithBufferSize / WithDLLPath / WithHeartbeat

Convenience wrappers for common engine options:

```go
mgr := manager.New("MyApp",
    manager.WithBufferSize(512),
    manager.WithDLLPath("D:/Custom/SimConnect.dll"),
    manager.WithHeartbeat("1Hz"),
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

```go
if mgr.IsAutoReconnect() {
    fmt.Printf("Will retry every %v\n", mgr.RetryInterval())
}
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
        manager.WithHeartbeat("6Hz"),
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
    │
    └─► manager.WithBufferSize() ──────► Engine.BufferSize
        manager.WithDLLPath() ─────────► Engine.DLLPath
        manager.WithHeartbeat() ───────► Engine.Heartbeat
```

## See Also

- [Client/Engine Configuration](config-client.md) - For direct engine access without manager
- [Examples](../examples/simconnect-manager) - Working manager example
