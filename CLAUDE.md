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

# Vet (disable unsafeptr for DLL interop false positives)
go vet -unsafeptr=false ./...

# Lint (if golangci-lint installed)
golangci-lint run ./...
```

## Conventions

- Functional options pattern for configuration (`ClientWith*`, `ManagerWith*`)
- `internal/` for private SimConnect bindings; `pkg/` for public API
- Each example is a standalone `main` package in `examples/<name>/`
- Datasets are organized by domain (aircraft, environment, facilities, etc.)
- Types use strong typing — enums, typed constants, dedicated structs
- Channel-based subscriptions for async message/state handling
- Zero external dependencies — standard library only

## MRLM Plugin Usage

This project uses the [mrlm devstack plugin](https://github.com/mrlm-net/devstack) for AI-assisted development. Available commands:

| Command | What it does |
|---------|-------------|
| `/spec` | Gather requirements, write user stories and acceptance criteria |
| `/design` | Design system architecture, define interfaces and technical patterns |
| `/build` | Implement code and unit tests (engineer only, no review) |
| `/review` | Systematic code review for correctness, style, and performance |
| `/test` | Run E2E, performance, UX, and accessibility testing |
| `/secure` | Vulnerability scan, SBOM generation, OWASP compliance check |
| `/deploy` | Infrastructure provisioning and deployment automation |
| `/make` | Full SDLC pipeline — from requirements through security scan |
| `/ask` | Ask any question using full agent toolkit (read-only) |
| `/write` | Generate articles, documentation, or marketing content |
| `/generate` | Scaffold new agents, skills, or commands |
| `/init` | Initialize project structure and CLAUDE.md |

### Recommended Workflow

For new features, use the full pipeline: `/make [feature description]`

For focused work, chain individual commands:
1. `/spec` — define what to build
2. `/design` — plan how to build it
3. `/build` — implement it
4. `/review` — review the code
5. `/test` — verify it works
6. `/secure` — check for vulnerabilities
