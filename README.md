# mrlm-net/simconnect

GoLang wrapper above SimConnect SDK, to allow Gophers create their flight simulator extensions, tools and controllers.

| | |
|--|--|
| Package name | github.com/mrlm-net/simconnect |
| Go version | 1.21+ |
| License | Apache 2.0 License |
| Platform | Windows only |

## Table of contents

• [Installation](#installation)  
• [Usage](#usage)  
• [Examples](#examples)  
• [API Reference](#api-reference)  
• [Debugging](#debugging)  
• [Advanced Usage](#advanced-usage)  
• [Contributing](#contributing)

## Installation

I'm using `go mod` so examples will be using it, you can install this package via Go modules.

```bash
go get github.com/mrlm-net/simconnect
```

**Requirements:**
- Windows OS (required for SimConnect SDK)
- Microsoft Flight Simulator or other SimConnect-compatible simulator
- SimConnect SDK (usually bundled with MSFS SDK)

## Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/mrlm-net/simconnect/pkg/client"
)

func main() {
    // Create a new SimConnect client
    simClient := client.New("MyFlightApp")
    if simClient == nil {
        log.Fatal("Failed to create SimConnect client")
    }

    // Connect to SimConnect
    if err := simClient.Connect(); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer simClient.Disconnect()

    // Process messages from SimConnect
    for message := range simClient.Stream() {
        if message.Error != nil {
            fmt.Printf("Error: %v\n", message.Error)
            continue
        }

        switch {
        case message.IsSimObjectData():
            fmt.Println("Received sim object data")
        case message.IsEvent():
            fmt.Println("Received event")
        case message.IsException():
            fmt.Println("Received exception")
        }
    }
}
```


**Documentation**: [SimConnect Inspector - Microsoft Docs](https://docs.flightsimulator.com/html/Developer_Mode/Menus/Tools/SimConnect_Inspector.htm)

## Examples

The `examples/` directory contains practical demonstrations of SimConnect features:

| Example | Description | Key Features |
|---------|-------------|--------------|
| [simple-controller](examples/simple-controller/) | Basic aircraft state monitoring and control | SimVar reading, Event triggering, External power control |
| [airplane-state](examples/airplane-state/) | Complete aircraft telemetry with web dashboard | Data definitions, HTTP server, Real-time telemetry |
| [airplane-doors](examples/airplane-doors/) | Interactive aircraft door control | Event handling, Keyboard input, Door toggle operations |
| [camera-controller](examples/camera-controller/) | Advanced camera control with web interface | Real-time camera monitoring, Interactive controls, Web dashboard |
| [camera-state](examples/camera-state/) | Camera view monitoring and switching | Camera state tracking, View changes, State monitoring |
| [message-parsing](examples/message-parsing/) | Message handling patterns | Signal handling, Graceful shutdown, Message parsing |

Run any example:
```bash
cd examples/simple-controller
go run main.go
```

## API Reference

### Core Components

#### Client Creation
```go
client := client.New("AppName")    // Create new client
err := client.Connect()            // Connect to SimConnect
defer client.Disconnect()         // Always disconnect
```

#### Message Processing
```go
for message := range client.Stream() {
    switch {
    case message.IsSimObjectData():  // Aircraft data
    case message.IsEvent():          // Event notifications  
    case message.IsException():      // SimConnect exceptions
    case message.IsOpen():           // Connection confirmed
    case message.IsQuit():           // Shutdown signal
    }
}
```

#### Data Definitions
```go
// Define data structure
client.AddToDataDefinition(defID, "PLANE ALTITUDE", "feet", types.DATATYPE_FLOAT64)
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER)
```

#### Event Handling
```go
// Map and trigger events
client.MapClientEventToSimEvent(eventID, "TOGGLE_EXTERNAL_POWER")
client.TransmitClientEvent(eventID, 0)
```

## Debugging

Enable SimConnect logging in MSFS Developer Mode:
1. Open Developer Mode → Windows → SimConnect Inspector
2. Monitor connection status and message flow
3. Check for exceptions and data validation errors

Common issues:
- **Connection failed**: Ensure MSFS is running and SimConnect is enabled
- **Data not received**: Verify data definitions match SimConnect documentation
- **Events not working**: Check event names against MSFS Key Events documentation

## Advanced Usage

For complex scenarios, see [Advanced Documentation](docs/):
- [Data Handling](docs/data-handling.md) - SimVar management and custom data types
- [Event System](docs/event-system.md) - Advanced event handling and system events
- [Error Handling](docs/error-handling.md) - Exception management and recovery
- [Performance](docs/performance.md) - Optimization and best practices

## Contributing

Contributions are welcomed and must follow Code of Conduct and common [Contributions guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md).

> If you'd like to report security issue please follow security guidelines.

All rights reserved © Martin Hrášek [<@marley-ma>](https://github.com/marley-ma) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)
