# Advanced Usage Guide

This document covers advanced patterns, performance optimization, and complex use cases for the SimConnect Go wrapper.

## Table of Contents

• [Performance Optimization](#performance-optimization)  
• [Complex Data Structures](#complex-data-structures)  
• [Multi-threaded Applications](#multi-threaded-applications)  
• [Custom Message Processors](#custom-message-processors)  
• [Integration Patterns](#integration-patterns)  
• [Advanced Examples](#advanced-examples)

## Performance Optimization

### Efficient Data Definitions

Group related data together to minimize the number of requests and improve performance.

```go
// Instead of multiple separate definitions
// BAD:
client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
client.AddToDataDefinition(2, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
client.AddToDataDefinition(3, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)

// GOOD: Group into single definition
defineID := 1
client.AddToDataDefinition(defineID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
client.AddToDataDefinition(defineID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
client.AddToDataDefinition(defineID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
```

### Smart Update Frequencies

Use appropriate periods and epsilon values to reduce unnecessary updates.

```go
// High-frequency critical data (flight controls)
client.AddToDataDefinition(1, "ELEVATOR POSITION", "position", types.SIMCONNECT_DATATYPE_FLOAT64, 0.001, 0)
client.RequestDataOnSimObject(1, 1, 0, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)

// Medium-frequency navigation data
client.AddToDataDefinition(2, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 10.0, 0) // 10ft epsilon
client.RequestDataOnSimObject(2, 2, 0, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)

// Low-frequency system data
client.AddToDataDefinition(3, "FUEL TOTAL QUANTITY", "gallons", types.SIMCONNECT_DATATYPE_FLOAT64, 1.0, 0) // 1 gallon epsilon
client.RequestDataOnSimObject(3, 3, 0, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 5, 0) // Every 5 seconds
```

### Message Queue Management

```go
type HighPerformanceClient struct {
    client      *client.Engine
    workerPool  *WorkerPool
    messageRate *RateLimiter
}

type WorkerPool struct {
    workers    int
    jobQueue   chan client.ParsedMessage
    wg         sync.WaitGroup
    quit       chan bool
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        workers:  workers,
        jobQueue: make(chan client.ParsedMessage, 1000), // Buffered channel
        quit:     make(chan bool),
    }
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker()
    }
}

func (wp *WorkerPool) worker() {
    defer wp.wg.Done()
    for {
        select {
        case message := <-wp.jobQueue:
            wp.processMessage(message)
        case <-wp.quit:
            return
        }
    }
}

func (wp *WorkerPool) processMessage(message client.ParsedMessage) {
    // Fast message processing logic
    switch {
    case message.IsSimObjectData():
        wp.processDataFast(message)
    case message.IsEvent():
        wp.processEventFast(message)
    }
}
```

## Complex Data Structures

### Aircraft State Manager

```go
import (
    "sync"
    "time"
    "unsafe"
)

type AircraftState struct {
    // Flight parameters
    Position struct {
        Latitude  float64
        Longitude float64
        Altitude  float64
    }
    
    // Flight dynamics
    Attitude struct {
        Pitch float64
        Bank  float64
        Heading float64
    }
    
    // Engine parameters
    Engines []EngineState
    
    // Systems
    Systems struct {
        ElectricalMain bool
        HydraulicMain  bool
        FuelPumps      []bool
    }
    
    // Metadata
    Timestamp time.Time
    mu        sync.RWMutex
}

type EngineState struct {
    N1       float64
    EGT      float64
    FuelFlow float64
    Running  bool
}

func NewAircraftStateManager(client *client.Engine) *AircraftStateManager {
    asm := &AircraftStateManager{
        client: client,
        state:  &AircraftState{},
    }
    
    asm.setupDataDefinitions()
    return asm
}

type AircraftStateManager struct {
    client *client.Engine
    state  *AircraftState
}

func (asm *AircraftStateManager) setupDataDefinitions() {
    // Position definition
    posDefID := 1
    asm.client.AddToDataDefinition(posDefID, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    asm.client.AddToDataDefinition(posDefID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
    asm.client.AddToDataDefinition(posDefID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
    
    // Attitude definition
    attDefID := 2
    asm.client.AddToDataDefinition(attDefID, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    asm.client.AddToDataDefinition(attDefID, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
    asm.client.AddToDataDefinition(attDefID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
    
    // Start requests
    asm.client.RequestDataOnSimObject(1, posDefID, 0, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
    asm.client.RequestDataOnSimObject(2, attDefID, 0, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
}

func (asm *AircraftStateManager) UpdateFromMessage(message client.ParsedMessage) {
    if !message.IsSimObjectData() {
        return
    }
    
    data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA)
    if !ok {
        return
    }
    
    asm.state.mu.Lock()
    defer asm.state.mu.Unlock()
    
    headerSize := unsafe.Sizeof(*data)
    dataPtr := unsafe.Pointer(&message.RawData[headerSize])
    
    switch data.DwRequestID {
    case 1: // Position data
        posData := (*[3]float64)(dataPtr)
        asm.state.Position.Latitude = posData[0]
        asm.state.Position.Longitude = posData[1]
        asm.state.Position.Altitude = posData[2]
        
    case 2: // Attitude data
        attData := (*[3]float64)(dataPtr)
        asm.state.Attitude.Pitch = attData[0]
        asm.state.Attitude.Bank = attData[1]
        asm.state.Attitude.Heading = attData[2]
    }
    
    asm.state.Timestamp = time.Now()
}

func (asm *AircraftStateManager) GetState() *AircraftState {
    asm.state.mu.RLock()
    defer asm.state.mu.RUnlock()
    
    // Return a copy to avoid race conditions
    stateCopy := *asm.state
    return &stateCopy
}
```

## Multi-threaded Applications

### Thread-Safe Event Dispatcher

```go
type EventDispatcher struct {
    client    *client.Engine
    handlers  map[types.SIMCONNECT_RECV_ID][]MessageHandler
    mu        sync.RWMutex
    wg        sync.WaitGroup
    quit      chan bool
}

type MessageHandler func(client.ParsedMessage) error

func NewEventDispatcher(client *client.Engine) *EventDispatcher {
    return &EventDispatcher{
        client:   client,
        handlers: make(map[types.SIMCONNECT_RECV_ID][]MessageHandler),
        quit:     make(chan bool),
    }
}

func (ed *EventDispatcher) RegisterHandler(messageType types.SIMCONNECT_RECV_ID, handler MessageHandler) {
    ed.mu.Lock()
    defer ed.mu.Unlock()
    
    ed.handlers[messageType] = append(ed.handlers[messageType], handler)
}

func (ed *EventDispatcher) Start() {
    ed.wg.Add(1)
    go ed.messageLoop()
}

func (ed *EventDispatcher) Stop() {
    close(ed.quit)
    ed.wg.Wait()
}

func (ed *EventDispatcher) messageLoop() {
    defer ed.wg.Done()
    
    for {
        select {
        case message := <-ed.client.Stream():
            ed.dispatchMessage(message)
        case <-ed.quit:
            return
        }
    }
}

func (ed *EventDispatcher) dispatchMessage(message client.ParsedMessage) {
    ed.mu.RLock()
    handlers, exists := ed.handlers[message.MessageType]
    ed.mu.RUnlock()
    
    if !exists {
        return
    }
    
    // Execute handlers concurrently
    var wg sync.WaitGroup
    for _, handler := range handlers {
        wg.Add(1)
        go func(h MessageHandler) {
            defer wg.Done()
            if err := h(message); err != nil {
                log.Printf("Handler error: %v", err)
            }
        }(handler)
    }
    wg.Wait()
}
```

## Custom Message Processors

### Filtering and Transformation Pipeline

```go
type MessageProcessor interface {
    Process(message client.ParsedMessage) (client.ParsedMessage, bool)
}

type MessagePipeline struct {
    processors []MessageProcessor
}

func NewMessagePipeline() *MessagePipeline {
    return &MessagePipeline{}
}

func (mp *MessagePipeline) AddProcessor(processor MessageProcessor) {
    mp.processors = append(mp.processors, processor)
}

func (mp *MessagePipeline) Process(message client.ParsedMessage) (client.ParsedMessage, bool) {
    current := message
    for _, processor := range mp.processors {
        var shouldContinue bool
        current, shouldContinue = processor.Process(current)
        if !shouldContinue {
            return current, false
        }
    }
    return current, true
}

// Example processors

type ErrorFilterProcessor struct{}

func (efp *ErrorFilterProcessor) Process(message client.ParsedMessage) (client.ParsedMessage, bool) {
    if message.Error != nil {
        log.Printf("Filtered error message: %v", message.Error)
        return message, false // Don't continue processing
    }
    return message, true
}

type DataValidationProcessor struct{}

func (dvp *DataValidationProcessor) Process(message client.ParsedMessage) (client.ParsedMessage, bool) {
    if message.IsSimObjectData() {
        if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok {
            // Validate data size
            expectedSize := int(data.DwDefineCount * 8) // 8 bytes per element
            if len(message.RawData) < expectedSize {
                log.Printf("Invalid data size: expected %d, got %d", expectedSize, len(message.RawData))
                return message, false
            }
        }
    }
    return message, true
}

type TimestampProcessor struct{}

func (tp *TimestampProcessor) Process(message client.ParsedMessage) (client.ParsedMessage, bool) {
    // Add timestamp metadata (this would require extending ParsedMessage)
    // For demonstration purposes
    log.Printf("Processing message at %v", time.Now())
    return message, true
}
```

## Integration Patterns

### Plugin Architecture

```go
type SimConnectPlugin interface {
    Name() string
    Initialize(*client.Engine) error
    HandleMessage(client.ParsedMessage) error
    Cleanup() error
}

type PluginManager struct {
    client  *client.Engine
    plugins map[string]SimConnectPlugin
    active  map[string]bool
    mu      sync.RWMutex
}

func NewPluginManager(client *client.Engine) *PluginManager {
    return &PluginManager{
        client:  client,
        plugins: make(map[string]SimConnectPlugin),
        active:  make(map[string]bool),
    }
}

func (pm *PluginManager) RegisterPlugin(plugin SimConnectPlugin) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    name := plugin.Name()
    if _, exists := pm.plugins[name]; exists {
        return fmt.Errorf("plugin %s already registered", name)
    }
    
    if err := plugin.Initialize(pm.client); err != nil {
        return fmt.Errorf("failed to initialize plugin %s: %v", name, err)
    }
    
    pm.plugins[name] = plugin
    pm.active[name] = true
    return nil
}

func (pm *PluginManager) ProcessMessage(message client.ParsedMessage) {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    
    for name, plugin := range pm.plugins {
        if pm.active[name] {
            if err := plugin.HandleMessage(message); err != nil {
                log.Printf("Plugin %s error: %v", name, err)
            }
        }
    }
}

// Example plugin
type TelemetryPlugin struct {
    output chan TelemetryData
}

type TelemetryData struct {
    Timestamp time.Time
    Altitude  float64
    Airspeed  float64
}

func (tp *TelemetryPlugin) Name() string {
    return "telemetry"
}

func (tp *TelemetryPlugin) Initialize(client *client.Engine) error {
    tp.output = make(chan TelemetryData, 100)
    
    // Set up data definitions
    defineID := 100
    client.AddToDataDefinition(defineID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    client.AddToDataDefinition(defineID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
    client.RequestDataOnSimObject(100, defineID, 0, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
    
    return nil
}

func (tp *TelemetryPlugin) HandleMessage(message client.ParsedMessage) error {
    if message.IsSimObjectData() {
        if data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA); ok && data.DwRequestID == 100 {
            // Parse telemetry data
            telemetry := tp.parseData(message)
            select {
            case tp.output <- telemetry:
            default:
                // Channel full, drop message
            }
        }
    }
    return nil
}

func (tp *TelemetryPlugin) parseData(message client.ParsedMessage) TelemetryData {
    // Implementation details...
    return TelemetryData{
        Timestamp: time.Now(),
        // Parse altitude and airspeed from message.RawData
    }
}

func (tp *TelemetryPlugin) Cleanup() error {
    close(tp.output)
    return nil
}

func (tp *TelemetryPlugin) GetTelemetryChannel() <-chan TelemetryData {
    return tp.output
}
```

## Advanced Examples

### Real-time Flight Data Recorder

```go
type FlightDataRecorder struct {
    client      *client.Engine
    recording   bool
    output      *os.File
    encoder     *json.Encoder
    sampleRate  time.Duration
    lastSample  time.Time
    mu          sync.Mutex
}

type FlightDataRecord struct {
    Timestamp time.Time `json:"timestamp"`
    Position  struct {
        Latitude  float64 `json:"latitude"`
        Longitude float64 `json:"longitude"`
        Altitude  float64 `json:"altitude"`
    } `json:"position"`
    Attitude struct {
        Pitch   float64 `json:"pitch"`
        Bank    float64 `json:"bank"`
        Heading float64 `json:"heading"`
    } `json:"attitude"`
    Performance struct {
        Airspeed    float64 `json:"airspeed"`
        VerticalSpeed float64 `json:"vertical_speed"`
        GroundSpeed float64 `json:"ground_speed"`
    } `json:"performance"`
}

func NewFlightDataRecorder(client *client.Engine, filename string, sampleRate time.Duration) (*FlightDataRecorder, error) {
    file, err := os.Create(filename)
    if err != nil {
        return nil, err
    }
    
    fdr := &FlightDataRecorder{
        client:     client,
        output:     file,
        encoder:    json.NewEncoder(file),
        sampleRate: sampleRate,
    }
    
    fdr.setupDataDefinitions()
    return fdr, nil
}

func (fdr *FlightDataRecorder) setupDataDefinitions() {
    // Complex data definition with all flight parameters
    defineID := 1
    
    // Position
    fdr.client.AddToDataDefinition(defineID, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    fdr.client.AddToDataDefinition(defineID, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
    fdr.client.AddToDataDefinition(defineID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
    
    // Attitude
    fdr.client.AddToDataDefinition(defineID, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 3)
    fdr.client.AddToDataDefinition(defineID, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 4)
    fdr.client.AddToDataDefinition(defineID, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 5)
    
    // Performance
    fdr.client.AddToDataDefinition(defineID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 6)
    fdr.client.AddToDataDefinition(defineID, "VERTICAL SPEED", "feet per minute", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 7)
    fdr.client.AddToDataDefinition(defineID, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 8)
    
    // Request high-frequency data
    fdr.client.RequestDataOnSimObject(1, defineID, 0, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
}

func (fdr *FlightDataRecorder) StartRecording() {
    fdr.mu.Lock()
    defer fdr.mu.Unlock()
    fdr.recording = true
}

func (fdr *FlightDataRecorder) StopRecording() {
    fdr.mu.Lock()
    defer fdr.mu.Unlock()
    fdr.recording = false
}

func (fdr *FlightDataRecorder) ProcessMessage(message client.ParsedMessage) {
    fdr.mu.Lock()
    defer fdr.mu.Unlock()
    
    if !fdr.recording || !message.IsSimObjectData() {
        return
    }
    
    now := time.Now()
    if now.Sub(fdr.lastSample) < fdr.sampleRate {
        return // Skip this sample
    }
    
    data, ok := message.Data.(*types.SIMCONNECT_RECV_SIMOBJECT_DATA)
    if !ok || data.DwRequestID != 1 {
        return
    }
    
    record := fdr.parseFlightData(message)
    if err := fdr.encoder.Encode(record); err != nil {
        log.Printf("Failed to encode flight data: %v", err)
    }
    
    fdr.lastSample = now
}

func (fdr *FlightDataRecorder) parseFlightData(message client.ParsedMessage) FlightDataRecord {
    // Parse the flight data from raw bytes
    // This would involve unsafe pointer operations similar to previous examples
    // Implementation details omitted for brevity
    
    return FlightDataRecord{
        Timestamp: time.Now(),
        // ... populate fields from message.RawData
    }
}

func (fdr *FlightDataRecorder) Close() error {
    fdr.mu.Lock()
    defer fdr.mu.Unlock()
    
    fdr.recording = false
    return fdr.output.Close()
}
```

These advanced patterns provide robust, performant, and maintainable solutions for complex SimConnect applications. Remember to always profile your application and optimize based on actual performance requirements.
