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
│   ├── dll/                 # Raw DLL syscall bindings (detect, main)
│   └── simconnect/          # Low-level SimConnect API wrapper
│       ├── config.go        #   Client configuration
│       ├── connection.go    #   Open/Close lifecycle
│       ├── data.go          #   Data definitions & requests
│       ├── dispatch.go      #   Message dispatching
│       ├── event.go         #   Event mapping & transmission
│       ├── facility.go      #   Facility data requests
│       ├── flight.go        #   Flight plan operations
│       ├── notification.go  #   Notification groups
│       ├── object.go        #   Generic SimObject operations
│       ├── object-ai.go     #   AI aircraft creation & management
│       └── system.go        #   System events & state
├── pkg/
│   ├── engine/              # High-level client, lifecycle, dispatching
│   │   ├── client.go        #   Client struct & options
│   │   ├── config.go        #   Engine configuration
│   │   ├── connection.go    #   Connection lifecycle
│   │   ├── dispatcher.go    #   Tiered buffer pool dispatch loop
│   │   ├── stream.go        #   Streaming data helpers
│   │   ├── message.go       #   Message type handling
│   │   ├── data.go          #   Data definition registration
│   │   ├── datasets.go      #   Dataset registration helpers
│   │   ├── event.go         #   Event subscription & emit
│   │   ├── facility.go      #   Facility data requests
│   │   ├── facility_datasets.go # Facility dataset helpers
│   │   ├── flight.go        #   Flight plan operations
│   │   ├── notification.go  #   Notification groups
│   │   ├── object.go        #   SimObject operations
│   │   ├── system.go        #   System events & state
│   │   └── logger.go        #   Structured slog logger
│   ├── manager/             # Connection manager with auto-reconnect
│   │   ├── main.go          #   Manager constructor & helpers
│   │   ├── instance.go      #   Instance struct & handler entry types
│   │   ├── config.go        #   Manager options (ManagerWith*)
│   │   ├── lifecycle.go     #   Start/Stop/reconnect loop
│   │   ├── connection.go    #   Connection state management
│   │   ├── dispatch.go      #   Dispatch routing hub
│   │   ├── dispatch-events.go      # System event dispatch
│   │   ├── dispatch-filenames.go   # Filename event dispatch
│   │   ├── dispatch-objects.go     # Object event dispatch
│   │   ├── dispatch-simstate.go    # SimState data dispatch
│   │   ├── handlers-connection.go  # Connection/message/open/quit handlers
│   │   ├── handlers-filename-events.go # Flight/aircraft file handlers
│   │   ├── handlers-object-events.go  # Object add/remove handlers
│   │   ├── handlers-system-events.go  # Crash/sound/view handlers
│   │   ├── handlers-simstate.go    # SimState/pause/sim running handlers
│   │   ├── custom_events.go #   Custom system event support
│   │   ├── datasets.go      #   Dataset registration
│   │   ├── request.go       #   Data request helpers
│   │   ├── ids.go           #   Define/Request ID allocation
│   │   ├── state.go         #   SimState struct & types
│   │   ├── state-enums.go   #   CameraState/CameraSubstate enums
│   │   ├── state-helpers.go #   SimState comparison & diffing
│   │   ├── simstate_registration.go # SimState dataset setup
│   │   ├── state-subscriptions.go   # State/SimState change subscriptions
│   │   ├── connection-event-subscriptions.go # Open/Quit subscriptions
│   │   ├── object-subscriptions.go  # Object add/remove subscriptions
│   │   ├── filename-subscriptions.go # Flight/aircraft file subscriptions
│   │   ├── subscription-base.go     # Shared subscription plumbing
│   │   ├── notify.go        #   Notification helpers
│   │   ├── getters.go       #   Public state accessors
│   │   └── manager.go       #   Manager interface definition
│   ├── types/               # Typed data structures, enums, events
│   │   ├── data.go          #   Data definition types
│   │   ├── event.go         #   Event ID enums
│   │   ├── exception.go     #   Exception types
│   │   ├── facility.go      #   Facility structs
│   │   ├── facility_enums.go #  Facility enum constants
│   │   ├── group.go         #   Group priority types
│   │   ├── hresult.go       #   HRESULT error codes
│   │   ├── object.go        #   Object types
│   │   ├── period.go        #   Period enums
│   │   ├── receiver.go      #   Receiver interfaces
│   │   ├── simobject.go     #   SimObject type constants
│   │   ├── system.go        #   System event/state types
│   │   └── others.go        #   Miscellaneous types
│   ├── datasets/            # Ready-made dataset definitions
│   │   ├── data.go          #   Base data interface
│   │   ├── dataset.go       #   Dataset registration helpers
│   │   ├── facility-data.go #   Facility data interface
│   │   ├── facility-dataset.go # Facility dataset helpers
│   │   ├── aircraft/        #   Aircraft position, attitude, engine
│   │   ├── environment/     #   Weather, time, ambient conditions
│   │   ├── facilities/      #   Airports, runways, taxiways, VORs, NDBs, ...
│   │   ├── objects/         #   SimObject data definitions
│   │   ├── simulator/       #   Simulator state variables
│   │   └── traffic/         #   AI traffic data definitions
│   ├── convert/             # Type conversion utilities
│   │   ├── altitude.go      #   Feet/meters conversion
│   │   ├── distance.go      #   NM/km/mi conversion
│   │   ├── icao.go          #   ICAO code validation & lookup
│   │   ├── icao-data.go     #   ICAO prefix region/country map
│   │   ├── position.go      #   Lat/lon conversion
│   │   └── speed.go         #   Knots/km/h/m/s conversion
│   └── calc/                # Calculation helpers
├── examples/                # Example applications (one per folder)
│   ├── basic-connection/    #   Minimal connect & disconnect
│   ├── lifecycle-connection/ #  Connection with lifecycle hooks
│   ├── await-connection/    #   Blocking connection wait
│   ├── read-messages/       #   Raw message reading
│   ├── read-objects/        #   SimObject data reading
│   ├── set-variables/       #   SimVar writing
│   ├── emit-events/         #   Event emission
│   ├── subscribe-events/    #   Event subscriptions
│   ├── read-facility/       #   Single facility lookup
│   ├── read-facilities/     #   Bulk facility reading
│   ├── subscribe-facilities/ #  Facility change subscriptions
│   ├── all-facilities/      #   Enumerate all facility types
│   ├── airport-details/     #   Airport detail inspection
│   ├── locate-airport/      #   Airport search by location
│   ├── read-waypoints/      #   Waypoint data reading
│   ├── using-datasets/      #   Pre-built dataset usage
│   ├── ai-traffic/          #   AI traffic injection
│   ├── manage-traffic/      #   AI traffic management
│   ├── monitor-traffic/     #   AI traffic monitoring
│   ├── simconnect-manager/  #   Manager with auto-reconnect
│   ├── simconnect-subscribe/ #  Manager subscriptions
│   ├── simconnect-state/    #   SimState tracking
│   ├── simconnect-events/   #   Manager event handling
│   ├── simconnect-facilities/ # Manager facility queries
│   ├── simconnect-traffic/  #   Manager traffic operations
│   └── simconnect-benchmark/ #  Performance benchmarking
└── docs/                    # Documentation
    ├── config-client.md     #   Client configuration reference
    ├── config-manager.md    #   Manager configuration reference
    ├── usage-client.md      #   Client usage guide
    ├── usage-manager.md     #   Manager usage guide
    └── manager-requests-ids.md # ID allocation reference
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

## Workload Management

Agents track work decisions, blockers, and outcomes in GitHub Issues.

**System**: GitHub Issues
**Repository**: `mrlm-net/simconnect`
**Configuration**:
- Use the `github-issues` skill for issue management
- Each task/feature gets its own GitHub Issue and a dedicated branch + PR
- Branch naming: `feat/<issue-number>-<short-description>`, `fix/<issue-number>-<short-description>`
- One PR per task — PRs reference the issue they resolve (e.g., `Closes #42`)
- Agents post decisions (e.g., "Chose X over Y because Z"), blockers, quality gate failures, and phase outcomes
- Agents do NOT post progress notifications or status updates — keep it human-consumable

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
| `/release` | Publish versioned release with changelog, git tag, and GitHub Release |
| `/scope` | Plan from GitHub issue or topic — analysis, design, planning, and backlog creation |
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
