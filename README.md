# mrlm-net/simconnect

[![Go Reference](https://pkg.go.dev/badge/github.com/mrlm-net/simconnect.svg)](https://pkg.go.dev/github.com/mrlm-net/simconnect)
[![Documentation](https://img.shields.io/badge/docs-simconnect.mrlm.net-blue)](https://simconnect.mrlm.net/)

> **Go wrapper for SimConnect.dll** — build Microsoft Flight Simulator 2020/2024 add-ons with type-safe, zero-dependency Go code.

## Features

### Engine — Direct SimConnect Access
- Full SimConnect DLL binding via syscalls — zero CGo, Windows-only
- Typed message stream via Go channels with callback handlers
- Manual and pre-built dataset definitions across 6 domains (aircraft, environment, facilities, objects, simulator, traffic)
- AI traffic management — create, remove, and control parked, enroute, and non-ATC aircraft with livery selection
- Facility data queries — airports, runways, parking, frequencies, VOR, NDB, waypoints, jetways, helipads, and 14 facility types total
- SimConnect.dll auto-detection via environment variables, SDK paths, and common installation locations

### Manager — Production Lifecycle
- Automatic connection lifecycle with reconnection and health monitoring
- 60+ SimVar state tracking via SimState — camera, position, speed, weather, VR, environment, realism settings
- Channel-based and callback-based subscriptions with message type filtering
- 10 typed system event handlers — Pause, Sim, Crashed, CrashReset, Sound, FlightLoaded, AircraftLoaded, FlightPlanActivated, ObjectAdded, ObjectRemoved

### Utilities
- Great-circle distance (haversine), altitude/distance/speed conversions, ICAO validation, WGS84 coordinate offsets
- Tiered buffer pooling and pre-allocated handler buffers for zero-allocation dispatching

*Zero external dependencies — standard library only.*

## Quick Start

```go
package main

import (
    "github.com/mrlm-net/simconnect"
)

func main() {
    client := simconnect.NewClient("MyApp", simconnect.ClientWithHeartbeat(simconnect.HEARTBEAT_6HZ))
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()
    // … interact with the simulator
}
```

## Examples

All examples include standalone `main` packages with individual READMEs. Browse the [`examples`](examples) folder or run any example directly:

```shell
go run ./examples/<name>
```

- **Getting Started** — [basic-connection](examples/basic-connection), [await-connection](examples/await-connection), [lifecycle-connection](examples/lifecycle-connection)
- **Manager Interface** — [simconnect-manager](examples/simconnect-manager), [simconnect-subscribe](examples/simconnect-subscribe), [simconnect-state](examples/simconnect-state), [simconnect-events](examples/simconnect-events)
- **Data Operations** — [read-messages](examples/read-messages), [read-objects](examples/read-objects), [set-variables](examples/set-variables), [emit-events](examples/emit-events), [subscribe-events](examples/subscribe-events), [using-datasets](examples/using-datasets)
- **Facilities & Navigation** — [subscribe-facilities](examples/subscribe-facilities), [read-facility](examples/read-facility), [read-facilities](examples/read-facilities), [read-waypoints](examples/read-waypoints), [all-facilities](examples/all-facilities), [airport-details](examples/airport-details), [locate-airport](examples/locate-airport), [simconnect-facilities](examples/simconnect-facilities)
- **AI Traffic** — [ai-traffic](examples/ai-traffic), [manage-traffic](examples/manage-traffic), [monitor-traffic](examples/monitor-traffic), [simconnect-traffic](examples/simconnect-traffic)
- **Performance** — [simconnect-benchmark](examples/simconnect-benchmark)
- **Tools** — [simvar-cli](examples/simvar-cli)

## Documentation

**[simconnect.mrlm.net](https://simconnect.mrlm.net/)** — Full documentation website with getting started guide, configuration reference, and usage guides.

- [Client Configuration](https://simconnect.mrlm.net/docs/config-client) — Engine/Client functional options
- [Client API Reference](https://simconnect.mrlm.net/docs/usage-client) — Complete Engine/Client API
- [Manager Configuration](https://simconnect.mrlm.net/docs/config-manager) — Manager functional options
- [Manager Usage](https://simconnect.mrlm.net/docs/usage-manager) — Lifecycle management, subscriptions, state handling
- [Request ID Management](https://simconnect.mrlm.net/docs/manager-requests-ids) — ID allocation strategy and conflict prevention
- [Event Lifecycle](https://simconnect.mrlm.net/docs/events-lifecycle) — Event lifecycle reference

## Packages

- **[`simconnect`](https://pkg.go.dev/github.com/mrlm-net/simconnect)** — Main entry point — `New()` for managed connection, `NewClient()` for direct engine access
- **[`pkg/engine`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/engine)** — High-level client, session lifecycle, message dispatching
- **[`pkg/manager`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/manager)** — Connection manager with auto-reconnect and state tracking
- **[`pkg/types`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/types)** — Typed data structures, enums, events
- **[`pkg/datasets`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/datasets)** — Pre-built dataset definitions (aircraft, environment, facilities, objects, simulator, traffic)
- **[`pkg/convert`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/convert)** — Unit conversions, ICAO validation, WGS84 coordinate offsets
- **[`pkg/calc`](https://pkg.go.dev/github.com/mrlm-net/simconnect/pkg/calc)** — Calculation helpers (haversine great-circle distance)

## Installation

```shell
go get github.com/mrlm-net/simconnect
```

## Requirements

| Requirement | Version |
|-------------|---------|
| Go | 1.25+ |
| Operating system | Windows |
| Microsoft Flight Simulator | 2020 / 2024 |
| SimConnect SDK | Bundled with MSFS |

## Contributing

_Contributions are welcomed and must follow the [Code of Conduct](https://github.com/mrlm-net/simconnect?tab=coc-ov-file) and common [Contribution guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md)._

> If you'd like to report a security issue please follow the [security guidelines](https://github.com/mrlm-net/simconnect?tab=security-ov-file).

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

---

<sup><sub>_All rights reserved © Martin Hrášek [<@mrlm-xyz>](https://github.com/mrlm-xyz) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)_</sub></sup>
