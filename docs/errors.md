# Error Handling & Debugging

This document covers error handling strategies, common issues, and debugging techniques when working with SimConnect.

## Table of Contents

• [Error Types](#error-types)  
• [Exception Handling](#exception-handling)  
• [Common Issues](#common-issues)  
• [Debugging Techniques](#debugging-techniques)  
• [Best Practices](#best-practices)

## Error Types

### Connection Errors

These occur during connection establishment or when the connection is lost.

```go
simClient := client.New("MyApp")
if simClient == nil {
    log.Fatal("Failed to create SimConnect client - check if SimConnect.dll is available")
}

if err := simClient.Connect(); err != nil {
    // Common causes:
    // - Simulator not running
    // - SimConnect not available
    // - Access permissions
    log.Fatalf("Connection failed: %v", err)
}
```

### API Call Errors

These occur when SimConnect API calls fail.

```go
err := client.AddToDataDefinition(1, "INVALID_SIMVAR", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
if err != nil {
    // Handle invalid simulation variable names, units, or data types
    fmt.Printf("Failed to add data definition: %v\n", err)
}
```

### Message Parsing Errors

These occur when received messages cannot be parsed correctly.

```go
for message := range client.Stream() {
    if message.Error != nil {
        // Handle parsing errors
        fmt.Printf("Message parsing error: %v\n", message.Error)
        continue
    }
    // Process valid message
}
```

## Exception Handling

SimConnect reports errors through exception messages. Always monitor for these:

### Exception Types

```go
const (
    SIMCONNECT_EXCEPTION_ERROR                    = 0x80000001
    SIMCONNECT_EXCEPTION_SIZE_MISMATCH           = 0x80000002
    SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID         = 0x80000003
    SIMCONNECT_EXCEPTION_UNOPENED               = 0x80000004
    SIMCONNECT_EXCEPTION_VERSION_MISMATCH       = 0x80000005
    SIMCONNECT_EXCEPTION_TOO_MANY_GROUPS        = 0x80000006
    SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED      = 0x80000007
    SIMCONNECT_EXCEPTION_TOO_MANY_EVENT_NAMES   = 0x80000008
    SIMCONNECT_EXCEPTION_EVENT_ID_DUPLICATE     = 0x80000009
    SIMCONNECT_EXCEPTION_TOO_MANY_MAPS          = 0x8000000A
    SIMCONNECT_EXCEPTION_TOO_MANY_OBJECTS       = 0x8000000B
    SIMCONNECT_EXCEPTION_TOO_MANY_REQUESTS      = 0x8000000C
    SIMCONNECT_EXCEPTION_WEATHER_INVALID_PORT   = 0x8000000D
    SIMCONNECT_EXCEPTION_WEATHER_INVALID_METAR  = 0x8000000E
    SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_GET_OBSERVATION = 0x8000000F
    SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_CREATE_STATION  = 0x80000010
    SIMCONNECT_EXCEPTION_WEATHER_UNABLE_TO_REMOVE_STATION  = 0x80000011
    SIMCONNECT_EXCEPTION_INVALID_DATA_TYPE      = 0x80000012
    SIMCONNECT_EXCEPTION_INVALID_DATA_SIZE      = 0x80000013
    SIMCONNECT_EXCEPTION_DATA_ERROR             = 0x80000014
    SIMCONNECT_EXCEPTION_INVALID_ARRAY          = 0x80000015
    SIMCONNECT_EXCEPTION_CREATE_OBJECT_FAILED   = 0x80000016
    SIMCONNECT_EXCEPTION_LOAD_FLIGHTPLAN_FAILED = 0x80000017
    SIMCONNECT_EXCEPTION_OPERATION_INVALID_FOR_OBJECT_TYPE = 0x80000018
    SIMCONNECT_EXCEPTION_ILLEGAL_OPERATION      = 0x80000019
    SIMCONNECT_EXCEPTION_ALREADY_SUBSCRIBED     = 0x8000001A
    SIMCONNECT_EXCEPTION_INVALID_ENUM           = 0x8000001B
    SIMCONNECT_EXCEPTION_DEFINITION_ERROR       = 0x8000001C
    SIMCONNECT_EXCEPTION_DUPLICATE_ID           = 0x8000001D
    SIMCONNECT_EXCEPTION_DATUM_ID               = 0x8000001E
    SIMCONNECT_EXCEPTION_OUT_OF_BOUNDS          = 0x8000001F
    SIMCONNECT_EXCEPTION_ALREADY_CREATED        = 0x80000020
    SIMCONNECT_EXCEPTION_OBJECT_OUTSIDE_REALITY_BUBBLE = 0x80000021
    SIMCONNECT_EXCEPTION_OBJECT_CONTAINER       = 0x80000022
    SIMCONNECT_EXCEPTION_OBJECT_AI              = 0x80000023
    SIMCONNECT_EXCEPTION_OBJECT_ATC             = 0x80000024
    SIMCONNECT_EXCEPTION_OBJECT_SCHEDULE        = 0x80000025
)
```

### Exception Handler

```go
func handleException(message client.ParsedMessage) {
    exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION)
    if !ok {
        return
    }

    switch exception.DwException {
    case 0x80000001: // SIMCONNECT_EXCEPTION_ERROR
        fmt.Printf("General SimConnect error for packet %d\n", exception.DwSendID)
        
    case 0x80000002: // SIMCONNECT_EXCEPTION_SIZE_MISMATCH
        fmt.Printf("Data size mismatch for packet %d - check data definitions\n", exception.DwSendID)
        
    case 0x80000003: // SIMCONNECT_EXCEPTION_UNRECOGNIZED_ID
        fmt.Printf("Unrecognized ID for packet %d - check request/definition IDs\n", exception.DwSendID)
        
    case 0x80000004: // SIMCONNECT_EXCEPTION_UNOPENED
        fmt.Printf("SimConnect not opened for packet %d\n", exception.DwSendID)
        
    case 0x80000007: // SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED
        fmt.Printf("Unrecognized name for packet %d - check SimVar/event names\n", exception.DwSendID)
        
    case 0x80000012: // SIMCONNECT_EXCEPTION_INVALID_DATA_TYPE
        fmt.Printf("Invalid data type for packet %d\n", exception.DwSendID)
        
    case 0x80000013: // SIMCONNECT_EXCEPTION_INVALID_DATA_SIZE
        fmt.Printf("Invalid data size for packet %d\n", exception.DwSendID)
        
    case 0x8000001D: // SIMCONNECT_EXCEPTION_DUPLICATE_ID
        fmt.Printf("Duplicate ID for packet %d - ID already in use\n", exception.DwSendID)
        
    default:
        fmt.Printf("Unknown exception 0x%08X for packet %d (index: %d)\n", 
            exception.DwException, exception.DwSendID, exception.DwIndex)
    }
}
```

## Common Issues

### 1. Simulator Not Running

**Problem**: Connection fails because the simulator is not running or SimConnect is not available.

**Solution**:
```go
simClient := client.New("MyApp")
if simClient == nil {
    fmt.Println("Error: Could not initialize SimConnect client")
    fmt.Println("Check:")
    fmt.Println("- Is the simulator (MSFS) running?")
    fmt.Println("- Is SimConnect.dll available at the expected path?")
    fmt.Println("- Are you running on Windows?")
    return
}
```

### 2. Invalid Simulation Variables

**Problem**: Using incorrect or deprecated simulation variable names.

**Solution**:
```go
// Bad - incorrect variable name
err := client.AddToDataDefinition(1, "AIRPLANE_ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)

// Good - correct variable name
err := client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
if err != nil {
    fmt.Printf("Failed to add data definition: %v\n", err)
    fmt.Println("Check simulation variable name in MSFS documentation")
}
```

### 3. Data Type Mismatches

**Problem**: Using wrong data types for simulation variables.

**Solution**:
```go
// Check MSFS documentation for correct data types
// Some variables are integers, others are floats
err := client.AddToDataDefinition(1, "COM ACTIVE FREQUENCY:1", "MHz", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
if err != nil {
    fmt.Printf("Data type error: %v\n", err)
    // Try different data type based on documentation
}
```

### 4. Memory Access Violations

**Problem**: Incorrect data parsing leading to memory access errors.

**Solution**:
```go
import "unsafe"

func safeDataParsing(message client.ParsedMessage) {
    data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA)
    if !ok {
        return
    }

    // Calculate expected data size
    expectedSize := int(unsafe.Sizeof(float64(0))) * 3 // 3 float64 values
    headerSize := int(unsafe.Sizeof(*data))
    
    // Validate sufficient data
    if len(message.RawData) < headerSize+expectedSize {
        fmt.Printf("Insufficient data: got %d bytes, expected %d\n", 
            len(message.RawData), headerSize+expectedSize)
        return
    }

    // Safe to parse data
    dataPtr := unsafe.Pointer(&message.RawData[headerSize])
    // ... parse data safely
}
```

### 5. ID Conflicts

**Problem**: Using duplicate IDs for requests, definitions, or events.

**Solution**:
```go
type IDManager struct {
    nextRequestID    int
    nextDefinitionID int
    nextEventID      int
    usedIDs          map[int]bool
}

func NewIDManager() *IDManager {
    return &IDManager{
        nextRequestID:    1,
        nextDefinitionID: 1,
        nextEventID:      1,
        usedIDs:         make(map[int]bool),
    }
}

func (idm *IDManager) GetRequestID() int {
    id := idm.nextRequestID
    idm.nextRequestID++
    idm.usedIDs[id] = true
    return id
}
```

## Debugging Techniques

### 1. Enable Verbose Logging

```go
type DebugClient struct {
    *client.Engine
    logger *log.Logger
}

func NewDebugClient(name string) *DebugClient {
    return &DebugClient{
        Engine: client.New(name),
        logger: log.New(os.Stdout, "[SimConnect] ", log.LstdFlags),
    }
}

func (dc *DebugClient) ProcessMessages() {
    for message := range dc.Stream() {
        dc.logger.Printf("Received message type: %d", message.MessageType)
        
        if message.Error != nil {
            dc.logger.Printf("Error: %v", message.Error)
            continue
        }

        switch {
        case message.IsSimObjectData():
            if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
                dc.logger.Printf("Data - Request: %d, Object: %d, Size: %d bytes", 
                    data.DwRequestID, data.DwObjectID, len(message.RawData))
            }
        case message.IsEvent():
            if event, ok := message.Data.(*types.SIMCONNECT_RECV_EVENT); ok {
                dc.logger.Printf("Event - Group: %d, Event: %d, Data: %d", 
                    event.UGroupID, event.UEventID, event.DwData)
            }
        case message.IsException():
            dc.handleException(message)
        }
    }
}

func (dc *DebugClient) handleException(message client.ParsedMessage) {
    if exception, ok := message.Data.(*types.SIMCONNECT_RECV_EXCEPTION); ok {
        dc.logger.Printf("EXCEPTION: 0x%08X for packet %d", 
            exception.DwException, exception.DwSendID)
    }
}
```

### 2. Raw Data Inspection

```go
func inspectRawData(message client.ParsedMessage) {
    fmt.Printf("Raw data (%d bytes): ", len(message.RawData))
    for i, b := range message.RawData {
        if i > 0 && i%16 == 0 {
            fmt.Println()
        }
        fmt.Printf("%02X ", b)
    }
    fmt.Println()
}
```

### 3. Connection Health Monitoring

```go
type ConnectionMonitor struct {
    client          *client.Engine
    lastMessageTime time.Time
    isConnected     bool
    timeout         time.Duration
}

func NewConnectionMonitor(client *client.Engine) *ConnectionMonitor {
    return &ConnectionMonitor{
        client:  client,
        timeout: 30 * time.Second, // 30 second timeout
    }
}

func (cm *ConnectionMonitor) Monitor() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            if cm.isConnected && time.Since(cm.lastMessageTime) > cm.timeout {
                fmt.Println("WARNING: No messages received for", cm.timeout)
                fmt.Println("Connection may be lost")
            }
        }
    }()

    for message := range cm.client.Stream() {
        cm.lastMessageTime = time.Now()
        
        if message.IsOpen() {
            cm.isConnected = true
            fmt.Println("Connection established")
        } else if message.IsQuit() {
            cm.isConnected = false
            fmt.Println("Connection lost")
            return
        }
        
        // Process other messages...
    }
}
```

## Best Practices

### 1. Graceful Error Recovery

```go
func robustSimConnectClient() {
    maxRetries := 3
    retryDelay := 5 * time.Second

    for attempt := 1; attempt <= maxRetries; attempt++ {
        simClient := client.New("MyApp")
        if simClient == nil {
            fmt.Printf("Attempt %d: Failed to create client\n", attempt)
            time.Sleep(retryDelay)
            continue
        }

        if err := simClient.Connect(); err != nil {
            fmt.Printf("Attempt %d: Connection failed: %v\n", attempt, err)
            time.Sleep(retryDelay)
            continue
        }

        // Success - start processing
        defer simClient.Disconnect()
        processMessages(simClient)
        return
    }

    log.Fatal("Failed to connect after", maxRetries, "attempts")
}
```

### 2. Comprehensive Error Handling

```go
func processMessages(client *client.Engine) {
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("Recovered from panic: %v\n", r)
        }
    }()

    for message := range client.Stream() {
        // Always check for errors first
        if message.Error != nil {
            log.Printf("Message error: %v", message.Error)
            continue
        }

        // Handle connection state
        if message.IsQuit() {
            log.Println("SimConnect quit - exiting")
            return
        }

        // Handle exceptions
        if message.IsException() {
            handleException(message)
            continue
        }

        // Process valid messages
        switch message.MessageType {
        case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
            if err := processSimObjectData(message); err != nil {
                log.Printf("Failed to process sim object data: %v", err)
            }
        case types.SIMCONNECT_RECV_ID_EVENT:
            if err := processEvent(message); err != nil {
                log.Printf("Failed to process event: %v", err)
            }
        }
    }
}
```

### 3. Resource Management

```go
type SimConnectManager struct {
    client     *client.Engine
    activeIDs  map[int]string
    cleanup    []func()
    mu         sync.Mutex
}

func (scm *SimConnectManager) AddDataDefinition(id int, name string, units string, dataType types.SIMCONNECT_DATATYPE) error {
    scm.mu.Lock()
    defer scm.mu.Unlock()

    if _, exists := scm.activeIDs[id]; exists {
        return fmt.Errorf("ID %d already in use", id)
    }

    err := scm.client.AddToDataDefinition(id, name, units, dataType, 0.0, 0)
    if err != nil {
        return err
    }

    scm.activeIDs[id] = fmt.Sprintf("DataDef:%s", name)
    scm.cleanup = append(scm.cleanup, func() {
        scm.client.ClearDataDefinition(id)
    })

    return nil
}

func (scm *SimConnectManager) Cleanup() {
    scm.mu.Lock()
    defer scm.mu.Unlock()

    for _, cleanupFunc := range scm.cleanup {
        cleanupFunc()
    }
    scm.cleanup = nil
    scm.activeIDs = make(map[int]string)
}
```

Remember: Always test your error handling paths and have recovery strategies for common failure scenarios!
