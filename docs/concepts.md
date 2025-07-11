# Core Concepts

This document explains the fundamental concepts of SimConnect and how they are implemented in this Go library.

## Table of Contents

- [SimConnect Overview](#simconnect-overview)
- [Connection Management](#connection-management)
- [Data Definitions](#data-definitions)
- [Simulation Objects](#simulation-objects)
- [Events](#events)
- [Message Processing](#message-processing)
- [Request-Response Pattern](#request-response-pattern)
- [Data Flow Architecture](#data-flow-architecture)

## SimConnect Overview

SimConnect is Microsoft's API for communicating with Flight Simulator. It provides a bridge between external applications and the simulator's internal systems, allowing you to:

- Read simulation variables (altitude, speed, position, etc.)
- Send commands and events to the simulator
- Monitor aircraft and environment state
- Control simulator behavior

### Key Characteristics

- **Asynchronous**: Most operations are request-based with responses coming via messages
- **Event-driven**: The simulator sends messages when data changes or events occur
- **Type-safe**: All data has specific types and units
- **Hierarchical**: Data and events are organized in logical groups

## Connection Management

### Client Instance

The `Engine` struct represents a SimConnect client connection:

```go
type Engine struct {
    ctx       context.Context      // Cancellation context
    cancel    context.CancelFunc   // Cancel function
    dll       *syscall.LazyDLL     // SimConnect DLL handle
    handle    uintptr             // Connection handle
    name      string              // Client application name
    queue     chan ParsedMessage  // Internal message queue
    wg        sync.WaitGroup     // WaitGroup to coordinate goroutines
    once      sync.Once          // Ensure cleanup happens only once
    isClosing bool               // Flag to indicate if we're shutting down
    mu        sync.RWMutex       // Mutex to protect isClosing flag
}
```

### Connection Lifecycle

1. **Creation** - `client.New()` creates an instance and loads the DLL
2. **Connection** - `Connect()` establishes communication with the simulator
3. **Operation** - Send requests and receive messages
4. **Disconnection** - `Disconnect()` cleanly closes the connection

**Example:**
```go
// Create client
sc := client.New("Weather Monitor")
defer sc.Disconnect()

// Connect to simulator
if err := sc.Connect(); err != nil {
    log.Fatal("Failed to connect:", err)
}

// Client is now ready for operations
```

## Data Definitions

Data definitions are templates that describe the structure of data you want to exchange with the simulator. They act as contracts between your application and SimConnect.

### Creating Data Definitions

A data definition consists of one or more simulation variables:

```go
// Define what data we want
const AIRCRAFT_DATA_DEF = 1

// Add variables to the definition
sc.AddToDataDefinition(AIRCRAFT_DATA_DEF, "PLANE ALTITUDE", "feet", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
sc.AddToDataDefinition(AIRCRAFT_DATA_DEF, "GROUND VELOCITY", "knots", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
sc.AddToDataDefinition(AIRCRAFT_DATA_DEF, "PLANE LATITUDE", "degrees", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
```

### Data Structure Mapping

Your Go struct must match the definition order exactly:

```go
type AircraftData struct {
    Altitude    float64  // Index 0: PLANE ALTITUDE
    GroundSpeed float64  // Index 1: GROUND VELOCITY  
    Latitude    float64  // Index 2: PLANE LATITUDE
}
```

### Variable Names and Units

SimConnect uses specific variable names and units. Common examples:

| Variable | Description | Common Units |
|----------|-------------|--------------|
| `PLANE ALTITUDE` | Aircraft altitude | `feet`, `meters` |
| `GROUND VELOCITY` | Ground speed | `knots`, `meters per second` |
| `PLANE LATITUDE` | Latitude position | `degrees`, `radians` |
| `PLANE LONGITUDE` | Longitude position | `degrees`, `radians` |
| `VERTICAL SPEED` | Climb/descent rate | `feet per minute` |
| `AIRSPEED INDICATED` | Indicated airspeed | `knots` |

## Simulation Objects

SimConnect works with different types of simulation objects:

### Object Types

- **User Aircraft** - The player's primary aircraft
- **AI Aircraft** - Computer-controlled aircraft
- **Ground Vehicles** - Vehicles on the ground
- **Boats** - Water-based vehicles

### Object Identification

Objects are identified by unique IDs:

```go
// User aircraft (most common)
types.SIMCONNECT_OBJECT_ID_USER

// All objects of a type
types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT
types.SIMCONNECT_SIMOBJECT_TYPE_GROUND
types.SIMCONNECT_SIMOBJECT_TYPE_BOAT
```

### Requesting Object Data

```go
// Request data for user aircraft every frame
sc.RequestDataOnSimObject(
    1,                                        // Request ID
    AIRCRAFT_DATA_DEF,                       // Data definition
    types.SIMCONNECT_OBJECT_ID_USER,         // User aircraft
    types.SIMCONNECT_PERIOD_SIM_FRAME,       // Every sim frame
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, // Only when changed
    0, 1, 0,                                 // Origin, interval, limit
)
```

## Events

Events represent actions or state changes in the simulator. They can be:

- **Sent to simulator** - Trigger actions (gear up, flaps down, etc.)
- **Received from simulator** - Notifications of state changes

### Event Mapping

Before using an event, map your client ID to a simulator event:

```go
const GEAR_TOGGLE_EVENT = 1

// Map client event 1 to simulator gear toggle
sc.MapClientEventToSimEvent(GEAR_TOGGLE_EVENT, "GEAR_TOGGLE")
```

### Sending Events

```go
// Toggle landing gear
sc.TransmitClientEvent(
    types.SIMCONNECT_OBJECT_ID_USER,  // Target: user aircraft
    GEAR_TOGGLE_EVENT,                // Event: gear toggle
    0,                                // No additional data
    0,                                // Default group
)
```

### Event Groups

Events can be organized into notification groups for management:

```go
const AIRCRAFT_EVENTS_GROUP = 1

// Add event to group
sc.AddClientEventToNotificationGroup(AIRCRAFT_EVENTS_GROUP, GEAR_TOGGLE_EVENT)

// Set group priority
sc.SetNotificationGroupPriority(AIRCRAFT_EVENTS_GROUP, 100)
```

## Message Processing

SimConnect communicates through messages. The library provides both high-level and low-level message handling.

### Message Types

Common message types include:

- `SIMCONNECT_RECV_ID_SIMOBJECT_DATA` - Simulation object data
- `SIMCONNECT_RECV_ID_EVENT` - Event notifications
- `SIMCONNECT_RECV_ID_EXCEPTION` - Error conditions
- `SIMCONNECT_RECV_ID_OPEN` - Connection established
- `SIMCONNECT_RECV_ID_QUIT` - Connection closed

### High-Level Processing

```go
// Start message stream processing
messageStream := sc.Stream()

// Process messages from the stream
for msg := range messageStream {
    if msg.Error != nil {
        log.Printf("Message error: %v", msg.Error)
        continue
    }
    
    switch {
    case msg.IsSimObjectData():
        if data, ok := msg.GetSimObjectData(); ok {
            // Handle simulation data
            processAircraftData(data)
        }
    case msg.IsEvent():
        if event, ok := msg.GetEvent(); ok {
            // Handle event
            processEvent(event)
        }
    case msg.IsException():
        if exc, ok := msg.GetException(); ok {
            // Handle error
            log.Printf("SimConnect exception: %d", exc.DwException)
        }
    }
}
```

### Message Structure

```go
type ParsedMessage struct {
    MessageType types.SIMCONNECT_RECV_ID  // Message type identifier
    Header      *types.SIMCONNECT_RECV    // Base header
    Data        interface{}               // Typed message data
    RawData     []byte                   // Raw bytes
    Error       error                    // Parsing error
}
```

## Request-Response Pattern

SimConnect follows an asynchronous request-response pattern:

1. **Send Request** - Your application sends a request with a unique ID
2. **Continue Processing** - Your application continues other work
3. **Receive Response** - SimConnect sends a response message with the same ID
4. **Match and Process** - Your application matches the response to the original request

### Example: System State Request

```go
const SIM_STATE_REQUEST = 100

// Request simulator state
sc.RequestSystemState(SIM_STATE_REQUEST, types.SIMCONNECT_SYSTEM_STATE_SIM)

// Later, in message processing...
messageStream := sc.Stream()
for msg := range messageStream {
    if msg.MessageType == types.SIMCONNECT_RECV_ID_SYSTEM_STATE {
        if state, ok := msg.Data.(*types.SIMCONNECT_RECV_SYSTEM_STATE); ok {
            if state.DwRequestID == SIM_STATE_REQUEST {
                // This is our response
                fmt.Printf("Sim state: %s", string(state.SzString[:]))
            }
        }
    }
}
```

## Data Flow Architecture

Understanding the data flow helps design efficient applications:

```
Your Application
       ↓ (Requests)
   SimConnect API
       ↓
 Flight Simulator
       ↓ (Events/Data)
   Message Queue
       ↓
  Message Parser
       ↓
   Your Handlers
```

### Efficient Data Handling

1. **Use Appropriate Periods** - Don't request data more frequently than needed
2. **Filter Changes** - Use `SIMCONNECT_DATA_REQUEST_FLAG_CHANGED` to reduce traffic
3. **Group Related Data** - Put related variables in the same definition
4. **Handle Errors** - Always check for exceptions and errors

### Performance Considerations

- **Message Queue Size** - The default buffer size is 100 messages
- **Processing Speed** - Process messages quickly to avoid queue backup
- **Memory Usage** - Large data definitions consume more memory
- **Network Traffic** - Minimize unnecessary data requests

### Example: Efficient Aircraft Monitoring

```go
// Group related variables in one definition
type FlightData struct {
    Altitude    float64
    Speed       float64
    Heading     float64
    Latitude    float64
    Longitude   float64
}

// Request data only when it changes, at reasonable frequency
sc.RequestDataOnSimObject(
    1, FLIGHT_DATA_DEF,
    types.SIMCONNECT_OBJECT_ID_USER,
    types.SIMCONNECT_PERIOD_SIM_FRAME,
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
    0, 5, 0,  // Every 5 frames, not every frame
)
```

This approach provides smooth data flow while minimizing performance impact on both your application and the simulator.
