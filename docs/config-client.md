# Engine/Client Configuration

The `engine` package provides a high-level client for interacting with SimConnect. Configuration is done via functional options passed to `engine.New()` or via the root `simconnect` package with `Client` prefixed options.

## Quick Start

### Using the Root Package

```go
import "github.com/mrlm-net/simconnect"

client := simconnect.NewClient("MyApp",
    simconnect.ClientWithBufferSize(512),
    simconnect.ClientWithDLLPath("C:/Custom/Path/SimConnect.dll"),
    simconnect.ClientWithHeartbeat(simconnect.HEARTBEAT_6HZ),
)
```

### Using the Engine Package Directly

```go
import "github.com/mrlm-net/simconnect/pkg/engine"

client := engine.New("MyApp",
    engine.WithBufferSize(512),
    engine.WithDLLPath("C:/Custom/Path/SimConnect.dll"),
    engine.WithHeartbeat(engine.HEARTBEAT_6HZ),
)
```

## Configuration Options

Client options are available via the root `simconnect` package (with `Client` prefix) and the `engine` subpackage.

| Root Package | Engine Package | Type | Default | Description |
|--------------|----------------|------|---------|-------------|
| `ClientWithBufferSize(size)` | `engine.WithBufferSize(size)` | `int` | `256` | Size of the message buffer for SimConnect communication |
| `ClientWithDLLPath(path)` | `engine.WithDLLPath(path)` | `string` | `C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll` | Path to the SimConnect DLL |
| `ClientWithContext(ctx)` | `engine.WithContext(ctx)` | `context.Context` | `context.Background()` | Context for lifecycle management |
| `ClientWithLogger(logger)` | `engine.WithLogger(logger)` | `*slog.Logger` | Text handler, INFO level | Logger for engine operations |
| `ClientWithLogLevel(level)` | `engine.WithLogLevel(level)` | `slog.Level` | `slog.LevelInfo` | Minimum level for default logger (use `ClientWithLogger` to provide a custom logger) |
| `ClientWithHeartbeat(freq)` | `engine.WithHeartbeat(freq)` | `engine.HeartbeatFrequency` | `engine.HEARTBEAT_6HZ` | Heartbeat frequency for connection monitoring |

## Option Details

### WithBufferSize

Sets the internal buffer size for receiving SimConnect messages. Increase this value if you're subscribing to high-frequency data or multiple data definitions simultaneously.

```go
engine.WithBufferSize(512)
```

### WithDLLPath

Specifies a custom path to the SimConnect DLL. Useful when the SDK is installed in a non-standard location.

```go
engine.WithDLLPath("D:/MSFS SDK/SimConnect SDK/lib/SimConnect.dll")
```

### WithContext

Provides a context for the engine lifecycle. When the context is cancelled, the engine will gracefully shut down.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

client := engine.New("MyApp", engine.WithContext(ctx))
```

### WithLogger

Sets a custom structured logger for engine operations. The engine logs at INFO level by default.

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client := engine.New("MyApp", engine.WithLogger(logger))
```

### WithLogLevel

Set the minimum level used when the engine constructs a default logger. This is a convenience so callers don't need to instantiate a full `slog.Logger` to control verbosity.

```go
client := engine.New("MyApp", engine.WithLogLevel(slog.LevelDebug))
// Or from root package
client := simconnect.NewClient("MyApp", simconnect.ClientWithLogLevel(slog.LevelDebug))
```

### WithHeartbeat

Configures the heartbeat frequency for monitoring the connection to the simulator. Use the typed constants from `pkg/engine` or the top-level `simconnect` package.

```go
engine.WithHeartbeat(engine.HEARTBEAT_1SEC)  // Check every second
engine.WithHeartbeat(engine.HEARTBEAT_6HZ)   // Check 6 times per second (default)

// Or from the root package:
simconnect.ClientWithHeartbeat(simconnect.HEARTBEAT_1SEC)
simconnect.ClientWithHeartbeat(simconnect.HEARTBEAT_6HZ)
```

## Underlying SimConnect Config

The engine embeds the internal SimConnect configuration:

| Field | Type | Description |
|-------|------|-------------|
| `BufferSize` | `int` | Message buffer size |
| `Context` | `context.Context` | Lifecycle context |
| `DLLPath` | `string` | Path to SimConnect.dll |

## Example: Full Configuration

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/mrlm-net/simconnect/pkg/engine"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))

    client := engine.New("MyApp",
        engine.WithContext(ctx),
        engine.WithLogger(logger),
        engine.WithBufferSize(512),
        engine.WithDLLPath("C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"),
        engine.WithHeartbeat(engine.HEARTBEAT_6HZ),
    )

    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    // Use client...
}
```

## See Also

- [Client Usage Guide](usage-client.md) - Complete API reference for data, events, AI, and facilities
- [Manager Configuration](config-manager.md) - For automatic connection lifecycle management
- [Examples](../examples) - Working code samples
