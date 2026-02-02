# mrlm-net/simconnect

GoLang wrapper over SimConnect.dll SDK for building Microsoft Flight Simulator 2020/2024 add-ons. Lightweight, typed, performant library.

## Stack

- **Language:** Go 1.25+
- **Platform:** Windows (requires SimConnect.dll from MSFS)
- **Module:** `github.com/mrlm-net/simconnect`
- **Dependencies:** Standard library only (zero external deps)

## Plugin Directive

Use `devstack:mrlm` agents, skills, and commands for all development tasks. Primary skill: `mrlm:golang`.

## Project Structure

```
├── main.go                  # Package entry point (New, NewClient)
├── go.mod
├── internal/
│   ├── dll/                 # Raw DLL syscall bindings
│   └── simconnect/          # Low-level SimConnect API wrapper
├── pkg/
│   ├── engine/              # High-level client, lifecycle, dispatching
│   ├── manager/             # Connection manager with auto-reconnect
│   ├── types/               # Typed data structures, enums, events
│   ├── datasets/            # Ready-made dataset definitions
│   │   ├── aircraft/
│   │   ├── environment/
│   │   ├── facilities/
│   │   ├── objects/
│   │   ├── simulator/
│   │   └── traffic/
│   ├── convert/             # Type conversion utilities
│   └── calc/                # Calculation helpers
├── examples/                # Example applications (one per folder)
└── docs/                    # Documentation
```

## Build & Test

```bash
# Build (library — no binary output)
go build ./...

# Run tests
go test ./...

# Run example
go run ./examples/basic-connection

# Vet & lint
go vet ./...
```

## Conventions

- Functional options pattern for configuration (`ClientWith*`, `ManagerWith*`)
- `internal/` for private SimConnect bindings; `pkg/` for public API
- Each example is a standalone `main` package in `examples/<name>/`
- Datasets are organized by domain (aircraft, environment, facilities, etc.)
- Types use strong typing — enums, typed constants, dedicated structs
- Channel-based subscriptions for async message/state handling
- Zero external dependencies — standard library only

## Development Workflow

- `/mrlm:plan` — analyse requirements and plan work
- `/mrlm:make` — implement via SDLC delegation
- `/mrlm:review` — review and test changes
