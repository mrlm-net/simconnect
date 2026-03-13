# SimVar CLI Example

## Overview

Interactive command-line tool for reading and writing MSFS Simulation Variables (SimVars) directly from the terminal. Unlike other examples in this repository, `simvar-cli` has its own `go.mod` and uses the [CURE](https://github.com/mrlm-net/cure) CLI framework for command routing alongside the SimConnect engine client for simulator communication.

## What It Does

1. **Read SimVars** - Fetches a single variable value from the simulator and prints it to stdout
2. **Write SimVars** - Sets a variable value on the user aircraft
3. **Emit Events** - Fires client events (e.g., AP_MASTER, GEAR_TOGGLE) with optional data parameters
4. **Listen for Events** - Monitors client events in real time with timestamps until Ctrl+C
5. **Interactive REPL** - Opens a persistent session for issuing multiple get/set/emit/listen commands without reconnecting
6. **Watch (stream)** - Streams a SimVar continuously at a chosen interval until Ctrl+C
7. **Auto-reconnection** - Retries connection to the simulator with 2-second intervals until successful
8. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed (SimConnect.dll accessible)
- Go 1.25+

## Building

This example has its own `go.mod` with an external dependency on CURE. It cannot be built from the repository root with `go run ./examples/simvar-cli`.

```bash
cd examples/simvar-cli
go run .
```

To build a standalone binary:

```bash
cd examples/simvar-cli
go build -o simvar-cli.exe .
```

## Usage

### Global Flags

All commands accept these flags before the subcommand:

| Flag | Default | Description |
|------|---------|-------------|
| `--dll-path` | `""` | Explicit path to SimConnect.dll |
| `--auto-detect` | `false` | Auto-detect SimConnect.dll location |
| `--log-level` | `warn` | Log level (`debug`, `info`, `warn`, `error`) |
| `--timeout` | `10` | Timeout in seconds for operations |
| `--format` | `table` | Output format: `table`, `json`, or `csv` |
| `--config` | `""` | Path to a JSON config file |

### Get a SimVar

Read a single variable value and print it to stdout.

```bash
# Syntax
simvar-cli get <variable-name> <unit> <datatype>

# Read current altitude in feet
simvar-cli get "PLANE ALTITUDE" feet float64

# Read airspeed in knots
simvar-cli get "AIRSPEED INDICATED" knots float64

# Read current camera state (unitless integer)
simvar-cli get "CAMERA STATE" "" int32

# With explicit DLL path and debug logging
simvar-cli --dll-path "C:\MSFS SDK\SimConnect SDK\lib\SimConnect.dll" --log-level debug get "PLANE ALTITUDE" feet float64
```

**Output:** The raw value is printed to stdout (e.g., `35024.5`). Connection status and errors go to stderr, so the value is safe to capture in scripts.

### Set a SimVar

Write a value to a simulator variable on the user aircraft.

```bash
# Syntax
simvar-cli set <variable-name> <unit> <datatype> <value>

# Switch to external camera view
simvar-cli set "CAMERA STATE" "" int32 3

# Set heading bug
simvar-cli set "AUTOPILOT HEADING LOCK DIR" degrees float64 270.0
```

**Output:** Prints `OK` to stdout on success, or a SimConnect exception on failure.

### Emit an Event

Fire a client event on the user aircraft. Supports 0-1 data values via `TransmitClientEvent` and 2-5 data values via `TransmitClientEventEx1`.

```bash
# Syntax
simvar-cli emit <event-name> [data...]

# Toggle autopilot master (data defaults to 0)
simvar-cli emit AP_MASTER

# Toggle a specific aircraft exit
simvar-cli emit TOGGLE_AIRCRAFT_EXIT 3

# Signed integer values (int32 range, cast to uint32)
simvar-cli emit AXIS_ELEVATOR_SET -8000

# Multiple data values (uses TransmitClientEventEx1)
simvar-cli emit SOME_EVENT 1 2 3 4 5
simvar-cli emit SOME_EVENT 100 200
```

**Output:** Prints `OK` to stdout on success, or a SimConnect exception on failure.

### Listen for Events

Monitor client events in real time. Prints received events with timestamps until interrupted with Ctrl+C.

```bash
# Syntax
simvar-cli listen <event-name> [event-name...]

# Listen for a single event
simvar-cli listen GEAR_TOGGLE

# Listen for multiple events
simvar-cli listen AP_MASTER GEAR_TOGGLE
```

**Output:** Each received event is printed as `[RFC3339 timestamp] EVENT_NAME data=VALUE`. Connection status goes to stderr.

### Watch (stream) a SimVar

Stream a SimVar value continuously until interrupted with Ctrl+C. Useful for monitoring variables in real time or piping output into other tools.

```bash
# Syntax
simvar-cli watch [--interval second|visual-frame|sim-frame] [--changed] <variable-name> <unit> <datatype>

# Stream altitude once per second (default interval)
simvar-cli watch "PLANE ALTITUDE" feet float64

# Stream at visual frame rate
simvar-cli watch --interval visual-frame "PLANE LATITUDE" degrees float64

# Only print when value changes (suppresses duplicate readings)
simvar-cli watch --interval sim-frame --changed "AUTOPILOT MASTER" "" int32

# Stream altitude as NDJSON
simvar-cli --format json watch "PLANE ALTITUDE" feet float64

# Stream to CSV file
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64 > altitude.csv
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `second` | Polling period: `second`, `visual-frame`, or `sim-frame` |
| `--changed` | `false` | Only output a reading when the value differs from the previous one |

**Interval mapping:**

| Value | SimConnect period | Notes |
|-------|-------------------|-------|
| `second` | `SIMCONNECT_PERIOD_SECOND` | One reading per simulated second |
| `visual-frame` | `SIMCONNECT_PERIOD_VISUAL_FRAME` | One reading per rendered frame |
| `sim-frame` | `SIMCONNECT_PERIOD_SIM_FRAME` | One reading per simulation tick |

Press Ctrl+C to stop streaming. The connection is closed cleanly on exit.

### REPL Mode

Start an interactive session. This is the default when no subcommand is given.

```bash
# Explicit
simvar-cli repl

# Implicit (no subcommand defaults to REPL)
simvar-cli
```

Inside the REPL:

```
simvar> get "PLANE ALTITUDE" feet float64
35024.5
simvar> set "CAMERA STATE" "" int32 3
OK
simvar> emit AP_MASTER
OK
simvar> emit AXIS_ELEVATOR_SET -8000
OK
simvar> listen GEAR_TOGGLE AP_MASTER
Listening for GEAR_TOGGLE
Listening for AP_MASTER
simvar> listeners
Active listeners: AP_MASTER, GEAR_TOGGLE
simvar> unlisten GEAR_TOGGLE
Stopped listening for GEAR_TOGGLE
simvar> help
Commands:
  get <variable-name> <unit> <datatype>        Read a SimVar value
  set <variable-name> <unit> <datatype> <value> Write a SimVar value
  emit <event-name> [data...]                  Fire a client event
  listen <event-name> [event-name...]           Subscribe to events
  unlisten <event-name>                         Unsubscribe from event
  listeners                                     Show active listeners
  help                                          Show this help
  exit | quit                                   End the session
simvar> exit
```

REPL commands:

| Command | Description |
|---------|-------------|
| `get <var> <unit> <datatype>` | Read a SimVar value |
| `set <var> <unit> <datatype> <value>` | Write a SimVar value |
| `emit <event-name> [data...]` | Fire a client event |
| `listen <event-name> [event-name...]` | Subscribe to events |
| `unlisten <event-name>` | Unsubscribe from an event |
| `listeners` | Show active event listeners |
| `help` | Show available commands |
| `exit` / `quit` | End the session |

The REPL maintains a single persistent connection and handles responses asynchronously, so it is significantly faster for multiple operations than running individual `get`/`set`/`emit` commands. Event listeners print incoming events to stderr to keep the prompt clean.

## Output Formats

The `--format` global flag controls how readings are rendered. It applies to the `get` and `watch` commands.

| Format | Description |
|--------|-------------|
| `table` | Human-readable, four labelled lines per reading (default) |
| `json` | One JSON object per reading, newline-delimited (NDJSON) |
| `csv` | RFC 4180 CSV: one header row followed by one data row per reading |

### table (default)

```
Name:     PLANE ALTITUDE
Value:    35024.5 feet
DataType: float64
Time:     2025-11-01T12:34:56.789Z
```

### json

Each reading is a complete JSON object followed by a newline. Multiple readings produce valid NDJSON that can be processed with `jq`.

```json
{"name":"PLANE ALTITUDE","value":"35024.5","unit":"feet","datatype":"float64","timestamp":"2025-11-01T12:34:56.789123456Z"}
```

```bash
simvar-cli --format json watch "PLANE ALTITUDE" feet float64 | jq '.value'
```

### csv

The first line is always a header row written by `FormatCSVHeader`. Each subsequent reading is a data row. Field order: `name,value,unit,datatype,timestamp`.

```
name,value,unit,datatype,timestamp
PLANE ALTITUDE,35024.5,feet,float64,2025-11-01T12:34:56.789123456Z
PLANE ALTITUDE,35100.2,feet,float64,2025-11-01T12:34:57.001234567Z
```

```bash
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64 > altitude.csv
```

## Config File

`simvar-cli` supports a JSON configuration file for setting defaults without repeating flags on every invocation.

### Resolution Order

The config file is resolved in this order. The first file found is used; later candidates are not checked.

1. `--config <path>` flag — explicit path; file must exist or the tool exits with an error
2. `SIMVAR_CLI_CONFIG` environment variable — explicit path; file must exist or the tool exits with an error
3. `%APPDATA%\simvar-cli\config.json` — user-level default; silently ignored if absent
4. `.\simvar-cli.json` in the current working directory — project-level default; silently ignored if absent

If no config file is found, the tool uses its built-in defaults.

### Config Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `dll_path` | string | Explicit path to SimConnect.dll | `""` |
| `auto_detect` | bool | Auto-detect SimConnect.dll location | `false` |
| `timeout` | int | Timeout in seconds for operations | `10` |
| `log_level` | string | Log level: `debug`, `info`, `warn`, `error` | `"warn"` |
| `format` | string | Output format: `table`, `json`, `csv` | `"table"` |

### Precedence

CLI flags always win over config file values, which win over built-in defaults. The tool uses `flag.Visit` to detect which flags were explicitly provided; unset flags fall back to the config, then to defaults.

### Example Config File

```json
{
  "dll_path": "C:\\MSFS SDK\\SimConnect SDK\\lib\\SimConnect.dll",
  "auto_detect": false,
  "timeout": 30,
  "log_level": "warn",
  "format": "table"
}
```

Save as `%APPDATA%\simvar-cli\config.json` for user-level defaults, or as `simvar-cli.json` in the directory you run the tool from for project-level defaults.

## Supported Data Types

| Type | Size | Go Type | Example Values |
|------|------|---------|----------------|
| `int32` | 4 bytes | `int32` | `0`, `3`, `-1` |
| `int64` | 8 bytes | `int64` | `0`, `100000` |
| `float32` | 4 bytes | `float32` | `3.14`, `0.0` |
| `float64` | 8 bytes | `float64` | `35024.5`, `245.3` |

Choose the data type based on the SimVar documentation. Most positional and rate variables use `float64`. State and enum variables typically use `int32`.

## Variable Name Quoting

Variable names containing spaces must be quoted. The CLI and REPL both support double-quoted strings:

```bash
# CLI: shell handles the quotes
simvar-cli get "PLANE ALTITUDE" feet float64

# REPL: the tokenizer handles the quotes
simvar> get "PLANE ALTITUDE" feet float64
```

For unitless variables (e.g., `CAMERA STATE`), pass an empty string `""` as the unit.

## Files

| File | Purpose |
|------|---------|
| `main.go` | Entrypoint, global flag parsing, config loading, CURE router setup |
| `bridge.go` | Shared utilities: ID counters, type parsing, value formatting, event mapping, output formatting |
| `config.go` | JSON config file loading with 4-step resolution |
| `get.go` | `get` command implementation |
| `set.go` | `set` command implementation |
| `emit.go` | `emit` command implementation (TransmitClientEvent / TransmitClientEventEx1) |
| `listen.go` | `listen` command implementation (notification group based event monitoring) |
| `repl.go` | `repl` command with interactive input loop, async response handling, and event commands |
| `watch.go` | `watch` command — continuous streaming with interval and changed-only filtering |
| `go.mod` | Standalone module with CURE dependency and parent module replace directive |

## SimVar Reference

For the full list of available simulation variables, units, and data types, see the official Microsoft Flight Simulator SDK documentation:

- [MSFS 2024 Simulation Variables](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimVars/Simulation_Variables.htm)

## Notes

- The tool connects as client name `SimVar CLI - Get`, `SimVar CLI - Set`, `SimVar CLI - Emit`, `SimVar CLI - Listen`, `SimVar CLI - REPL`, or `SimVar CLI - Watch` depending on the command
- Each `get`, `set`, `emit`, `listen`, and `watch` invocation creates a fresh connection; use REPL mode for repeated operations
- Event mappings (MapClientEventToSimEvent) are cached and reused across multiple emit/listen calls within a session
- The `go.mod` uses a `replace` directive pointing to the parent module (`../..`) for local development
- `unsafe.Pointer` is used internally for `SetDataOnSimObject` calls, matching the pattern in the `set-variables` example
