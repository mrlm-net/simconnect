# SimVar CLI

Read, write and stream MSFS SimVars, emit and listen for client events, or work in an interactive REPL, from a terminal. Windows only; needs MSFS 2020/2024 running.

## Quick start

```bash
cd cmd/simvar-cli              # own go.mod: build here, not from the repository root
go build -o simvar-cli.exe .

simvar-cli get "PLANE ALTITUDE" feet float64
simvar-cli set "AUTOPILOT HEADING LOCK DIR" degrees float64 270.0
simvar-cli emit AP_MASTER
simvar-cli watch --interval second "PLANE ALTITUDE" feet float64
simvar-cli list                # registry metadata, no simulator needed
simvar-cli                     # interactive REPL
```

Release zips (`simvar-cli-vX.Y.Z-windows-amd64.zip`) on the [releases page](https://github.com/mrlm-net/simconnect/releases/latest) carry a pre-built binary.

## Full reference

Commands, global flags, output formats, config file and examples: [SimVar CLI documentation](https://simconnect.mrlm.net/docs/simvar-cli) (source: [docs/simvar-cli.md](../../docs/simvar-cli.md)).
