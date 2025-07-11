# Error Handling Guide

This document provides comprehensive guidance on handling errors when working with the SimConnect Go library.

## Table of Contents

- [Error Types](#error-types)
- [Connection Errors](#connection-errors)
- [SimConnect Exceptions](#simconnect-exceptions)
- [Message Processing Errors](#message-processing-errors)
- [Best Practices](#best-practices)
- [Error Recovery Strategies](#error-recovery-strategies)

## Error Types

The SimConnect Go library handles several types of errors:

### 1. Go Standard Errors

Standard Go errors returned by library functions:

```go
// Connection errors
if err := sc.Connect(); err != nil {
    log.Fatal("Failed to connect:", err)
}

// Data definition errors
if err := sc.AddToDataDefinition(1, "INVALID_VARIABLE", "feet", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0); err != nil {
    log.Printf("Failed to add data definition: %v", err)
}
```

### 2. SimConnect Exceptions

SimConnect-specific exceptions sent via messages:

```go
messageStream := sc.Stream()
for msg := range messageStream {
    if msg.IsException() {
        if exc, ok := msg.GetException(); ok {
            handleSimConnectException(exc)
        }
    }
}

func handleSimConnectException(exc *types.SIMCONNECT_RECV_EXCEPTION) {
    switch exc.DwException {
    case types.SIMCONNECT_EXCEPTION_ERROR:
        log.Printf("General SimConnect error: %d", exc.DwSendID)
    case types.SIMCONNECT_EXCEPTION_SIZE_MISMATCH:
        log.Printf("Data size mismatch for request: %d", exc.DwSendID)
    case types.SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID:
        log.Printf("Unrecognized ID: %d", exc.DwSendID)
    default:
        log.Printf("Unknown SimConnect exception: %d", exc.DwException)
    }
}
```

### 3. Message Parsing Errors

Errors during message parsing are stored in the `ParsedMessage`:

```go
for msg := range messageStream {
    if msg.Error != nil {
        log.Printf("Message parsing error: %v", msg.Error)
        continue
    }
    // Process valid message
}
```

## Connection Errors

### Common Connection Issues

1. **SimConnect DLL Not Found**
```go
sc := client.New("My App")
if sc == nil {
    log.Fatal("Failed to create client - DLL not found or invalid")
}
```

2. **Simulator Not Running**
```go
if err := sc.Connect(); err != nil {
    if strings.Contains(err.Error(), "0x80004005") {
        log.Fatal("Flight Simulator is not running")
    }
    log.Fatal("Connection failed:", err)
}
```

3. **Permission Issues**
```go
if err := sc.Connect(); err != nil {
    if strings.Contains(err.Error(), "0x80070005") {
        log.Fatal("Access denied - run as administrator or check SimConnect permissions")
    }
    log.Fatal("Connection failed:", err)
}
```

### Robust Connection Handling

```go
func connectWithRetry(sc *client.Engine, maxAttempts int) error {
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        err := sc.Connect()
        if err == nil {
            log.Printf("Connected successfully on attempt %d", attempt)
            return nil
        }
        
        log.Printf("Connection attempt %d failed: %v", attempt, err)
        
        if attempt < maxAttempts {
            waitTime := time.Duration(attempt) * 2 * time.Second
            log.Printf("Waiting %v before retry...", waitTime)
            time.Sleep(waitTime)
        }
    }
    
    return fmt.Errorf("failed to connect after %d attempts", maxAttempts)
}
```

## SimConnect Exceptions

### Exception Types

The most common SimConnect exceptions:

| Exception | Value | Description | Common Causes |
|-----------|-------|-------------|---------------|
| `SIMCONNECT_EXCEPTION_NONE` | 0 | No error | Success |
| `SIMCONNECT_EXCEPTION_ERROR` | 1 | General error | Various issues |
| `SIMCONNECT_EXCEPTION_SIZE_MISMATCH` | 2 | Data size mismatch | Struct size incorrect |
| `SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID` | 3 | Unknown ID | Invalid request/definition ID |
| `SIMCONNECT_EXCEPTION_UNOPENED` | 4 | Connection not open | Connection lost |
| `SIMCONNECT_EXCEPTION_VERSION_MISMATCH` | 5 | Version mismatch | SDK version incompatible |
| `SIMCONNECT_EXCEPTION_TOO_MANY_GROUPS` | 6 | Too many groups | Group limit exceeded |
| `SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED` | 7 | Unknown name | Invalid variable/event name |
| `SIMCONNECT_EXCEPTION_TOO_MANY_EVENT_NAMES` | 8 | Too many events | Event limit exceeded |
| `SIMCONNECT_EXCEPTION_EVENT_ID_DUPLICATE` | 9 | Duplicate event ID | Event already mapped |
| `SIMCONNECT_EXCEPTION_TOO_MANY_MAPS` | 10 | Too many data maps | Map limit exceeded |
| `SIMCONNECT_EXCEPTION_TOO_MANY_OBJECTS` | 11 | Too many objects | Object limit exceeded |
| `SIMCONNECT_EXCEPTION_TOO_MANY_REQUESTS` | 12 | Too many requests | Request limit exceeded |
| `SIMCONNECT_EXCEPTION_WEATHER_INVALID_PORT` | 13 | Invalid weather port | Weather system error |
| `SIMCONNECT_EXCEPTION_WEATHER_INVALID_METAR` | 14 | Invalid METAR | Weather data format error |
| `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_GET_OBSERVATION` | 15 | Weather observation failed | Weather system unavailable |
| `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_CREATE_STATION` | 16 | Weather station creation failed | Weather system error |
| `SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_REMOVE_STATION` | 17 | Weather station removal failed | Weather system error |

### Exception Handling Patterns

```go
func handleException(exc *types.SIMCONNECT_RECV_EXCEPTION) error {
    switch exc.DwException {
    case types.SIMCONNECT_EXCEPTION_NONE:
        return nil // No error
        
    case types.SIMCONNECT_EXCEPTION_SIZE_MISMATCH:
        return fmt.Errorf("data structure size mismatch for request %d at index %d", 
            exc.DwSendID, exc.DwIndex)
            
    case types.SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID:
        return fmt.Errorf("unrecognized ID %d", exc.DwSendID)
        
    case types.SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED:
        return fmt.Errorf("unrecognized variable or event name for request %d", exc.DwSendID)
        
    case types.SIMCONNECT_EXCEPTION_UNOPENED:
        return fmt.Errorf("connection is not open")
        
    default:
        return fmt.Errorf("SimConnect exception %d for request %d", 
            exc.DwException, exc.DwSendID)
    }
}
```

## Message Processing Errors

### Handling Processing Errors

```go
func processMessages(sc *client.Engine) {
    messageStream := sc.Stream()
    
    for msg := range messageStream {
        // Check for parsing errors
        if msg.Error != nil {
            log.Printf("Message parsing error: %v", msg.Error)
            continue
        }
        
        // Handle different message types
        switch {
        case msg.IsException():
            if exc, ok := msg.GetException(); ok {
                if err := handleException(exc); err != nil {
                    log.Printf("SimConnect exception: %v", err)
                }
            }
            
        case msg.IsSimObjectData():
            if err := processSimObjectData(msg); err != nil {
                log.Printf("Failed to process sim object data: %v", err)
            }
            
        case msg.IsEvent():
            if err := processEvent(msg); err != nil {
                log.Printf("Failed to process event: %v", err)
            }
        }
    }
}
```

### Buffer Overflow Protection

The library includes protection against buffer overflow in message processing:

```go
// The library automatically handles buffer size validation
// and logs warnings when the message queue is full
if msg.Error != nil {
    if strings.Contains(msg.Error.Error(), "queue is full") {
        log.Printf("Warning: Message queue overflow - consider increasing buffer size or processing messages faster")
    }
}
```

## Best Practices

### 1. Always Check for Errors

```go
// ❌ Bad: Ignoring errors
sc.Connect()
sc.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)

// ✅ Good: Checking all errors
if err := sc.Connect(); err != nil {
    return fmt.Errorf("connection failed: %w", err)
}

if err := sc.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0); err != nil {
    return fmt.Errorf("failed to add data definition: %w", err)
}
```

### 2. Use Context for Cancellation

```go
func monitorAircraft(ctx context.Context, sc *client.Engine) error {
    messageStream := sc.Stream()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
            
        case msg := <-messageStream:
            if msg.Error != nil {
                log.Printf("Message error: %v", msg.Error)
                continue
            }
            // Process message
        }
    }
}
```

### 3. Implement Graceful Shutdown

```go
func main() {
    sc := client.New("My App")
    if sc == nil {
        log.Fatal("Failed to create client")
    }
    
    // Setup graceful shutdown
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        log.Println("Shutting down...")
        sc.Disconnect()
        os.Exit(0)
    }()
    
    if err := sc.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    
    // Main processing loop
    processMessages(sc)
}
```

## Error Recovery Strategies

### 1. Connection Recovery

```go
func maintainConnection(sc *client.Engine) {
    for {
        if err := sc.Connect(); err != nil {
            log.Printf("Connection lost: %v", err)
            log.Println("Attempting to reconnect in 5 seconds...")
            time.Sleep(5 * time.Second)
            continue
        }
        
        log.Println("Connection established")
        break
    }
}
```

### 2. Data Definition Recovery

```go
func setupDataDefinitions(sc *client.Engine) error {
    definitions := []struct {
        id       uint32
        variable string
        unit     string
        dataType types.SIMCONNECT_DATATYPE
        index    uint32
    }{
        {1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0},
        {1, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 1},
        // Add more definitions...
    }
    
    for _, def := range definitions {
        if err := sc.AddToDataDefinition(def.id, def.variable, def.unit, 
            def.dataType, 0.0, def.index); err != nil {
            log.Printf("Failed to add definition for %s: %v", def.variable, err)
            // Continue with other definitions instead of failing completely
        }
    }
    
    return nil
}
```

### 3. Message Processing Recovery

```go
func robustMessageProcessing(sc *client.Engine) {
    messageStream := sc.Stream()
    errorCount := 0
    maxErrors := 10
    
    for msg := range messageStream {
        if msg.Error != nil {
            errorCount++
            log.Printf("Message error (%d/%d): %v", errorCount, maxErrors, msg.Error)
            
            if errorCount >= maxErrors {
                log.Printf("Too many consecutive errors, attempting reconnection...")
                sc.Disconnect()
                maintainConnection(sc)
                errorCount = 0
            }
            continue
        }
        
        // Reset error count on successful message
        errorCount = 0
        
        // Process message...
    }
}
```

Following these error handling practices will make your SimConnect applications more robust and reliable in production environments.
