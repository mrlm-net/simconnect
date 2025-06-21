# Error Handling

Comprehensive error handling, exception management, and recovery strategies for SimConnect applications.

## Exception Types

### Common SimConnect Exceptions

```go
const (
    EXCEPTION_NONE                 = 0
    EXCEPTION_ERROR               = 1
    EXCEPTION_SIZE_MISMATCH       = 2
    EXCEPTION_UNRECOGNIZED_ID     = 3
    EXCEPTION_UNOPENED            = 4
    EXCEPTION_VERSION_MISMATCH    = 5
    EXCEPTION_TOO_MANY_GROUPS     = 6
    EXCEPTION_NAME_UNRECOGNIZED   = 7
    EXCEPTION_TOO_MANY_EVENT_NAMES = 8
    EXCEPTION_EVENT_ID_DUPLICATE  = 9
    EXCEPTION_TOO_MANY_MAPS       = 10
    EXCEPTION_TOO_MANY_OBJECTS    = 11
    EXCEPTION_TOO_MANY_REQUESTS   = 12
    EXCEPTION_WEATHER_INVALID_PORT = 13
    EXCEPTION_WEATHER_INVALID_METAR = 14
    EXCEPTION_WEATHER_UNABLE_TO_GET_OBSERVATION = 15
    EXCEPTION_WEATHER_UNABLE_TO_CREATE_STATION = 16
    EXCEPTION_WEATHER_UNABLE_TO_REMOVE_STATION = 17
    EXCEPTION_INVALID_DATA_TYPE   = 18
    EXCEPTION_INVALID_DATA_SIZE   = 19
    EXCEPTION_DATA_ERROR          = 20
    EXCEPTION_INVALID_ARRAY       = 21
    EXCEPTION_CREATE_OBJECT_FAILED = 22
    EXCEPTION_LOAD_FLIGHTPLAN_FAILED = 23
    EXCEPTION_OPERATION_INVALID_FOR_OBJECT_TYPE = 24
    EXCEPTION_ILLEGAL_OPERATION   = 25
    EXCEPTION_ALREADY_SUBSCRIBED  = 26
    EXCEPTION_INVALID_ENUM        = 27
    EXCEPTION_DEFINITION_ERROR    = 28
    EXCEPTION_DUPLICATE_ID        = 29
    EXCEPTION_DATUM_ID            = 30
    EXCEPTION_OUT_OF_BOUNDS       = 31
    EXCEPTION_ALREADY_CREATED     = 32
    EXCEPTION_OBJECT_OUTSIDE_REALITY_BUBBLE = 33
    EXCEPTION_OBJECT_CONTAINER    = 34
    EXCEPTION_OBJECT_AI           = 35
    EXCEPTION_OBJECT_ATC          = 36
    EXCEPTION_OBJECT_SCHEDULE     = 37
)
```

## Exception Handling Patterns

### Basic Exception Processing

```go
case msg.IsException():
    if exception, ok := msg.GetException(); ok {
        handleException(exception)
    }

func handleException(ex *types.Exception) {
    fmt.Printf("SimConnect Exception: %s (Code: %d, Send ID: %d, Index: %d)\n", 
        getExceptionName(ex.ExceptionCode), 
        ex.ExceptionCode, 
        ex.SendID, 
        ex.Index)
}
```

### Exception Recovery Strategies

```go
func handleExceptionWithRecovery(client *client.Engine, ex *types.Exception) {
    switch ex.ExceptionCode {
    case types.EXCEPTION_NAME_UNRECOGNIZED:
        fmt.Printf("Unknown SimVar or event name in request %d\n", ex.SendID)
        // Don't retry - fix the name
        
    case types.EXCEPTION_UNOPENED:
        fmt.Println("Connection lost, attempting reconnect...")
        if err := reconnectWithBackoff(client); err != nil {
            log.Fatal("Failed to reconnect:", err)
        }
        
    case types.EXCEPTION_SIZE_MISMATCH:
        fmt.Printf("Data size mismatch for definition %d\n", ex.SendID)
        // Review data structure alignment
        
    case types.EXCEPTION_TOO_MANY_REQUESTS:
        fmt.Println("Too many active requests, throttling...")
        time.Sleep(1 * time.Second)
        
    default:
        fmt.Printf("Unhandled exception: %d\n", ex.ExceptionCode)
    }
}
```

