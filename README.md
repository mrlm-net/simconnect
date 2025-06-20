# mrlm-net/simconnect

GoLang wrapper above SimConnect SDK, to allow Gophers create their flight simulator extensions, tools and controllers.

| | |
|--|--|
| Package name | github.com/mrlm-net/simconnect |
| Go version | 1.21+ |
| License | MIT License |
| Platform | Windows only |

## Table of contents

• [Installation](#installation)  
• [Usage](#usage)  
• [Examples](#examples)  
• [API Reference](#api-reference)  
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

## Examples

The package includes several practical examples in the [`examples/`](./examples/) directory:

- **[Simple Controller](./examples/simple-controller/)** - Monitor and control aircraft external power
- **[Camera State](./examples/camera-state/)** - Monitor camera position and orientation
- **[Message Parsing](./examples/message-parsing/)** - Parse and display different SimConnect message types

## API Reference

For detailed API documentation, see the [docs/](./docs/) directory:

- **[Client API](./docs/client.md)** - Core client functionality and connection management
- **[Types Reference](./docs/types.md)** - SimConnect data types and structures
- **[Events & Data](./docs/events-data.md)** - Working with events and simulation data
- **[Message Handling](./docs/messages.md)** - Processing SimConnect messages
- **[Error Handling](./docs/errors.md)** - Error handling and debugging

## Advanced Usage

### Working with Custom Data Definitions

```go
// Define custom data structure
type AircraftData struct {
    Altitude  float64
    Airspeed  float64
    Heading   float64
}

// Add data definition
err := client.AddToDataDefinition(
    1,                                    // Definition ID
    "PLANE ALTITUDE",                     // SimVar name
    "feet",                              // Units
    types.SIMCONNECT_DATATYPE_FLOAT64,   // Data type
    0.0,                                 // Epsilon
    0,                                   // Datum ID
)
```

### Event Handling

```go
// Map event to SimConnect
err := client.MapClientEventToSimEvent(1, "TOGGLE_EXTERNAL_POWER")

// Add event to notification group
err := client.AddClientEventToNotificationGroup(1, 1)

// Transmit event
err := client.TransmitClientEvent(0, 1, 0, 1)
```

See [Advanced Usage Guide](./docs/advanced.md) for more examples.

## Contributing

Contributions are welcomed and must follow Code of Conduct and common [Contributions guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md).

> If you'd like to report security issue please follow security guidelines.

All rights reserved © Martin Hrášek [<@marley-ma>](https://github.com/marley-ma) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)

### Client Interface

Coming soon...

### Event Types

Coming soon...

## Contributing

Contributions are welcomed and must follow Code of Conduct and common [Contributions guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md).

> If you'd like to report security issue please follow security guidelines.

All rights reserved © Martin Hrášek [<@marley-ma>](https://github.com/marley-ma) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)
