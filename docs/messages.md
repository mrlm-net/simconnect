# Message Handling Reference

This document covers how to process and handle messages received from SimConnect using the client API.

## Table of Contents

• [Message Structure](#message-structure)  
• [Message Types](#message-types)  
• [Message Processing](#message-processing)  
• [Helper Methods](#helper-methods)  
• [Common Patterns](#common-patterns)

## Message Structure

All messages from SimConnect are wrapped in the `ParsedMessage` structure:

```go
type ParsedMessage struct {
    MessageType types.SIMCONNECT_RECV_ID // The type of message received
    Header      *types.SIMCONNECT_RECV   // Base header information
    Data        interface{}              // Parsed message data (specific type based on MessageType)
    RawData     []byte                   // Raw byte data for custom parsing if needed
    Error       error                    // Any parsing error that occurred
}
```

### Fields Explanation

- **MessageType**: Identifies what kind of message this is (event, data, exception, etc.)
- **Header**: Contains basic information like message size and version
- **Data**: The actual message data, cast to the appropriate type based on MessageType
- **RawData**: Raw bytes from SimConnect, useful for custom parsing or debugging
- **Error**: Any error that occurred during message parsing

## Message Types

### Connection Messages

#### `SIMCONNECT_RECV_ID_OPEN`
Received when connection to SimConnect is established.

```go
if message.IsOpen() {
    fmt.Println("Connected to SimConnect")
}
```

#### `SIMCONNECT_RECV_ID_QUIT`
Received when SimConnect is shutting down or connection is lost.

```go
if message.IsQuit() {
    fmt.Println("SimConnect connection closed")
    return // Exit message processing loop
}
```

### Data Messages

#### `SIMCONNECT_RECV_ID_SIMOBJECT_DATA`
Contains simulation object data requested via `RequestDataOnSimObject`.

```go
if message.IsSimObjectData() {
    if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
        fmt.Printf("Received data for request %d, object %d\n", 
            data.DwRequestID, data.DwObjectID)
        // Parse actual data from message.RawData
    }
}
```

### Event Messages

#### `SIMCONNECT_RECV_ID_EVENT`
Contains event notifications from the simulator.

```go
if message.IsEvent() {
    if event, ok := message.Data.(*types.SIMCONNECT_RECV_EVENT); ok {
        fmt.Printf("Event %d triggered with data %d\n", 
            event.UEventID, event.DwData)
    }
}
```

### Error Messages

#### `SIMCONNECT_RECV_ID_EXCEPTION`
Contains exception information when errors occur.

```go
if message.IsException() {
    if exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION); ok {
        fmt.Printf("Exception %d occurred for packet %d\n", 
            exception.DwException, exception.DwSendID)
    }
}
```

## Message Processing

### Basic Message Loop

```go
for message := range client.Stream() {
    // Always check for parsing errors first
    if message.Error != nil {
        fmt.Printf("Message parsing error: %v\n", message.Error)
        continue
    }

    // Process based on message type
    switch message.MessageType {
    case types.SIMCONNECT_RECV_ID_OPEN:
        fmt.Println("Connected to SimConnect")
        
    case types.SIMCONNECT_RECV_ID_QUIT:
        fmt.Println("SimConnect disconnected")
        return
        
    case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
        handleSimObjectData(message)
        
    case types.SIMCONNECT_RECV_ID_EVENT:
        handleEvent(message)
        
    case types.SIMCONNECT_RECV_ID_EXCEPTION:
        handleException(message)
        
    default:
        fmt.Printf("Unhandled message type: %d\n", message.MessageType)
    }
}
```

### Type-Safe Message Handlers

```go
func handleSimObjectData(message client.ParsedMessage) {
    data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA)
    if !ok {
        fmt.Println("Failed to cast to SIMCONNECT_RECV_SIMOBJECT_DATA")
        return
    }

    fmt.Printf("Data received - Request: %d, Object: %d, Definition: %d\n",
        data.DwRequestID, data.DwObjectID, data.DwDefineID)

    // Process the actual data payload
    processDataPayload(message.RawData, data)
}

func handleEvent(message client.ParsedMessage) {
    event, ok := message.Data.(*types.SIMCONNECT_RECV_EVENT)
    if !ok {
        fmt.Println("Failed to cast to SIMCONNECT_RECV_EVENT")
        return
    }

    fmt.Printf("Event received - Group: %d, Event: %d, Data: %d\n",
        event.UGroupID, event.UEventID, event.DwData)
}

func handleException(message client.ParsedMessage) {
    exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION)
    if !ok {
        fmt.Println("Failed to cast to SIMCONNECT_RECV_EXCEPTION")
        return
    }

    fmt.Printf("Exception %d occurred for packet %d (index: %d)\n",
        exception.DwException, exception.DwSendID, exception.DwIndex)
}
```

## Helper Methods

The `ParsedMessage` struct provides convenient helper methods for type checking:

### `IsSimObjectData() bool`
Returns true if the message contains simulation object data.

```go
if message.IsSimObjectData() {
    // Handle sim object data
}
```

### `IsEvent() bool`
Returns true if the message is an event notification.

```go
if message.IsEvent() {
    // Handle event
}
```

### `IsException() bool`
Returns true if the message is an exception.

```go
if message.IsException() {
    // Handle exception
}
```

### `IsOpen() bool`
Returns true if the message is a connection open confirmation.

```go
if message.IsOpen() {
    // Connection established
}
```

### `IsQuit() bool`
Returns true if the message is a connection quit notification.

```go
if message.IsQuit() {
    // Connection closed
    return
}
```

## Common Patterns

### Data Parsing with Struct Mapping

```go
import "unsafe"

// Define your data structure to match the SimConnect data layout
type AircraftData struct {
    Altitude float64 // Must match the order and types in your data definition
    Airspeed float64
    Heading  float64
}

func parseAircraftData(message client.ParsedMessage) (*AircraftData, error) {
    data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA)
    if !ok {
        return nil, fmt.Errorf("invalid message type")
    }

    // Calculate offset to actual data
    headerSize := unsafe.Sizeof(*data)
    if len(message.RawData) < int(headerSize+unsafe.Sizeof(AircraftData{})) {
        return nil, fmt.Errorf("insufficient data")
    }

    // Cast raw data to our structure
    dataPtr := unsafe.Pointer(&message.RawData[headerSize])
    aircraftData := (*AircraftData)(dataPtr)
    
    return aircraftData, nil
}

// Usage
for message := range client.Stream() {
    if message.IsSimObjectData() {
        if aircraftData, err := parseAircraftData(message); err == nil {
            fmt.Printf("Alt: %.1f, Speed: %.1f, Heading: %.1f\n",
                aircraftData.Altitude, aircraftData.Airspeed, aircraftData.Heading)
        }
    }
}
```

### Request-Based Message Routing

```go
type DataHandler func(client.ParsedMessage)

// Map request IDs to handlers
var requestHandlers = map[uint32]DataHandler{
    1: handleAircraftData,
    2: handleTrafficData,
    3: handleWeatherData,
}

func routeMessage(message client.ParsedMessage) {
    if message.IsSimObjectData() {
        if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
            if handler, exists := requestHandlers[data.DwRequestID]; exists {
                handler(message)
            } else {
                fmt.Printf("No handler for request ID: %d\n", data.DwRequestID)
            }
        }
    }
}

func handleAircraftData(message client.ParsedMessage) {
    // Handle aircraft-specific data
}

func handleTrafficData(message client.ParsedMessage) {
    // Handle traffic data
}
```

### Event-Driven State Machine

```go
type SystemState int

const (
    StateDisconnected SystemState = iota
    StateConnected
    StateReady
    StateError
)

type EventProcessor struct {
    state   SystemState
    client  *client.Engine
}

func (ep *EventProcessor) ProcessMessage(message client.ParsedMessage) {
    switch ep.state {
    case StateDisconnected:
        if message.IsOpen() {
            ep.state = StateConnected
            ep.initializeDataRequests()
        }
        
    case StateConnected:
        if message.IsSimObjectData() {
            ep.state = StateReady
            fmt.Println("System ready")
        } else if message.IsException() {
            ep.state = StateError
            ep.handleError(message)
        }
        
    case StateReady:
        ep.processNormalOperation(message)
        
    case StateError:
        if message.IsOpen() {
            ep.state = StateConnected
            ep.initializeDataRequests()
        }
    }
    
    // Always handle quit messages
    if message.IsQuit() {
        ep.state = StateDisconnected
    }
}
```

### Error Recovery

```go
func processWithRetry(message client.ParsedMessage) {
    if message.Error != nil {
        fmt.Printf("Message error: %v\n", message.Error)
        return
    }

    if message.IsException() {
        if exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION); ok {
            switch exception.DwException {
            case 0x80000001: // SIMCONNECT_EXCEPTION_ERROR
                fmt.Println("General SimConnect error")
            case 0x80000002: // SIMCONNECT_EXCEPTION_SIZE_MISMATCH
                fmt.Println("Data size mismatch - check data definitions")
            case 0x80000003: // SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID
                fmt.Println("Unrecognized ID - check request/definition IDs")
            default:
                fmt.Printf("Unknown exception: 0x%08X\n", exception.DwException)
            }
        }
    }
}
```

## Best Practices

1. **Always check for errors**: Check `message.Error` before processing message data
2. **Use type assertions safely**: Always check the `ok` return value when casting
3. **Handle connection state**: Monitor open/quit messages to track connection status
4. **Validate data size**: Ensure sufficient data before parsing raw bytes
5. **Use helper methods**: Leverage the `Is*()` methods for cleaner code
6. **Implement timeouts**: Don't let message processing block indefinitely

```go
// Good practice: comprehensive message handling
for message := range client.Stream() {
    // Error check
    if message.Error != nil {
        log.Printf("Message error: %v", message.Error)
        continue
    }

    // Connection state tracking
    if message.IsQuit() {
        log.Println("SimConnect disconnected")
        break
    }

    // Type-safe processing
    if message.IsSimObjectData() {
        if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
            // Validate before processing
            if validateDataMessage(data, message.RawData) {
                processSimObjectData(data, message.RawData)
            }
        }
    }
}
```