## Connection Error Handling

### Reconnection with Backoff

```go
func reconnectWithBackoff(client *client.Engine) error {
    backoff := time.Second
    maxBackoff := 30 * time.Second
    maxRetries := 5
    
    for i := 0; i < maxRetries; i++ {
        fmt.Printf("Reconnection attempt %d/%d...\n", i+1, maxRetries)
        
        if err := client.Connect(); err == nil {
            fmt.Println("Reconnected successfully")
            return nil
        }
        
        if i < maxRetries-1 {
            fmt.Printf("Reconnection failed, waiting %v...\n", backoff)
            time.Sleep(backoff)
            backoff *= 2
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }
    
    return fmt.Errorf("failed to reconnect after %d attempts", maxRetries)
}
```

### Connection State Monitoring

```go
type ConnectionState struct {
    Connected    bool
    LastPing     time.Time
    ReconnectCount int
}

func monitorConnection(client *client.Engine, state *ConnectionState) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        if time.Since(state.LastPing) > 10*time.Second {
            fmt.Println("Connection appears stale, checking...")
            if !pingSimConnect(client) {
                state.Connected = false
                if err := reconnectWithBackoff(client); err != nil {
                    log.Printf("Failed to restore connection: %v", err)
                } else {
                    state.Connected = true
                    state.ReconnectCount++
                }
            }
        }
    }
}
```

## Data Validation

### Input Validation

```go
func validateAltitude(altitude float64) error {
    if altitude < -1000 || altitude > 100000 {
        return fmt.Errorf("altitude %f outside valid range [-1000, 100000]", altitude)
    }
    return nil
}

func validateLatLon(lat, lon float64) error {
    if lat < -90 || lat > 90 {
        return fmt.Errorf("latitude %f outside valid range [-90, 90]", lat)
    }
    if lon < -180 || lon > 180 {
        return fmt.Errorf("longitude %f outside valid range [-180, 180]", lon)
    }
    return nil
}
```

### Data Structure Validation

```go
func validateAircraftData(data *AircraftData) []error {
    var errors []error
    
    if err := validateAltitude(data.Altitude); err != nil {
        errors = append(errors, err)
    }
    
    if err := validateLatLon(data.Latitude, data.Longitude); err != nil {
        errors = append(errors, err)
    }
    
    if data.GroundSpeed < 0 || data.GroundSpeed > 1000 {
        errors = append(errors, fmt.Errorf("invalid ground speed: %f", data.GroundSpeed))
    }
    
    return errors
}
```

## Error Logging

### Structured Logging

```go
import "log/slog"

func setupLogging() *slog.Logger {
    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
}

func logException(logger *slog.Logger, ex *types.Exception) {
    logger.Error("SimConnect exception",
        "code", ex.ExceptionCode,
        "name", getExceptionName(ex.ExceptionCode),
        "sendID", ex.SendID,
        "index", ex.Index,
    )
}
```

### Error Context

```go
type ErrorContext struct {
    Operation   string
    RequestID   uint32
    Timestamp   time.Time
    RetryCount  int
}

func (ec *ErrorContext) LogError(err error) {
    log.Printf("[%s] Error in %s (ID: %d, Retry: %d): %v", 
        ec.Timestamp.Format(time.RFC3339),
        ec.Operation,
        ec.RequestID, 
        ec.RetryCount,
        err)
}
```

## Best Practices

### Graceful Degradation

```go
func requestDataSafely(client *client.Engine, reqID, defID uint32) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic in requestDataSafely: %v", r)
        }
    }()
    
    if client == nil {
        log.Println("Client is nil, skipping data request")
        return
    }
    
    client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_ONCE)
}
```

### Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    failureCount    int
    lastFailureTime time.Time
    state          string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Since(cb.lastFailureTime) > 30*time.Second {
            cb.state = "half-open"
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }
    
    err := fn()
    if err != nil {
        cb.failureCount++
        cb.lastFailureTime = time.Now()
        
        if cb.failureCount >= 3 {
            cb.state = "open"
        }
        return err
    }
    
    cb.failureCount = 0
    cb.state = "closed"
    return nil
}
```
