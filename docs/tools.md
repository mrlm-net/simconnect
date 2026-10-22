---
title: "Tools"
description: "The commands that ship with the SDK: the airport map, simvar-cli, traffic-actuator and traffic-director, and the example programs used as tools (offline airport data, jetway helpers, glitch and smoothness checks)."
section: "tools"
order: 1
---

# Tools

Every tool runs from a simconnect checkout. The airport map and `simvar-cli` are modules of their own under `cmd/`; run them with `go run -C cmd/<tool> .` from the repository root (create a `go.work` once so they build against the checkout: `go work init . ./cmd/airport-map ./cmd/simvar-cli`). The traffic split commands and the examples are part of the root module: `go run ./cmd/<tool>` and `go run ./examples/<name>`.

| Tool | What it does | Runs on |
|---|---|---|
| [airport-map](#airport-map) | The traffic control app built on the SDK and the tool it is debugged with | Windows, beside the simulator (or `-file` offline) |
| [simvar-cli](#simvar-cli) | Get, set and watch SimVars, emit and listen for events, an interactive REPL | Windows |
| [traffic-actuator](#traffic-actuator-and-traffic-director) | The simulator side of a split traffic World | Windows, beside the simulator |
| [traffic-director](#traffic-actuator-and-traffic-director) | The decisions of a split traffic World, the World's HTTP API and the map's page | Anywhere, Linux included |
| [scene-cli](#scene-cli) | Writes camera scenes against a running airport map | Windows, beside the map |

## airport-map

```bash
go run -C cmd/airport-map .               # connect and open LKPR
go run -C cmd/airport-map . -icao LOWW    # another airport
go run -C cmd/airport-map . -addr :8080   # on the network: tablets, phones, other controllers
```

Then open <http://127.0.0.1:8080/?icao=LKPR>. The map is its own module and is not tagged with every library release, so always run it from a checkout rather than with `go install`. Every flag (`-addr`, `-icao`, `-dump`, `-file`, `-log-dir`, `-airways`, `-airspace`, `-piper`, `-voices`, `-accents`, `-token`, `-view-token`, `-scenes`, `-split`, `-pprof`) is listed with its default in [Examples: Run it](examples.md#run-it), followed by a tour of the map.

## simvar-cli

```bash
go run -C cmd/simvar-cli . get "PLANE ALTITUDE" feet float64
go run -C cmd/simvar-cli . --format json watch "PLANE ALTITUDE" feet float64
go run -C cmd/simvar-cli . repl
```

The complete reference is [SimVar CLI](simvar-cli.md).

## traffic-actuator and traffic-director

The traffic World can run in two parts. The **actuator** runs beside the simulator: it keeps the SimConnect connection and moves every aircraft at frame rate. The **director** takes every decision (schedule, ATC, sequencing, separation, conflicts), needs no simulator and serves the World's HTTP API. Only decisions cross the network, about once a second.

```sh
# on the simulator's PC
go run ./cmd/traffic-actuator -listen :7710 -token s3cret
# anywhere: the API and the map's page on :8080
go run ./cmd/traffic-director -actuator simpc:7710 -token s3cret -addr :8080 -web cmd/airport-map/web
```

Behind a router the actuator dials the director instead (`-director host:7710`, `-tls`), and a director serving several players waits for them with `-listen`.

| traffic-actuator flag | Default | |
|---|---|---|
| `-listen` | `:7710` | address directors connect to |
| `-token` | | token a director must give (`""`: none, trusted networks only) |
| `-director` | | dial this director (`host:port`) instead of listening |
| `-tls`, `-tls-ca` | `false` | with `-director`: the link over TLS, verified with the system roots or this CA |
| `-data-dir` | `.` | local settings (custom pushes, de-icing pads) |

| traffic-director flag | Default | |
|---|---|---|
| `-actuator` | `127.0.0.1:7710` | the actuator to drive |
| `-token` | | the actuator's token |
| `-listen` | | wait on this address for actuators that dial in, instead of dialling `-actuator` |
| `-addr` | `127.0.0.1:8080` | HTTP address of the World's API |
| `-web` | | directory of the map's page to serve (`cmd/airport-map/web`); `""`: the API only |
| `-airways` | | airway graph for flight plans |
| `-log-dir`, `-data-dir` | `.` | the traffic log, local settings |
| `-api-token`, `-view-token` | | tokens to control or to read the HTTP API (`auto`: a random one) |
| `-jwks`, `-jwks-iss`, `-jwks-aud`, `-jwks-api` | | with `-listen`: verify each actuator's session token against a JWKS (URL or file) instead of `-token`, and what it gives on the API |
| `-tls-cert`, `-tls-key` | | with `-listen`: serve the link over TLS |

How the split works, multiplayer and the link protocol are on [Traffic World](traffic-world.md#split-a-director-anywhere-an-actuator-beside-the-simulator).

## scene-cli

Writes camera scenes against a running airport map: spawns the aircraft and cars to frame, captures the simulator's camera as a composition (with a screenshot), and plays shots, glides and scenes.

```bash
go run ./cmd/scene-cli spawn -tail DEMO009 -model FSLTL_WANTED_A320_Midnight
go run ./cmd/scene-cli capture hero -tail DEMO009
go run ./cmd/scene-cli shot -tail DEMO009 -glide 4
go run ./cmd/scene-cli scene 4-showcase
go run ./cmd/scene-cli off
```

`-map` is the map's address (default `http://127.0.0.1:8080`) and `-icao` the airport (default `LKPR`). The other commands are `car` (a car near an aircraft, at a position or at the camera), `vehicle-radio [on|off]`, `moves` and `glides`. Scenes and compositions are on [Camera](camera.md).

## Examples used as tools

| Example | What it does |
|---|---|
| [airport-dump](https://github.com/mrlm-net/simconnect/tree/main/examples/airport-dump) | Writes airports' offline data (layout, procedures, airways, ILS, the worldwide airport list) for the World without a simulator |
| [jetway-helpers](https://github.com/mrlm-net/simconnect/tree/main/examples/jetway-helpers) | Writes the jetway helper package into the Community folder ([Jetway Helpers](jetway.md)) |
| [glitch-watch](https://github.com/mrlm-net/simconnect/tree/main/examples/glitch-watch) | Reports frame jumps of every aircraft the map drives, or everything near the user aircraft with `-near` |
| [glitch-log](https://github.com/mrlm-net/simconnect/tree/main/examples/glitch-log) | Follows the traffic log and flags radio and traffic problems |
| [smoothness-report](https://github.com/mrlm-net/simconnect/tree/main/examples/smoothness-report) | Draws the [Smoothness](traffic-smoothness.md) charts from the benchmarks and their history |
| [spike-airways](https://github.com/mrlm-net/simconnect/tree/main/examples/spike-airways) | Crawls the airway network around a centre into the map's `-airways` file |
