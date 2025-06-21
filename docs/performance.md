# Performance

Optimization strategies, best practices, and performance considerations for SimConnect applications.

## Connection Optimization

### Connection Pool Management

```go
// ❌ Creating multiple connections
client1 := client.New("App1")
client2 := client.New("App2")

// ✅ Single connection with proper resource sharing
client := client.New("MyApp")
// Share client across goroutines with proper synchronization
```

### Efficient Stream Buffer Sizing

```go
// Default buffer size is 100 - adjust based on message volume
const CUSTOM_STREAM_BUFFER_SIZE = 500

// For high-frequency data applications
const HIGH_FREQUENCY_BUFFER = 1000

// For low-frequency control applications  
const LOW_FREQUENCY_BUFFER = 50
```

## Data Request Optimization

### Batch Data Definitions

```go
// ❌ Multiple separate definitions
client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.DATATYPE_FLOAT64)
client.RequestDataOnSimObject(1, 1, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

client.AddToDataDefinition(2, "PLANE LATITUDE", "radians", types.DATATYPE_FLOAT64)
client.RequestDataOnSimObject(2, 2, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

// ✅ Single definition with multiple variables
client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.DATATYPE_FLOAT64)
client.AddToDataDefinition(1, "PLANE LATITUDE", "radians", types.DATATYPE_FLOAT64)
client.AddToDataDefinition(1, "PLANE LONGITUDE", "radians", types.DATATYPE_FLOAT64)
client.RequestDataOnSimObject(1, 1, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)
```

### Appropriate Update Frequencies

```go
// High-frequency data (flight controls, instruments)
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

// Medium-frequency data (navigation, engine parameters)  
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_VISUAL_FRAME)

// Low-frequency data (fuel, configuration)
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SECOND)

// Static data (aircraft info)
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_ONCE)
```

## Memory Management

### Efficient Data Structures

```go
// ❌ Using strings for frequently updated data
type IneffientData struct {
    Altitude string
    Speed    string
    Heading  string
}

// ✅ Using appropriate numeric types
type EfficientData struct {
    Altitude float64
    Speed    float64  
    Heading  float64
}
```

### Pool Reusable Objects

```go
var dataPool = sync.Pool{
    New: func() interface{} {
        return &AircraftData{}
    },
}

func processMessage(msg ParsedMessage) {
    if msg.IsSimObjectData() {
        data := dataPool.Get().(*AircraftData)
        defer dataPool.Put(data)
        
        // Use data...
    }
}
```

## Goroutine Management

### Optimal Goroutine Architecture

```go
func main() {
    client := client.New("MyApp")
    defer client.Disconnect()
    
    // Single message processing goroutine
    go processMessages(client)
    
    // Single background task goroutine  
    go backgroundTasks(client)
    
    // Avoid creating goroutines per message
    // ❌ go handleMessage(msg) // Don't do this
}
```

### Channel Sizing

```go
// Size channels based on expected load
dataChan := make(chan AircraftData, 100)      // High frequency
eventChan := make(chan Event, 10)             // Low frequency  
errorChan := make(chan error, 50)             // Medium frequency
```

## Network Optimization

### Event Batching

```go
type EventBatch struct {
    events []Event
    timer  *time.Timer
}

func (eb *EventBatch) AddEvent(event Event) {
    eb.events = append(eb.events, event)
    
    if len(eb.events) >= 10 { // Batch size threshold
        eb.Flush()
    } else if eb.timer == nil {
        eb.timer = time.AfterFunc(100*time.Millisecond, eb.Flush) // Time threshold
    }
}

func (eb *EventBatch) Flush() {
    if eb.timer != nil {
        eb.timer.Stop()
        eb.timer = nil
    }
    
    // Send all events at once
    for _, event := range eb.events {
        client.TransmitClientEvent(event.ID, event.Data)
    }
    eb.events = eb.events[:0] // Reset slice
}
```

