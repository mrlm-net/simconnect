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
• [Documentation](#documentation)
• [API Reference](#api-reference)  
• [Use Cases](#use-cases)
• [Features](#features)
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

The SimConnect Go library provides a comprehensive wrapper around Microsoft's SimConnect SDK, enabling Go developers to create flight simulator extensions, tools, and controllers. Here's a quick start example:

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/mrlm-net/simconnect/pkg/client"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
    // Create a new SimConnect client
    sc := client.New("My Flight Sim App")
    if sc == nil {
        log.Fatal("Failed to create SimConnect client")
    }
    defer sc.Disconnect()
    
    // Connect to the simulator
    if err := sc.Connect(); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    
    fmt.Println("Connected to Flight Simulator!")
    
    // Start processing messages
    messageStream := sc.Stream()
    
    // Process messages
    for msg := range messageStream {
        if msg.Error != nil {
            log.Printf("Message error: %v", msg.Error)
            continue
        }
        
        switch {
        case msg.IsException():
            if exc, ok := msg.GetException(); ok {
                fmt.Printf("SimConnect exception: %d\n", exc.DwException)
            }
        case msg.IsOpen():
            fmt.Println("Connection confirmed!")
        }
    }
}
```

For more detailed usage patterns and advanced features, see the [Documentation](#documentation) section below.

## Examples

The `examples/` directory contains several working examples demonstrating various library features:

- **[airplane-state](examples/airplane-state/)** - Real-time aircraft state monitoring with web interface
- **[airplane-doors](examples/airplane-doors/)** - Door control system example
- **[camera-controller](examples/camera-controller/)** - Camera position and movement control
- **[camera-state](examples/camera-state/)** - Camera state monitoring
- **[message-parsing](examples/message-parsing/)** - Low-level message parsing example
- **[simple-controller](examples/simple-controller/)** - Basic aircraft control operations

Each example includes its own README with specific setup and usage instructions.

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[API Reference](docs/api-reference.md)** - Complete API documentation with method signatures and parameters
- **[Core Concepts](docs/concepts.md)** - Understanding SimConnect concepts and how they map to the Go library
- **[Basic Usage Guide](docs/basic-usage.md)** - Step-by-step guide for common operations
- **[Advanced Usage](docs/advanced-usage.md)** - Advanced patterns, performance optimization, and best practices
- **[Message Types](docs/message-types.md)** - Complete reference for all supported message types
- **[Data Types](docs/data-types.md)** - SimConnect data types and their Go equivalents
- **[Error Handling](docs/error-handling.md)** - Comprehensive error handling strategies
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

## API Reference

For quick reference, the main client methods include:

### Connection Management
- `client.New(name)` - Create new client instance
- `Connect()` - Establish connection to simulator
- `Disconnect()` - Close connection and cleanup

### Data Operations
- `AddToDataDefinition()` - Define data structures for sim variables
- `RequestDataOnSimObject()` - Request periodic data updates
- `SetDataOnSimObject()` - Set sim variable values

### Event Operations
- `MapClientEventToSimEvent()` - Map custom events to sim events
- `TransmitClientEvent()` - Send events to simulator
- `AddClientEventToNotificationGroup()` - Group events for organization

### Message Processing
- `Stream()` - Start message processing and return message channel
- `DispatchProc()` - Custom message dispatch handling

## Debugging

Enable debug logging by setting the environment variable:

```bash
export SIMCONNECT_DEBUG=1
```

This will output detailed information about:
- Connection establishment
- Message parsing and processing
- API call results
- Error conditions

## Advanced Usage

The library supports advanced scenarios including:

- **Real-time Data Streaming** - Efficient handling of high-frequency sim data
- **Event-driven Architecture** - Reactive programming patterns with sim events
- **Multi-client Support** - Managing multiple concurrent connections
- **Custom Message Processing** - Low-level message handling for specialized needs
- **Performance Optimization** - Techniques for high-performance applications

See the [Advanced Usage Guide](docs/advanced-usage.md) for detailed examples and best practices.

## Features

This library provides a comprehensive Go wrapper for the SimConnect SDK with the following capabilities:

### Core Features
- **Full SimConnect API Coverage** - Complete implementation of SimConnect functions
- **Type-Safe Operations** - Strongly typed Go interfaces for all SimConnect operations
- **Asynchronous Message Processing** - Non-blocking message handling with channels
- **Automatic Memory Management** - Safe handling of SimConnect memory and resources
- **Error Handling** - Comprehensive error reporting and exception handling
- **Cross-Version Compatibility** - Works with Microsoft Flight Simulator 2020 and 2024

### Data Operations
- **Simulation Variables** - Read/write aircraft, environment, and system variables
- **Real-time Monitoring** - Stream aircraft state, position, and flight parameters
- **Custom Data Definitions** - Define structured data requests for multiple variables
- **Data Filtering** - Request data only when values change to optimize performance
- **Multiple Update Frequencies** - Per-frame, per-second, or one-time data requests

### Event System
- **Event Mapping** - Map custom events to simulator events
- **Event Transmission** - Send commands and key events to the simulator
- **Event Notifications** - Receive notifications when events occur
- **Event Groups** - Organize events into logical groups with priorities
- **System Events** - Subscribe to simulator lifecycle and timing events

### Object Management
- **Aircraft Control** - Control user aircraft and AI aircraft
- **Object Tracking** - Monitor AI objects (aircraft, vehicles, boats)
- **Object Creation** - Add AI objects to the simulation
- **Multi-object Support** - Work with multiple simulation objects simultaneously

### Advanced Features
- **Facility Data** - Access airport, navigation aid, and waypoint information
- **Client Data Areas** - Share data between SimConnect clients
- **Input Events** - Handle and generate input events
- **Weather Integration** - Read and modify weather conditions
- **Flight Plans** - Access and manipulate flight plan data

### Development Support
- **Rich Examples** - Multiple working examples for different use cases
- **Comprehensive Documentation** - Detailed guides and API reference
- **Debug Support** - Built-in logging and debugging capabilities
- **Performance Optimization** - Efficient message processing and memory usage

## Use Cases

This library enables a wide range of applications for flight simulation enthusiasts and developers:

### Flight Training Tools
- **Performance Monitoring** - Track aircraft performance metrics during training flights
- **Approach Analysis** - Monitor approach angles, speeds, and landing statistics
- **Flight Replay Systems** - Record and replay flight data for analysis
- **Instructor Dashboards** - Real-time monitoring tools for flight instructors

### Aircraft Systems Simulation
- **Custom Instruments** - Build external aircraft instruments and displays
- **System Controllers** - Create hardware interfaces for aircraft systems
- **Engine Monitoring** - Track engine parameters and performance
- **Navigation Systems** - Build custom GPS and navigation displays

### Air Traffic and Operations
- **Traffic Monitoring** - Track AI aircraft and their flight paths
- **Airport Operations** - Monitor ground vehicles and aircraft at airports
- **Fleet Management** - Track multiple aircraft in multiplayer scenarios
- **Weather Stations** - Monitor and log weather conditions

### Entertainment and Utilities
- **Flight Tracking** - Live flight tracking with web interfaces
- **Screen Overlays** - In-game information overlays and HUDs
- **Voice Control** - Voice-activated aircraft controls
- **Mobile Companions** - Mobile apps that connect to your flight

### Research and Analysis
- **Flight Data Collection** - Gather flight data for research purposes
- **Performance Analysis** - Analyze aircraft and pilot performance
- **Route Optimization** - Study flight paths and fuel efficiency
- **Weather Impact Studies** - Analyze weather effects on flight operations

### Hardware Integration
- **Physical Controls** - Connect real aircraft controls and switches
- **Motion Platforms** - Drive motion simulators based on flight data
- **LED Displays** - Control external LED displays showing flight information
- **Haptic Feedback** - Provide tactile feedback based on flight conditions

### Web and Mobile Applications
- **Flight Dashboards** - Web-based flight information displays
- **Mobile Monitoring** - Monitor flights from mobile devices
- **Live Streaming** - Add flight data to streaming overlays
- **Social Features** - Share flight progress with friends and communities

### Educational Applications
- **STEM Education** - Teach physics and mathematics through flight simulation
- **Aviation Training** - Supplement pilot training with custom tools
- **Engineering Projects** - University and college aviation engineering projects
- **Research Platforms** - Academic research in aviation and aerodynamics

## Contributing

Contributions are welcomed and must follow Code of Conduct and common [Contributions guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md).

> If you'd like to report security issue please follow security guidelines.

All rights reserved © Martin Hrášek [<@marley-ma>](https://github.com/marley-ma) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)
