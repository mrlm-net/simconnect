# Basic Usage Guide

This guide walks you through common operations with the SimConnect Go library, from basic setup to reading aircraft data and sending commands.

## Table of Contents

- [Getting Started](#getting-started)
- [Reading Aircraft Data](#reading-aircraft-data)
- [Sending Commands](#sending-commands)
- [Monitoring Events](#monitoring-events)
- [Working with System State](#working-with-system-state)
- [Error Handling](#error-handling)
- [Common Patterns](#common-patterns)

## Getting Started

### Prerequisites

Before using the library, ensure you have:

1. **Windows OS** - SimConnect only works on Windows
2. **Flight Simulator** - MSFS 2020/2024 or compatible simulator
3. **SimConnect SDK** - Usually bundled with MSFS SDK
4. **Go 1.21+** - Required Go version

### Basic Setup

Create a simple application that connects to the simulator:

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/mrlm-net/simconnect/pkg/client"
)

func main() {
    // Create SimConnect client
    sc := client.New("Basic Example")
    if sc == nil {
        log.Fatal("Failed to create SimConnect client")
    }
    defer sc.Disconnect()
    
    // Connect to simulator
    fmt.Println("Connecting to Flight Simulator...")
    if err := sc.Connect(); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    fmt.Println("Connected successfully!")
    
    // Start message processing
    messageStream := sc.Stream()
    
    // Process messages in a goroutine
    go func() {
        for msg := range messageStream {
            if msg.Error != nil {
                log.Printf("Message error: %v", msg.Error)
                continue
            }
            
            switch {
            case msg.IsOpen():
                fmt.Println("Connection confirmed!")
            case msg.IsException():
                if exc, ok := msg.GetException(); ok {
                    log.Printf("SimConnect exception: %d", exc.DwException)
                }
            }
        }
    }()
    
    // Wait for interrupt signal
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
    
    fmt.Println("Shutting down...")
}
```

## Reading Aircraft Data

### Step 1: Define Data Structure

Create a Go struct that matches the data you want to read:

```go
// Aircraft data structure - order matters!
type AircraftState struct {
    Altitude    float64  // PLANE ALTITUDE in feet
    Speed       float64  // GROUND VELOCITY in knots
    Latitude    float64  // PLANE LATITUDE in degrees
    Longitude   float64  // PLANE LONGITUDE in degrees
    Heading     float64  // PLANE HEADING DEGREES TRUE in degrees
}
```

### Step 2: Create Data Definition

Define what variables you want SimConnect to provide:

```go
const AIRCRAFT_DATA_ID = 1

func setupDataDefinition(sc *client.Engine) error {
    // Add each variable to the definition
    vars := []struct {
        name     string
        units    string
        dataType types.SIMCONNECT_DATATYPE
    }{
        {"PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64},
        {"GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
        {"PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64},
        {"PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64},
        {"PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64},
    }
    
    for i, v := range vars {
        err := sc.AddToDataDefinition(
            AIRCRAFT_DATA_ID,  // Definition ID
            v.name,           // Variable name
            v.units,          // Units
            v.dataType,       // Data type
            0.0,              // Epsilon (change threshold)
            i,                // Datum ID
        )
        if err != nil {
            return fmt.Errorf("failed to add %s: %v", v.name, err)
        }
    }
    
    return nil
}
```

### Step 3: Request Data Updates

Ask SimConnect to send data updates:

```go
const DATA_REQUEST_ID = 1

func requestAircraftData(sc *client.Engine) error {
    return sc.RequestDataOnSimObject(
        DATA_REQUEST_ID,                           // Request ID
        AIRCRAFT_DATA_ID,                         // Data definition ID
        types.SIMCONNECT_OBJECT_ID_USER,          // User aircraft
        types.SIMCONNECT_PERIOD_SIM_FRAME,        // Every sim frame
        types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, // Only when changed
        0, 1, 0,                                  // Origin, interval, limit
    )
}
```

### Step 4: Process Data Messages

Handle incoming data in your message loop:

```go
func processMessages(sc *client.Engine) {
    messages := sc.Stream()
    
    for msg := range messages {
        switch {
        case msg.IsSimObjectData():
            if data, ok := msg.GetSimObjectData(); ok {
                if data.DwRequestID == DATA_REQUEST_ID {
                    processAircraftData(data)
                }
            }
        case msg.IsException():
            if exc, ok := msg.GetException(); ok {
                log.Printf("SimConnect error: %d", exc.DwException)
            }
        }
    }
}

func processAircraftData(data *types.SIMCONNECT_RECV_SIMOBJECT_DATA) {
    // Cast raw data to our struct
    aircraft := (*AircraftState)(unsafe.Pointer(&data.DwData))
    
    fmt.Printf("Aircraft State:\n")
    fmt.Printf("  Altitude: %.0f ft\n", aircraft.Altitude)
    fmt.Printf("  Speed: %.1f kts\n", aircraft.Speed)
    fmt.Printf("  Position: %.6f°, %.6f°\n", aircraft.Latitude, aircraft.Longitude)
    fmt.Printf("  Heading: %.0f°\n", aircraft.Heading)
    fmt.Println()
}
```

### Complete Example

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "unsafe"
    
    "github.com/mrlm-net/simconnect/pkg/client"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    AIRCRAFT_DATA_ID = 1
    DATA_REQUEST_ID  = 1
)

type AircraftState struct {
    Altitude  float64
    Speed     float64
    Latitude  float64
    Longitude float64
    Heading   float64
}

func main() {
    sc := client.New("Aircraft Monitor")
    if sc == nil {
        log.Fatal("Failed to create client")
    }
    defer sc.Disconnect()
    
    if err := sc.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    
    // Setup data definition
    if err := setupDataDefinition(sc); err != nil {
        log.Fatal("Data definition failed:", err)
    }
    
    // Start message processing
    messageStream := sc.Stream()
    
    // Request data
    if err := requestAircraftData(sc); err != nil {
        log.Fatal("Data request failed:", err)
    }
    
    // Process messages
    go processMessages(sc)
    
    // Wait for interrupt
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
}

// ... (implement setupDataDefinition, requestAircraftData, processMessages as shown above)
```

## Sending Commands

Sending commands to the simulator involves mapping events and transmitting them.

### Step 1: Map Events

```go
const (
    GEAR_TOGGLE_EVENT = 1
    FLAPS_UP_EVENT    = 2
    FLAPS_DOWN_EVENT  = 3
)

func setupEvents(sc *client.Engine) error {
    events := map[int]string{
        GEAR_TOGGLE_EVENT: "GEAR_TOGGLE",
        FLAPS_UP_EVENT:    "FLAPS_UP",
        FLAPS_DOWN_EVENT:  "FLAPS_DOWN",
    }
    
    for id, name := range events {
        if err := sc.MapClientEventToSimEvent(id, name); err != nil {
            return fmt.Errorf("failed to map event %s: %v", name, err)
        }
    }
    
    return nil
}
```

### Step 2: Send Commands

```go
func toggleGear(sc *client.Engine) error {
    return sc.TransmitClientEvent(
        types.SIMCONNECT_OBJECT_ID_USER,  // Target: user aircraft
        GEAR_TOGGLE_EVENT,                // Event ID
        0,                                // No data
        0,                                // Default group
    )
}

func setFlaps(sc *client.Engine, up bool) error {
    eventID := FLAPS_DOWN_EVENT
    if up {
        eventID = FLAPS_UP_EVENT
    }
    
    return sc.TransmitClientEvent(
        types.SIMCONNECT_OBJECT_ID_USER,
        eventID,
        0,
        0,
    )
}
```

### Command with Parameters

Some events accept parameters:

```go
const SET_HEADING_EVENT = 4

func setHeading(sc *client.Engine, heading int) error {
    // Map event that accepts a parameter
    if err := sc.MapClientEventToSimEvent(SET_HEADING_EVENT, "HEADING_BUG_SET"); err != nil {
        return err
    }
    
    // Send event with heading value
    return sc.TransmitClientEvent(
        types.SIMCONNECT_OBJECT_ID_USER,
        SET_HEADING_EVENT,
        heading,  // Heading in degrees
        0,
    )
}
```

## Monitoring Events

Listen for events from the simulator:

### Step 1: Setup Event Groups

```go
const AIRCRAFT_EVENTS_GROUP = 1

func setupEventMonitoring(sc *client.Engine) error {
    // Add events to notification group
    events := []int{GEAR_TOGGLE_EVENT, FLAPS_UP_EVENT, FLAPS_DOWN_EVENT}
    
    for _, eventID := range events {
        err := sc.AddClientEventToNotificationGroup(AIRCRAFT_EVENTS_GROUP, eventID)
        if err != nil {
            return err
        }
    }
    
    // Set group priority
    return sc.SetNotificationGroupPriority(AIRCRAFT_EVENTS_GROUP, 100)
}
```

### Step 2: Handle Event Messages

```go
func processEventMessages(sc *client.Engine) {
    messages := sc.Stream()
    
    for msg := range messages {
        if msg.IsEvent() {
            if event, ok := msg.GetEvent(); ok {
                handleEvent(event)
            }
        }
    }
}

func handleEvent(event *types.SIMCONNECT_RECV_EVENT) {
    switch event.UEventID {
    case GEAR_TOGGLE_EVENT:
        fmt.Println("Landing gear toggled")
    case FLAPS_UP_EVENT:
        fmt.Println("Flaps raised")
    case FLAPS_DOWN_EVENT:
        fmt.Println("Flaps lowered")
    default:
        fmt.Printf("Unknown event: %d\n", event.UEventID)
    }
}
```

## Working with System State

Query and monitor simulator system state:

### Requesting System State

```go
const SIM_STATE_REQUEST = 100

func checkSimulatorState(sc *client.Engine) error {
    // Request various system states
    states := []string{"Sim", "DialogMode", "Flight"}
    
    for i, state := range states {
        err := sc.RequestSystemState(SIM_STATE_REQUEST+i, state)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### Processing State Responses

```go
func processSystemState(msg *ParsedMessage) {
    if state, ok := msg.GetSystemState(); ok {
        switch state.DwRequestID {
        case SIM_STATE_REQUEST:
            fmt.Printf("Sim state: %s\n", state.SzString)
        case SIM_STATE_REQUEST + 1:
            fmt.Printf("Dialog mode: %s\n", state.SzString)
        case SIM_STATE_REQUEST + 2:
            fmt.Printf("Flight state: %s\n", state.SzString)
        }
    }
}
```

## Error Handling

Proper error handling is crucial for stable applications:

### Connection Errors

```go
func connectWithRetry(sc *client.Engine, maxRetries int) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = sc.Connect()
        if err == nil {
            return nil
        }
        
        // Check specific error conditions
        errStr := err.Error()
        switch {
        case strings.Contains(errStr, "0x80040108"):
            fmt.Println("Simulator not running, retrying in 5 seconds...")
            time.Sleep(5 * time.Second)
        case strings.Contains(errStr, "0x80040109"):
            return fmt.Errorf("connection limit reached")
        default:
            fmt.Printf("Connection attempt %d failed: %v\n", i+1, err)
            time.Sleep(2 * time.Second)
        }
    }
    return fmt.Errorf("failed to connect after %d attempts: %v", maxRetries, err)
}
```

### Message Processing Errors

```go
func safeMessageProcessing(sc *client.Engine) {
    messages := sc.Stream()
    
    for msg := range messages {
        // Check for parsing errors
        if msg.Error != nil {
            log.Printf("Message parsing error: %v", msg.Error)
            continue
        }
        
        // Handle exceptions
        if msg.IsException() {
            if exc, ok := msg.GetException(); ok {
                handleException(exc)
            }
            continue
        }
        
        // Process normal messages
        processMessage(msg)
    }
}

func handleException(exc *types.SIMCONNECT_RECV_EXCEPTION) {
    switch exc.DwException {
    case types.SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED:
        log.Println("Unrecognized variable or event name")
    case types.SIMCONNECT_EXCEPTION_OBJECT_NOT_FOUND:
        log.Println("Object not found")
    case types.SIMCONNECT_EXCEPTION_DATA_ERROR:
        log.Println("Data error")
    default:
        log.Printf("SimConnect exception: %d", exc.DwException)
    }
}
```

## Common Patterns

### Data Polling with Timeout

```go
func pollAircraftData(sc *client.Engine, timeout time.Duration) (*AircraftState, error) {
    // Request data
    requestID := int(time.Now().UnixNano())
    err := sc.RequestDataOnSimObject(
        requestID, AIRCRAFT_DATA_ID,
        types.SIMCONNECT_OBJECT_ID_USER,
        types.SIMCONNECT_PERIOD_ONCE,
        0, 0, 1, 1,
    )
    if err != nil {
        return nil, err
    }
    
    // Wait for response with timeout
    messages := sc.Stream()
    timeoutChan := time.After(timeout)
    
    for {
        select {
        case msg := <-messages:
            if msg.IsSimObjectData() {
                if data, ok := msg.GetSimObjectData(); ok && data.DwRequestID == uint32(requestID) {
                    return (*AircraftState)(unsafe.Pointer(&data.DwData)), nil
                }
            }
        case <-timeoutChan:
            return nil, fmt.Errorf("timeout waiting for data")
        }
    }
}
```

### Event-driven State Machine

```go
type FlightPhase int

const (
    OnGround FlightPhase = iota
    Taxi
    Takeoff
    Climb
    Cruise
    Descent
    Approach
    Landing
)

type FlightMonitor struct {
    sc          *client.Engine
    currentPhase FlightPhase
    lastAltitude float64
}

func (fm *FlightMonitor) updatePhase(aircraft *AircraftState) {
    switch fm.currentPhase {
    case OnGround:
        if aircraft.Speed > 5 {
            fm.currentPhase = Taxi
            fmt.Println("Phase: Taxi")
        }
    case Taxi:
        if aircraft.Speed > 60 && aircraft.Altitude > 100 {
            fm.currentPhase = Takeoff
            fmt.Println("Phase: Takeoff")
        }
    case Takeoff:
        if aircraft.Altitude > 1000 {
            fm.currentPhase = Climb
            fmt.Println("Phase: Climb")
        }
    // ... additional phase logic
    }
    fm.lastAltitude = aircraft.Altitude
}
```

### Graceful Shutdown

```go
func runWithGracefulShutdown(sc *client.Engine) {
    // Setup shutdown channel
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
    
    // Start processing
    go processMessages(sc)
    
    // Wait for shutdown signal
    <-shutdown
    
    fmt.Println("Shutting down...")
    
    // Stop message processing
    if err := sc.Disconnect(); err != nil {
        log.Printf("Disconnect error: %v", err)
    }
    
    fmt.Println("Shutdown complete")
}
```

These patterns provide a solid foundation for building robust SimConnect applications. Remember to always handle errors appropriately and test your applications thoroughly with the simulator.