### Reduce Unnecessary Requests

```go
// ❌ Requesting data when not needed
if !aircraftInFlight {
    // Still requesting flight data
    client.RequestDataOnSimObject(reqID, flightDefID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)
}

// ✅ Conditional data requests
if aircraftInFlight {
    client.RequestDataOnSimObject(reqID, flightDefID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)
} else {
    client.RequestDataOnSimObject(reqID, groundDefID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SECOND)
}
```

## CPU Optimization

### Minimize Processing in Message Loop

```go
// ❌ Heavy processing in message loop
for msg := range client.Stream() {
    if msg.IsSimObjectData() {
        data := parseData(msg)
        processComplexCalculations(data)  // Blocks message loop
        updateDatabase(data)              // Slow I/O operation
    }
}

// ✅ Offload processing to worker goroutines  
dataChan := make(chan SimObjectData, 100)

// Message loop - fast processing only
for msg := range client.Stream() {
    if msg.IsSimObjectData() {
        data := parseData(msg)
        select {
        case dataChan <- data:
        default:
            // Channel full - drop message to avoid blocking
        }
    }
}

// Separate worker for heavy processing
go func() {
    for data := range dataChan {
        processComplexCalculations(data)
        updateDatabase(data)
    }
}()
```

### Efficient Data Conversion

```go
// ❌ Repeated conversions
func processAircraftData(data *AircraftData) {
    latDeg := data.Latitude * 180.0 / math.Pi    // Convert every time
    lonDeg := data.Longitude * 180.0 / math.Pi   // Convert every time
    hdgDeg := data.Heading * 180.0 / math.Pi     // Convert every time
}

// ✅ Pre-computed conversion factor
const RAD_TO_DEG = 180.0 / math.Pi

func processAircraftData(data *AircraftData) {
    latDeg := data.Latitude * RAD_TO_DEG
    lonDeg := data.Longitude * RAD_TO_DEG  
    hdgDeg := data.Heading * RAD_TO_DEG
}
```

## Profiling and Monitoring

### Basic Performance Monitoring

```go
type PerformanceMonitor struct {
    messageCount    uint64
    lastResetTime   time.Time
    processingTimes []time.Duration
}

func (pm *PerformanceMonitor) RecordMessage(processingTime time.Duration) {
    atomic.AddUint64(&pm.messageCount, 1)
    
    // Sample processing times (every 100th message)
    if pm.messageCount%100 == 0 {
        pm.processingTimes = append(pm.processingTimes, processingTime)
        if len(pm.processingTimes) > 1000 {
            pm.processingTimes = pm.processingTimes[100:] // Keep recent samples
        }
    }
}

func (pm *PerformanceMonitor) GetStats() (float64, time.Duration) {
    elapsed := time.Since(pm.lastResetTime)
    rate := float64(atomic.LoadUint64(&pm.messageCount)) / elapsed.Seconds()
    
    var avgProcessingTime time.Duration
    if len(pm.processingTimes) > 0 {
        var total time.Duration
        for _, t := range pm.processingTimes {
            total += t
        }
        avgProcessingTime = total / time.Duration(len(pm.processingTimes))
    }
    
    return rate, avgProcessingTime
}
```

### Memory Usage Tracking

```go
import "runtime"

func logMemoryUsage() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    log.Printf("Memory: Alloc=%d KB, TotalAlloc=%d KB, Sys=%d KB, NumGC=%d",
        m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC)
}

// Call periodically
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        logMemoryUsage()
    }
}()
```

## Performance Benchmarks

Target performance metrics for well-optimized applications:

| Metric | Good | Excellent |
|--------|------|-----------|
| Message Processing Rate | >1000 msg/sec | >5000 msg/sec |
| Memory Usage (steady state) | <50MB | <20MB |
| CPU Usage (average) | <10% | <5% |
| Message Processing Latency | <1ms | <0.1ms |
| Connection Recovery Time | <5s | <2s |
