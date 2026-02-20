# SimVar CLI Example

## Overview

Interactive command-line tool for reading and writing MSFS Simulation Variables (SimVars) directly from the terminal. Unlike other examples in this repository, `simvar-cli` has its own `go.mod` and uses the [CURE](https://github.com/mrlm-net/cure) CLI framework for command routing alongside the SimConnect engine client for simulator communication.

## What It Does

1. **Read SimVars** - Fetches a single variable value from the simulator and prints it to stdout
2. **Write SimVars** - Sets a variable value on the user aircraft
3. **Interactive REPL** - Opens a persistent session for issuing multiple get/set commands without reconnecting
4. **Auto-reconnection** - Retries connection to the simulator with 2-second intervals until successful
5. **Graceful shutdown** - Responds to Ctrl+C interrupt signals cleanly

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
simvar> get "AIRSPEED INDICATED" knots float64
245.3
simvar> set "CAMERA STATE" "" int32 3
OK
simvar> get "CAMERA STATE" "" int32
3
simvar> exit
```

REPL commands:

| Command | Description |
|---------|-------------|
| `get <var> <unit> <datatype>` | Read a SimVar value |
| `set <var> <unit> <datatype> <value>` | Write a SimVar value |
| `exit` / `quit` | End the session |

The REPL maintains a single persistent connection and handles responses asynchronously, so it is significantly faster for multiple operations than running individual `get`/`set` commands.

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
| `main.go` | Entrypoint, global flag parsing, CURE router setup |
| `bridge.go` | Shared utilities: ID counters, type parsing, value formatting |
| `get.go` | `get` command implementation |
| `set.go` | `set` command implementation |
| `repl.go` | `repl` command with interactive input loop and async response handling |
| `go.mod` | Standalone module with CURE dependency and parent module replace directive |

## SimVar Reference

For the full list of available simulation variables, units, and data types, see the official Microsoft Flight Simulator SDK documentation:

- [MSFS 2024 Simulation Variables](https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimVars/Simulation_Variables.htm)

## Notes

- The tool connects as client name `SimVar CLI - Get`, `SimVar CLI - Set`, or `SimVar CLI - REPL` depending on the command
- Each `get` and `set` invocation creates a fresh connection; use REPL mode for repeated operations
- The `go.mod` uses a `replace` directive pointing to the parent module (`../..`) for local development
- `unsafe.Pointer` is used internally for `SetDataOnSimObject` calls, matching the pattern in the `set-variables` example
