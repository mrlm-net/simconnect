# Airplane Doors Example

Interactive aircraft door control system with keyboard input handling.

## What it demonstrates

- **Event System**: Use `TOGGLE_AIRCRAFT_EXIT` event with parameters
- **Interactive Input**: Real-time keyboard command processing
- **Concurrent Goroutines**: Separate message processing and input handling
- **Signal Handling**: Graceful shutdown on Ctrl+C or system signals
- **Parameter Events**: Pass door numbers (1-4) to SimConnect events

## How to run

```bash
cd examples/airplane-doors
go run main.go
```

## Controls

| Key | Action |
|-----|--------|
| `1` | Toggle Door 1 (main entry) |
| `2` | Toggle Door 2 (secondary) |
| `3` | Toggle Door 3 (emergency) |
| `4` | Toggle Door 4 (service) |
| `q` | Quit application |
| Ctrl+C | Emergency shutdown |

## Key code patterns

```go
// Event with parameters
client.MapClientEventToSimEvent(EVENT_TOGGLE_AIRCRAFT_EXIT, "TOGGLE_AIRCRAFT_EXIT")
client.TransmitClientEvent(EVENT_TOGGLE_AIRCRAFT_EXIT, uint32(doorNumber))

// Concurrent input handling
go keyboardInputHandler(simClient)

// Signal handling for graceful shutdown
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
```

## Aircraft Compatibility

- Works with aircraft that have configurable doors
- Door availability depends on aircraft model
- Some aircraft may have fewer than 4 doors
- Effect visible in external camera view

## Requirements

- Running MSFS with compatible aircraft
- Aircraft with configurable door systems
- External camera view recommended for visual feedback
