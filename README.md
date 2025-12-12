# mrlm-net/simconnect

[![Go Reference](https://pkg.go.dev/badge/github.com/mrlm-net/simconnect.svg)](https://pkg.go.dev/github.com/mrlm-net/simconnect)
[![License](https://img.shields.io/github/license/mrlm-net/simconnect)](LICENSE)

> **GoLang wrapper over SimConnect.dll SDK** — build Microsoft Flight Simulator add-ons with ease.

The main aim of this package is to abstract and describe common use-cases for your add-on application. We strive to keep it **small**, **simple**, **intuitive**, and **performant**. Feel free to contribute if there is something you are missing!

## Features

- Lightweight abstraction over `SimConnect.dll`
- Typed data structures and enums for safer code
- Ready-to-use dataset definitions
- Comprehensive examples covering most scenarios

## Requirements

| Requirement | Version |
|-------------|---------|
| Go | 1.25+ |
| Operating system | Windows |
| Microsoft Flight Simulator | 2020 / 2024 |
| SimConnect SDK | Bundled with MSFS |

## Installation

```shell
go get github.com/mrlm-net/simconnect
```

## Quick Start

```go
package main

import (
    "github.com/mrlm-net/simconnect"
)

func main() {
    client := simconnect.NewClient("MyApp")
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()
    // … interact with the simulator
}
```

## Examples

| Example | Description |
|---------|-------------|
| [`basic-connection`](examples/basic-connection) | Minimal setup that connects to the simulator and prints status updates. |
| [`await-connection`](examples/await-connection) | Demonstrates waiting for the SimConnect server and retrying the connection sequence. |
| [`lifecycle-connection`](examples/lifecycle-connection) | Showcases clean connection lifecycle management including graceful shutdown. |
| [`simconnect-manager`](examples/simconnect-manager) | Uses the Manager interface for automatic connection lifecycle and reconnection handling. |
| [`simconnect-subscribe`](examples/simconnect-subscribe) | Demonstrates channel-based subscriptions for messages and state changes using the Manager interface. |
| [`read-messages`](examples/read-messages) | Reads incoming SimConnect messages and displays their payloads. |
| [`read-objects`](examples/read-objects) | Retrieves simulator objects and inspects their properties. |
| [`set-variables`](examples/set-variables) | Writes data back to the simulator to control aircraft state. |
| [`emit-events`](examples/emit-events) | Sends custom events into the simulator. |
| [`subscribe-events`](examples/subscribe-events) | Listens to simulator events and reacts to them. |
| [`subscribe-facilities`](examples/subscribe-facilities) | Subscribes to facility data streams. |
| [`read-facility`](examples/read-facility) | Resolves a single facility by ICAO identifier. |
| [`read-facilities`](examples/read-facilities) | Enumerates facilities matching a filter. |
| [`ai-traffic`](examples/ai-traffic) | Drives AI traffic plans using flight plans shipped in `examples/ai-traffic/plans`. |

> **Tip:** Browse the [`examples`](examples) folder to explore additional scenarios as they are added.

## Packages

| Package | Description |
|---------|-------------|
| `simconnect` (root) | Main entry point providing `New()` for a managed connection (returns `manager.Manager`) and `NewClient()` for direct engine access (returns `engine.Client`). |
| [`pkg/engine`](pkg/engine) | High-level client that manages the SimConnect session lifecycle, message dispatching, and data subscriptions. Use this when you want batteries-included helpers around the lower-level API. |
| [`pkg/manager`](pkg/manager) | Connection lifecycle manager with automatic reconnection support. Ideal for long-running services that need robust connection handling. |
| [`pkg/types`](pkg/types) | Strongly typed representations of SimConnect data structures, events, and helper enums used across the public API. |
| [`pkg/datasets`](pkg/datasets) | Ready-made dataset definitions that describe common SimConnect data requests, providing reusable building blocks for your own subscriptions. |
| [`pkg/convert`](pkg/convert) | Utility helpers to convert between Go-native types and SimConnect-specific formats when marshalling data in and out of requests. |

> Detailed documentation will be available in the `docs` folder.

## Configuration

Both the `engine.Client` and `manager.Manager` support configuration via functional options:

```go
// Direct engine client
client := engine.New("MyApp",
    engine.WithBufferSize(512),
    engine.WithHeartbeat("6Hz"),
)

// Managed connection with auto-reconnect
mgr := manager.New("MyApp",
    manager.WithAutoReconnect(true),
    manager.WithRetryInterval(10 * time.Second),
    manager.WithBufferSize(512),
)
```

| Documentation | Description |
|---------------|-------------|
| [Engine/Client Config](docs/config-client.md) | Buffer size, DLL path, heartbeat, logging |
| [Manager Config](docs/config-manager.md) | Auto-reconnect, retry intervals, timeouts, plus all engine options |

## Contributing

_Contributions are welcomed and must follow the [Code of Conduct](https://github.com/mrlm-net/simconnect?tab=coc-ov-file) and common [Contribution guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md)._

> If you'd like to report a security issue please follow the [security guidelines](https://github.com/mrlm-net/simconnect?tab=security-ov-file).

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

---

<sup><sub>_All rights reserved © Martin Hrášek [<@mrlm-xyz>](https://github.com/mrlm-xyz) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)_</sub></sup>