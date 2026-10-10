# mrlm-net/simconnect

GoLang wrapper over SimConnect.dll SDK for building Microsoft Flight Simulator 2020/2024 add-ons. Lightweight, typed, performant library.

## Stack

- **Language:** Go 1.27+
- **Platform:** Windows (requires SimConnect.dll from MSFS)
- **Module:** `github.com/mrlm-net/simconnect`
- **Dependencies:** Standard library only (zero external deps)

## Plugin Directive

Use `devstack:mrlm` agents, skills, and commands for all development tasks. Primary skill: `mrlm:golang`.

## Project Structure

- `main.go` — package entry point (New, NewClient)
- `internal/dll` — raw DLL syscall bindings (detect, main)
- `internal/simconnect` — low-level SimConnect API wrapper: data, events, facilities, flight plans, AI objects, client data, input/flow events, CommBus, camera
- `pkg/engine` — high-level client: options, connection lifecycle, tiered-buffer dispatch loop, streams, datasets, slog logger
- `pkg/manager` — connection manager with auto-reconnect: SimState, connection/sim state subscriptions, object/filename/system events, ID allocation, the manager's Fleet, resubscribe on reconnect
- `pkg/types` — typed structs, enums, events, exceptions, HRESULTs, receivers
- `pkg/datasets` — ready-made dataset definitions by domain (aircraft, environment, facilities, objects, simulator, traffic)
- `pkg/convert` — unit conversions (altitude, distance, speed, pressure, temperature, weight, angles), lat/lon ↔ offsets, ICAO codes
- `pkg/calc` — geodesy: haversine, bearing, cross/along-track, displacement, great circle, Dubins paths, magnetic variation, wind components
- `pkg/dict` — embedded tables replaceable at runtime: telephony, types, wake, performance, airports (#768)
- `pkg/registry` — SimVar metadata: names, units, data types, writability
- `pkg/addons` — installed add-ons (no SimConnect): packages path, Community/streamed scan, aircraft → package, processes
- `pkg/avionics` — user aircraft radios: COM active/standby, swap, squawk
- `pkg/gsx` — GSX Pro state from its L:vars (services, passengers, cargo, doors, gate), settable names
- `pkg/lvars` — write L:vars on the user aircraft (our own signals, GSX settings)
- `pkg/systems` — user aircraft systems by profile: default SimVars, per-model JSON (Fenix L:vars), local overrides, take-off speeds
- `pkg/flight` — flight recording: Recorder (any object, every frame), Track (versioned JSON lines, At interpolation)
- `pkg/pilot` — a copilot as pilot flying (logic only): the autopilot through climb, cruise, descent and approach, requests to the player as PM
- `pkg/checklist` — normal checklists as data (family default, model sets, local override) and a runner checking items against systems.State; findings for flight.Assess
- `pkg/camera` — add-on camera (MSFS 2024): poses, shots, drone moves, Director
- `pkg/airport` — ground layout, taxi graph, routing, stands, SIDs/STARs/approaches, limits, Locate/Tracker; `testdata/` LKPR capture
- `pkg/nav` — fixes, airway crawl and routing, weather, runway in use, ATIS, flight plans and .pln, performance
- `pkg/traffic` — AI aircraft: Fleet, taxi/arrival controllers, injected motion, pushback and tugs, service vehicles, schedules, TrafficManager, separation, sequencing, holds, TCAS, radio and phraseology, profiles
- `pkg/traffic/world` — the airport map's traffic engine as a package (#710): New/Run/RunOn/Feed, Snapshot, Do/Get, real traffic, director/actuator link; `world/gen` — `remote.js`, the `go generate` script that writes `remote_gen.go`; `world/scenes` — built-in scripted camera scenes (cast, cues, shots) as embedded JSON: departure, arrival, mixed
- `examples/` — one standalone `main` per folder; `spike-*` are throwaway experiments
- `cmd/airport-map` — the airport map: front end of pkg/traffic/world, page, voice (own go.mod: voice-goio)
- `cmd/traffic-actuator` — the World's simulator side beside MSFS: listens, or dials a director
- `cmd/traffic-director` — the World's decisions without a simulator; serves the API and the map's page
- `cmd/simvar-cli` — interactive SimVar get/set CLI (own go.mod)
- `docs/` — `docs/*.md`, one page per area, site front matter; start at `docs/getting-started.md`; grep `docs/` before writing new pages
- `website/` — SvelteKit static docs site; reads `docs/*.md` (not subfolders); sidebar sections in `src/lib/config/navigation.ts`

## Build & Test

```bash
# Build (library — no binary output)
go build ./...

# Run tests
go test ./...

# Run example
go run ./examples/basic-connection

# Examples with own go.mod (have external dependencies)
cd cmd/simvar-cli && go run .
cd cmd/airport-map && go run .

# Vet (disable unsafeptr for DLL interop false positives)
go vet -unsafeptr=false ./...

# Lint (if golangci-lint installed)
golangci-lint run ./...
```

## Consumers

- `mycrew-online/app` and `simconnect-mcp` import this library (pkg/traffic/world, pkg/nav, pkg/airport, pkg/systems, pkg/gsx, ...). Check their callers before changing exported API.
- `simconnect-mcp` bundles `docs/*.md` at its go.mod version via `go generate`, so doc renames reach it on its next bump.

## Release

- Features = minor, fixes = patch; add a `CHANGELOG.md` entry.
- Squash-merge the PR, then tag the squash commit (verify it before tagging).
- Chain push / PR / merge / tag with `&&` only, so a failed step stops the chain.

## Conventions

- Functional options pattern for configuration (`ClientWith*`, `ManagerWith*`)
- `internal/` for private SimConnect bindings; `pkg/` for public API
- Each example is a standalone `main` package in `examples/<name>/`
- Datasets are organized by domain (aircraft, environment, facilities, etc.)
- Types use strong typing — enums, typed constants, dedicated structs
- Channel-based subscriptions for async message/state handling
- Zero external dependencies — standard library only

## Workload Management

- Work is tracked in GitHub Issues on `mrlm-net/simconnect` (use the `github-issues` skill).
- Each task gets its own issue, branch (`feat/<issue>-<short>` / `fix/<issue>-<short>`) and one PR referencing it (`Closes #42`).
- Post decisions, blockers and quality-gate failures on the issue; no progress chatter.
- Project board IDs and `gh project` commands: [docs/dev/github-board.md](docs/dev/github-board.md).
