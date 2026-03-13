---
title: "SimVar CLI"
description: "Interactive CLI tool for reading, writing, and streaming MSFS SimVars from the command line"
section: "packages"
order: 1
---

# SimVar CLI

`simvar-cli` is a standalone Windows command-line tool for reading, writing, and streaming MSFS Simulation Variables (SimVars) directly from a terminal. It connects to a running simulator via SimConnect, executes the requested operation, and exits — or streams continuously until you press Ctrl+C.

The tool lives in `cmd/simvar-cli/` and has its own `go.mod`. It cannot be built from the repository root.

## Commands

| Command | Description |
|---------|-------------|
| `get` | Read one SimVar value and print it |
| `set` | Write a value to a SimVar |
| `emit` | Fire a client event with optional data |
| `listen` | Monitor client events in real time |
| `repl` | Interactive session (default when no command is given) |
| `watch` | Stream a SimVar continuously at a chosen interval |

## Installation / Build

```bash
cd cmd/simvar-cli
go build -o simvar-cli.exe .
```

**Prerequisites:**

- Windows OS — SimConnect is Windows-only
- Microsoft Flight Simulator 2020 or 2024 running
- SimConnect SDK installed (SimConnect.dll accessible)
- Go 1.25+

The `go.mod` includes a `replace` directive pointing to the parent module (`../..`) for local development.

## Global Flags

These flags are accepted before the subcommand name and apply to all commands.

| Flag | Default | Description |
|------|---------|-------------|
| `--dll-path <path>` | `""` | Explicit path to SimConnect.dll |
| `--auto-detect` | `false` | Auto-detect SimConnect.dll location |
| `--log-level <level>` | `warn` | Log level: `debug`, `info`, `warn`, `error` |
| `--timeout <seconds>` | `10` | Operation timeout in seconds |
| `--format <format>` | `table` | Output format: `table`, `json`, or `csv` |
| `--config <path>` | `""` | Path to a JSON config file |

Flag precedence (highest to lowest): CLI flag > config file value > built-in default. The tool uses `flag.Visit` to detect which flags were explicitly set on the command line, so config values are only applied for flags you did not provide.

## Commands

### get

Read one SimVar value and print it to stdout.

```
simvar-cli [global flags] get <variable-name> <unit> <datatype>
```

```bash
simvar-cli get "PLANE ALTITUDE" feet float64
simvar-cli get "AIRSPEED INDICATED" knots float64
simvar-cli get "CAMERA STATE" "" int32
```

The raw value is printed to stdout. Connection status and errors go to stderr, so the value is safe to pipe or redirect.

### set

Write a value to a SimVar on the user aircraft.

```
simvar-cli [global flags] set <variable-name> <unit> <datatype> <value>
```

```bash
simvar-cli set "AUTOPILOT HEADING LOCK DIR" degrees float64 270.0
simvar-cli set "CAMERA STATE" "" int32 3
```

Prints `OK` on success or a SimConnect exception description on failure.

### emit

Fire a client event. Accepts 0–5 integer data values. Uses `TransmitClientEvent` for 0–1 values and `TransmitClientEventEx1` for 2–5 values. Data values are parsed as int32 and cast to uint32, so negative values are accepted.

```
simvar-cli [global flags] emit <event-name> [data...]
```

```bash
simvar-cli emit AP_MASTER
simvar-cli emit TOGGLE_AIRCRAFT_EXIT 3
simvar-cli emit AXIS_ELEVATOR_SET -8000
simvar-cli emit SOME_EVENT 1 2 3 4 5
```

### listen

Monitor client events in real time. Prints each received event as `[RFC3339 timestamp] EVENT_NAME data=VALUE` until interrupted with Ctrl+C.

```
simvar-cli [global flags] listen <event-name> [event-name...]
```

```bash
simvar-cli listen GEAR_TOGGLE
simvar-cli listen AP_MASTER GEAR_TOGGLE
```

### repl

Start an interactive session with a persistent simulator connection. This is the default when no subcommand is given. The REPL is significantly faster for multiple operations because it avoids the connection overhead of individual commands.

```
simvar-cli [global flags] repl
simvar-cli                       # implicit REPL
```

Inside the REPL, the following commands are available:

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

### watch

Stream a SimVar continuously, printing one reading per update until interrupted with Ctrl+C. The `--format` global flag controls the output format — useful for piping into `jq` or redirecting to a CSV file.

```
simvar-cli [global flags] watch [--interval second|visual-frame|sim-frame] [--changed] <variable-name> <unit> <datatype>
```

