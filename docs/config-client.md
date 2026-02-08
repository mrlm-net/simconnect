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
| `ClientWithAutoDetect()` | `engine.WithAutoDetect()` | - | disabled | Enable automatic SimConnect DLL path detection |
| `ClientWithLogLevelFromString(level)` | `engine.WithLogLevelFromString(level)` | `string` | - | Set log level from string ("debug", "info", "warn", "error") |

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

### WithAutoDetect

Enables automatic detection of the SimConnect DLL path. When enabled, the engine searches for the DLL in this priority order:

0. `SIMCONNECT_DLL` environment variable (direct path to DLL)
1. SDK root environment variables: `MSFS_SDK`, `MSFS2024_SDK`, `MSFS2020_SDK`
2. Common installation paths (C:/MSFS 2024 SDK, C:/Program Files/..., etc.)
3. User home directory (~)

Within each SDK root, both `SimConnect SDK/lib/SimConnect.dll` and `lib/SimConnect.dll` layouts are checked.

```go
// Auto-detect (recommended for most users)
client := engine.New("MyApp", engine.WithAutoDetect())

// Or from root package
client := simconnect.NewClient("MyApp", simconnect.ClientWithAutoDetect())
```

When enabled, detection runs regardless of any path set via `WithDLLPath`. If detection succeeds, the detected path overrides the configured path. If detection fails, the existing path (default or explicit) is kept as fallback.

### WithLogLevelFromString

Convenience function to set log level from a textual representation:

```go
engine.WithLogLevelFromString("debug")  // Same as WithLogLevel(slog.LevelDebug)
engine.WithLogLevelFromString("warn")   // Same as WithLogLevel(slog.LevelWarn)
```

Accepted values: `"debug"`, `"info"`, `"warn"`, `"warning"`, `"error"`, `"err"` (case-insensitive). Unknown values default to INFO.

## Underlying SimConnect Config

The engine embeds the internal SimConnect configuration:

| Field | Type | Description |
|-------|------|-------------|
| `BufferSize` | `int` | Message buffer size |
| `Context` | `context.Context` | Lifecycle context |
| `DLLPath` | `string` | Path to SimConnect.dll |

## DLL Auto-Detection

The package provides automatic DLL detection to simplify setup across different MSFS installations.

### Public API

You can detect the DLL path without creating a client:

```go
import "github.com/mrlm-net/simconnect"

dllPath, err := simconnect.DetectDLLPath()
if err != nil {
    if errors.Is(err, simconnect.ErrDLLNotFound) {
        log.Fatal("SimConnect DLL not found — install MSFS SDK or set SIMCONNECT_DLL env var")
    }
}
fmt.Println("Found DLL at:", dllPath)
```

### Detection Priority

| Priority | Source | Example |
|----------|--------|---------|
| 0 | `SIMCONNECT_DLL` env var | `C:/Custom/SimConnect.dll` |
| 1 | SDK root env vars (`MSFS_SDK`, `MSFS2024_SDK`, `MSFS2020_SDK`) | `%MSFS_SDK%/SimConnect SDK/lib/SimConnect.dll` |
| 2 | Common installation paths | `C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll` |
| 3 | User home directory | `~/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll` |

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