See [watch command](#watch-command) for full details.

## watch command

The `watch` command subscribes to a SimVar using `RequestDataOnSimObject` and streams readings until SIGINT. It accepts two command-specific flags in addition to all global flags.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `second` | Polling period: `second`, `visual-frame`, or `sim-frame` |
| `--changed` | `false` | Only output a reading when the value differs from the previous one |

### Interval Values

| Value | SimConnect constant | When a reading is sent |
|-------|---------------------|------------------------|
| `second` | `SIMCONNECT_PERIOD_SECOND` | Once per simulated second |
| `visual-frame` | `SIMCONNECT_PERIOD_VISUAL_FRAME` | Once per rendered frame |
| `sim-frame` | `SIMCONNECT_PERIOD_SIM_FRAME` | Once per simulation tick |

`sim-frame` produces the highest frequency. Use `--changed` with `sim-frame` or `visual-frame` to suppress noise when the variable is stable.

### --changed behaviour

When `--changed` is set, the tool compares each incoming value string against the previous one and skips the write step if they are equal. The comparison is string-level (post-formatting), so it reflects the value as it would appear in output.

### Examples

```bash
# Stream altitude once per second in the default table format
simvar-cli watch "PLANE ALTITUDE" feet float64

# Stream latitude at visual frame rate
simvar-cli watch --interval visual-frame "PLANE LATITUDE" degrees float64

# Stream autopilot state only when it changes
simvar-cli watch --interval sim-frame --changed "AUTOPILOT MASTER" "" int32

# Pipe altitude readings as NDJSON into jq
simvar-cli --format json watch "PLANE ALTITUDE" feet float64 | jq -r '.value'

# Record altitude to CSV
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64 > altitude.csv
```

Press Ctrl+C to stop. The data definition and simulator connection are cleaned up before exit.

## Output Formats

The `--format` global flag applies to the `get` and `watch` commands. Each reading is one `FormattedValue` struct rendered according to the chosen format.

### table (default)

Four labelled lines per reading. The label column is 9 characters wide for alignment.

```
Name:     PLANE ALTITUDE
Value:    35024.5 feet
DataType: float64
Time:     2025-11-01T12:34:56.789123456Z
```

### json

One JSON object per reading followed by a newline — valid NDJSON. Each object carries the fields `name`, `value`, `unit`, `datatype`, and `timestamp` (RFC 3339 with nanoseconds).

```json
{"name":"PLANE ALTITUDE","value":"35024.5","unit":"feet","datatype":"float64","timestamp":"2025-11-01T12:34:56.789123456Z"}
```

Multiple readings produce one object per line, suitable for processing with `jq`:

```bash
simvar-cli --format json watch "PLANE ALTITUDE" feet float64 | jq '.value'
```

### csv

RFC 4180 CSV output. The header row is written once at stream start by `FormatCSVHeader`. Each subsequent reading is one data row. Field order matches the JSON keys: `name`, `value`, `unit`, `datatype`, `timestamp`.

```
name,value,unit,datatype,timestamp
PLANE ALTITUDE,35024.5,feet,float64,2025-11-01T12:34:56.789123456Z
PLANE ALTITUDE,35100.2,feet,float64,2025-11-01T12:34:57.001234567Z
```

Redirect to a file for post-processing:

```bash
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64 > altitude.csv
```

## Config File

`simvar-cli` supports a JSON configuration file for setting persistent defaults without repeating flags on every invocation.

### Resolution Order

The tool searches for a config file in this order. The first file found is decoded; later candidates are not checked.

1. `--config <path>` — explicit path provided on the command line; the file must exist or the tool exits with an error
2. `SIMVAR_CLI_CONFIG` environment variable — explicit path; the file must exist or the tool exits with an error
3. `%APPDATA%\simvar-cli\config.json` — user-level default location; silently ignored if absent
4. `.\simvar-cli.json` in the current working directory — project-level default; silently ignored if absent

If no file is found at any candidate, the tool starts with built-in defaults and no error is reported.

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dll_path` | string | `""` | Explicit path to SimConnect.dll |
| `auto_detect` | bool | `false` | Auto-detect SimConnect.dll location |
| `timeout` | int | `10` | Operation timeout in seconds |
| `log_level` | string | `"warn"` | Log level: `debug`, `info`, `warn`, `error` |
| `format` | string | `"table"` | Output format: `table`, `json`, `csv` |

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

Save as `%APPDATA%\simvar-cli\config.json` for a user-level default that applies everywhere, or as `simvar-cli.json` in a project directory for per-project defaults.

> **Note:** CLI flags always override config file values. A flag you explicitly pass on the command line will never be overwritten by the config.

## Examples

### Read altitude and pipe to another tool

```bash
simvar-cli get "PLANE ALTITUDE" feet float64
# 35024.5
```

### Set autopilot heading

```bash
simvar-cli set "AUTOPILOT HEADING LOCK DIR" degrees float64 090.0
# OK
```

### Toggle gear and verify

```bash
simvar-cli emit GEAR_TOGGLE
simvar-cli get "GEAR TOTAL PCT EXTENDED" "" float64
```

### Stream altitude as NDJSON and extract values with jq

```bash
simvar-cli --format json watch "PLANE ALTITUDE" feet float64 | jq -r '.value'
```

### Record a flight data trace to CSV

```bash
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64 > trace.csv
```

### Use a project config file

```bash
# Create simvar-cli.json in the working directory
echo '{"dll_path":"C:\\SDK\\SimConnect.dll","format":"json"}' > simvar-cli.json

# All subsequent runs use these defaults
simvar-cli watch "PLANE ALTITUDE" feet float64
```

### Override config with CLI flags

```bash
# Config sets format = "json", but this run uses CSV instead
simvar-cli --format csv watch "PLANE ALTITUDE" feet float64
```

## Supported Data Types

| Type | Size | Go Type | Example Values |
|------|------|---------|----------------|
| `int32` | 4 bytes | `int32` | `0`, `3`, `-1` |
| `int64` | 8 bytes | `int64` | `0`, `100000` |
| `float32` | 4 bytes | `float32` | `3.14`, `0.0` |
| `float64` | 8 bytes | `float64` | `35024.5`, `245.3` |

Most positional and rate SimVars use `float64`. State and enum SimVars typically use `int32`.

## See Also

- [MSFS 2024 Simulation Variables reference](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimVars/Simulation_Variables.htm)
- [Engine/Client Usage](usage-client.md)
- [Manager Usage](usage-manager.md)
