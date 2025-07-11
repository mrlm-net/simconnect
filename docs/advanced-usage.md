# Advanced Usage

This document covers advanced patterns, performance optimization, and sophisticated use cases for the SimConnect Go library.

## Table of Contents

- [High-Performance Data Streaming](#high-performance-data-streaming)
- [Custom Message Processing](#custom-message-processing)
- [Multi-Client Architecture](#multi-client-architecture)
- [Memory Management](#memory-management)
- [Performance Optimization](#performance-optimization)
- [Advanced Event Handling](#advanced-event-handling)
- [Client Data Areas](#client-data-areas)
- [Facility Data Integration](#facility-data-integration)
- [Input Event Management](#input-event-management)
- [Real-World Applications](#real-world-applications)

## High-Performance Data Streaming

For applications requiring high-frequency data updates with minimal latency, implement optimized streaming patterns.

### Optimized Data Structures

Use memory-aligned structures for efficient data access:

```go
// Aligned struct for performance
type HighFrequencyData struct {
    Timestamp       int64   // 8 bytes
    Altitude        float64 // 8 bytes
    Speed           float64 // 8 bytes
    Heading         float64 // 8 bytes
    VerticalSpeed   float64 // 8 bytes
    PitchDegrees    float64 // 8 bytes
    BankDegrees     float64 // 8 bytes
    GForce          float64 // 8 bytes
    _               [8]byte // Padding for 64-byte alignment
}

// Size: 64 bytes (cache line aligned)
```

### Batch Processing

Process multiple messages in batches to reduce overhead:

```go
type BatchProcessor struct {
    sc           *client.Engine
    batchSize    int
    batchBuffer  []ParsedMessage
    flushTimer   *time.Timer
    flushChan    chan struct{}
    processor    func([]ParsedMessage)
}

func NewBatchProcessor(sc *client.Engine, batchSize int, flushInterval time.Duration, processor func([]ParsedMessage)) *BatchProcessor {
    bp := &BatchProcessor{
        sc:          sc,
        batchSize:   batchSize,
        batchBuffer: make([]ParsedMessage, 0, batchSize),
        flushChan:   make(chan struct{}, 1),
        processor:   processor,
    }
    
    bp.flushTimer = time.AfterFunc(flushInterval, func() {
        select {
        case bp.flushChan <- struct{}{}:
        default:
        }
    })
    
    return bp
}

func (bp *BatchProcessor) Start() {
    messages := bp.sc.Stream()
    
    for {
        select {
        case msg := <-messages:
            bp.addToBatch(msg)
        case <-bp.flushChan:
            bp.flush()
        }
    }
}

func (bp *BatchProcessor) addToBatch(msg ParsedMessage) {
    bp.batchBuffer = append(bp.batchBuffer, msg)
    
    if len(bp.batchBuffer) >= bp.batchSize {
        bp.flush()
    }
}

func (bp *BatchProcessor) flush() {
    if len(bp.batchBuffer) > 0 {
        bp.processor(bp.batchBuffer)
        bp.batchBuffer = bp.batchBuffer[:0] // Clear but keep capacity
    }
    bp.flushTimer.Reset(time.Millisecond * 100)
}
```

### Lock-Free Message Queue

Implement a lock-free queue for high-throughput scenarios:

```go
type LockFreeQueue struct {
    head   uint64
    tail   uint64
    mask   uint64
    buffer []unsafe.Pointer
}

func NewLockFreeQueue(size int) *LockFreeQueue {
    // Size must be power of 2
    if size&(size-1) != 0 {
        panic("size must be power of 2")
    }
    
    return &LockFreeQueue{
        buffer: make([]unsafe.Pointer, size),
        mask:   uint64(size - 1),
    }
}

func (q *LockFreeQueue) Enqueue(item unsafe.Pointer) bool {
    tail := atomic.LoadUint64(&q.tail)
    head := atomic.LoadUint64(&q.head)
    
    if tail-head >= uint64(len(q.buffer)) {
        return false // Queue full
    }
    
    q.buffer[tail&q.mask] = item
    atomic.StoreUint64(&q.tail, tail+1)
    return true
}

func (q *LockFreeQueue) Dequeue() unsafe.Pointer {
    head := atomic.LoadUint64(&q.head)
    tail := atomic.LoadUint64(&q.tail)
    
    if head >= tail {
        return nil // Queue empty
    }
    
    item := q.buffer[head&q.mask]
    atomic.StoreUint64(&q.head, head+1)
    return item
}
```

## Custom Message Processing

Implement custom message processors for specialized handling:

### Low-Level Message Handler

```go
type CustomMessageHandler struct {
    sc              *client.Engine
    dataProcessors  map[types.SIMCONNECT_RECV_ID]func(*types.SIMCONNECT_RECV, uint32)
    rawBuffer       []byte
    messageStats    map[types.SIMCONNECT_RECV_ID]int64
    statsMutex      sync.RWMutex
}

func NewCustomMessageHandler(sc *client.Engine) *CustomMessageHandler {
    return &CustomMessageHandler{
        sc:             sc,
        dataProcessors: make(map[types.SIMCONNECT_RECV_ID]func(*types.SIMCONNECT_RECV, uint32)),
        rawBuffer:      make([]byte, 65536), // 64KB buffer
        messageStats:   make(map[types.SIMCONNECT_RECV_ID]int64),
    }
}

func (cmh *CustomMessageHandler) RegisterProcessor(messageType types.SIMCONNECT_RECV_ID, processor func(*types.SIMCONNECT_RECV, uint32)) {
    cmh.dataProcessors[messageType] = processor
}

func (cmh *CustomMessageHandler) ProcessMessage(recv *types.SIMCONNECT_RECV, cbData uint32, context uintptr) {
    // Update statistics
    cmh.statsMutex.Lock()
    cmh.messageStats[recv.DwID]++
    cmh.statsMutex.Unlock()
    
    // Find and execute processor
    if processor, exists := cmh.dataProcessors[recv.DwID]; exists {
        processor(recv, cbData)
    } else {
        cmh.defaultProcessor(recv, cbData)
    }
}

func (cmh *CustomMessageHandler) defaultProcessor(recv *types.SIMCONNECT_RECV, cbData uint32) {
    // Default handling for unregistered message types
    log.Printf("Unhandled message type: %d, size: %d", recv.DwID, cbData)
}

func (cmh *CustomMessageHandler) Start() error {
    callback := func(recv *types.SIMCONNECT_RECV, cbData uint32, context uintptr) {
        cmh.ProcessMessage(recv, cbData, context)
    }
    
    return cmh.sc.DispatchProc(callback, 0)
}
```

### Message Filtering Pipeline

```go
type MessageFilter interface {
    Filter(msg ParsedMessage) bool
}

type MessageProcessor interface {
    Process(msg ParsedMessage) error
}

type FilteredProcessor struct {
    filters    []MessageFilter
    processors []MessageProcessor
}

func NewFilteredProcessor() *FilteredProcessor {
    return &FilteredProcessor{
        filters:    make([]MessageFilter, 0),
        processors: make([]MessageProcessor, 0),
    }
}

func (fp *FilteredProcessor) AddFilter(filter MessageFilter) {
    fp.filters = append(fp.filters, filter)
}

func (fp *FilteredProcessor) AddProcessor(processor MessageProcessor) {
    fp.processors = append(fp.processors, processor)
}

func (fp *FilteredProcessor) ProcessMessage(msg ParsedMessage) error {
    // Apply filters
    for _, filter := range fp.filters {
        if !filter.Filter(msg) {
            return nil // Message filtered out
        }
    }
    
    // Process message
    for _, processor := range fp.processors {
        if err := processor.Process(msg); err != nil {
            return err
        }
    }
    
    return nil
}

// Example filters
type MessageTypeFilter struct {
    allowedTypes map[types.SIMCONNECT_RECV_ID]bool
}

func (mtf *MessageTypeFilter) Filter(msg ParsedMessage) bool {
    return mtf.allowedTypes[msg.MessageType]
}

type RateLimitFilter struct {
    lastMessage time.Time
    minInterval time.Duration
}

func (rlf *RateLimitFilter) Filter(msg ParsedMessage) bool {
    now := time.Now()
    if now.Sub(rlf.lastMessage) >= rlf.minInterval {
        rlf.lastMessage = now
        return true
    }
    return false
}
```

## Multi-Client Architecture

For applications requiring multiple SimConnect connections:

### Connection Pool

```go
type ConnectionPool struct {
    connections []*client.Engine
    current     int
    mutex       sync.RWMutex
    names       []string
}

func NewConnectionPool(names []string) (*ConnectionPool, error) {
    pool := &ConnectionPool{
        connections: make([]*client.Engine, 0, len(names)),
        names:       names,
    }
    
    for _, name := range names {
        sc := client.New(name)
        if sc == nil {
            return nil, fmt.Errorf("failed to create client: %s", name)
        }
        
        if err := sc.Connect(); err != nil {
            return nil, fmt.Errorf("failed to connect client %s: %v", name, err)
        }
        
        pool.connections = append(pool.connections, sc)
    }
    
    return pool, nil
}

func (cp *ConnectionPool) GetConnection() *client.Engine {
    cp.mutex.Lock()
    defer cp.mutex.Unlock()
    
    conn := cp.connections[cp.current]
    cp.current = (cp.current + 1) % len(cp.connections)
    return conn
}

func (cp *ConnectionPool) ExecuteOnAll(fn func(*client.Engine) error) error {
    cp.mutex.RLock()
    defer cp.mutex.RUnlock()
    
    for i, conn := range cp.connections {
        if err := fn(conn); err != nil {
            return fmt.Errorf("error on connection %s: %v", cp.names[i], err)
        }
    }
    return nil
}

func (cp *ConnectionPool) Close() error {
    cp.mutex.Lock()
    defer cp.mutex.Unlock()
    
    var lastErr error
    for _, conn := range cp.connections {
        if err := conn.Disconnect(); err != nil {
            lastErr = err
        }
    }
    return lastErr
}
```

### Load Balancing

```go
type LoadBalancer struct {
    pool       *ConnectionPool
    strategy   LoadBalanceStrategy
    statistics map[int]*ConnectionStats
    mutex      sync.RWMutex
}

type LoadBalanceStrategy int

const (
    RoundRobin LoadBalanceStrategy = iota
    LeastConnections
    ResponseTime
)

type ConnectionStats struct {
    ActiveRequests int64
    TotalRequests  int64
    AvgResponseTime time.Duration
    LastUsed       time.Time
}

func (lb *LoadBalancer) selectConnection() *client.Engine {
    lb.mutex.RLock()
    defer lb.mutex.RUnlock()
    
    switch lb.strategy {
    case LeastConnections:
        return lb.selectLeastConnections()
    case ResponseTime:
        return lb.selectFastestResponse()
    default:
        return lb.pool.GetConnection()
    }
}

func (lb *LoadBalancer) selectLeastConnections() *client.Engine {
    minConnections := int64(math.MaxInt64)
    var selected *client.Engine
    
    for i, conn := range lb.pool.connections {
        stats := lb.statistics[i]
        if stats.ActiveRequests < minConnections {
            minConnections = stats.ActiveRequests
            selected = conn
        }
    }
    
    return selected
}
```

## Memory Management

Efficient memory management for high-performance applications:

### Object Pooling

```go
type MessagePool struct {
    pool sync.Pool
}

func NewMessagePool() *MessagePool {
    return &MessagePool{
        pool: sync.Pool{
            New: func() interface{} {
                return &ParsedMessage{
                    RawData: make([]byte, 0, 1024),
                }
            },
        },
    }
}

func (mp *MessagePool) Get() *ParsedMessage {
    return mp.pool.Get().(*ParsedMessage)
}

func (mp *MessagePool) Put(msg *ParsedMessage) {
    // Reset message
    msg.MessageType = 0
    msg.Header = nil
    msg.Data = nil
    msg.RawData = msg.RawData[:0]
    msg.Error = nil
    
    mp.pool.Put(msg)
}
```

### Buffer Management

```go
type BufferManager struct {
    buffers map[int]sync.Pool
    mutex   sync.RWMutex
}

func NewBufferManager() *BufferManager {
    return &BufferManager{
        buffers: make(map[int]sync.Pool),
    }
}

func (bm *BufferManager) GetBuffer(size int) []byte {
    // Round up to next power of 2
    size = nextPowerOf2(size)
    
    bm.mutex.RLock()
    pool, exists := bm.buffers[size]
    bm.mutex.RUnlock()
    
    if !exists {
        bm.mutex.Lock()
        pool = sync.Pool{
            New: func() interface{} {
                return make([]byte, size)
            },
        }
        bm.buffers[size] = pool
        bm.mutex.Unlock()
    }
    
    return pool.Get().([]byte)
}

func (bm *BufferManager) PutBuffer(buf []byte) {
    size := len(buf)
    
    bm.mutex.RLock()
    pool, exists := bm.buffers[size]
    bm.mutex.RUnlock()
    
    if exists {
        pool.Put(buf)
    }
}

func nextPowerOf2(n int) int {
    if n <= 0 {
        return 1
    }
    n--
    n |= n >> 1
    n |= n >> 2
    n |= n >> 4
    n |= n >> 8
    n |= n >> 16
    n++
    return n
}
```

## Performance Optimization

### Data Request Optimization

```go
type OptimizedDataManager struct {
    sc               *client.Engine
    definitions      map[int]*DataDefinition
    activeRequests   map[int]*DataRequest
    requestScheduler *RequestScheduler
}

type DataDefinition struct {
    ID        int
    Variables []VariableDefinition
    Size      int
    Frequency time.Duration
}

type VariableDefinition struct {
    Name     string
    Units    string
    DataType types.SIMCONNECT_DATATYPE
    Epsilon  float32
}

type DataRequest struct {
    ID          int
    DefinitionID int
    LastUpdate  time.Time
    UpdateRate  time.Duration
    Priority    int
}

func (odm *OptimizedDataManager) OptimizeRequests() {
    // Group requests by update frequency
    frequencyGroups := make(map[time.Duration][]*DataRequest)
    
    for _, req := range odm.activeRequests {
        frequencyGroups[req.UpdateRate] = append(frequencyGroups[req.UpdateRate], req)
    }
    
    // Schedule requests to minimize SimConnect load
    for frequency, requests := range frequencyGroups {
        odm.scheduleRequestGroup(requests, frequency)
    }
}

func (odm *OptimizedDataManager) scheduleRequestGroup(requests []*DataRequest, frequency time.Duration) {
    // Stagger requests to distribute load
    interval := frequency / time.Duration(len(requests))
    
    for i, req := range requests {
        delay := time.Duration(i) * interval
        odm.requestScheduler.ScheduleRequest(req, delay)
    }
}
```

### Adaptive Sampling

```go
type AdaptiveSampler struct {
    sc             *client.Engine
    variables      map[string]*VariableState
    baseFrequency  time.Duration
    maxFrequency   time.Duration
    minFrequency   time.Duration
}

type VariableState struct {
    Name           string
    CurrentValue   float64
    LastValue      float64
    ChangeRate     float64
    Frequency      time.Duration
    LastUpdate     time.Time
    SampleHistory  []float64
    StabilityScore float64
}

func (as *AdaptiveSampler) UpdateSamplingRate(variable *VariableState) {
    // Calculate change rate
    timeDelta := time.Since(variable.LastUpdate)
    valueDelta := math.Abs(variable.CurrentValue - variable.LastValue)
    variable.ChangeRate = valueDelta / timeDelta.Seconds()
    
    // Calculate stability score
    as.calculateStability(variable)
    
    // Adjust frequency based on change rate and stability
    if variable.ChangeRate > 1.0 || variable.StabilityScore < 0.8 {
        // High change rate or low stability - increase frequency
        variable.Frequency = time.Duration(float64(variable.Frequency) * 0.8)
        if variable.Frequency < as.maxFrequency {
            variable.Frequency = as.maxFrequency
        }
    } else if variable.ChangeRate < 0.1 && variable.StabilityScore > 0.95 {
        // Low change rate and high stability - decrease frequency
        variable.Frequency = time.Duration(float64(variable.Frequency) * 1.2)
        if variable.Frequency > as.minFrequency {
            variable.Frequency = as.minFrequency
        }
    }
}

func (as *AdaptiveSampler) calculateStability(variable *VariableState) {
    if len(variable.SampleHistory) < 10 {
        variable.StabilityScore = 0.5 // Default for insufficient data
        return
    }
    
    // Calculate coefficient of variation
    mean := 0.0
    for _, value := range variable.SampleHistory {
        mean += value
    }
    mean /= float64(len(variable.SampleHistory))
    
    variance := 0.0
    for _, value := range variable.SampleHistory {
        variance += math.Pow(value-mean, 2)
    }
    variance /= float64(len(variable.SampleHistory))
    
    stdDev := math.Sqrt(variance)
    cv := stdDev / mean
    
    // Convert to stability score (0-1, higher is more stable)
    variable.StabilityScore = 1.0 / (1.0 + cv)
}
```

## Advanced Event Handling

### Event Aggregation

```go
type EventAggregator struct {
    events      map[types.SIMCONNECT_RECV_ID][]ParsedMessage
    handlers    map[types.SIMCONNECT_RECV_ID]func([]ParsedMessage)
    flushTimer  *time.Timer
    flushPeriod time.Duration
    mutex       sync.RWMutex
}

func NewEventAggregator(flushPeriod time.Duration) *EventAggregator {
    ea := &EventAggregator{
        events:      make(map[types.SIMCONNECT_RECV_ID][]ParsedMessage),
        handlers:    make(map[types.SIMCONNECT_RECV_ID]func([]ParsedMessage)),
        flushPeriod: flushPeriod,
    }
    
    ea.flushTimer = time.AfterFunc(flushPeriod, ea.flush)
    return ea
}

func (ea *EventAggregator) AddEvent(msg ParsedMessage) {
    ea.mutex.Lock()
    defer ea.mutex.Unlock()
    
    ea.events[msg.MessageType] = append(ea.events[msg.MessageType], msg)
}

func (ea *EventAggregator) RegisterHandler(eventType types.SIMCONNECT_RECV_ID, handler func([]ParsedMessage)) {
    ea.mutex.Lock()
    defer ea.mutex.Unlock()
    
    ea.handlers[eventType] = handler
}

func (ea *EventAggregator) flush() {
    ea.mutex.Lock()
    events := ea.events
    ea.events = make(map[types.SIMCONNECT_RECV_ID][]ParsedMessage)
    ea.mutex.Unlock()
    
    // Process aggregated events
    for eventType, eventList := range events {
        if handler, exists := ea.handlers[eventType]; exists {
            go handler(eventList) // Process asynchronously
        }
    }
    
    // Reset timer
    ea.flushTimer.Reset(ea.flushPeriod)
}
```

## Client Data Areas

Client data areas enable communication between SimConnect clients:

### Shared Data Manager

```go
type SharedDataManager struct {
    sc         *client.Engine
    areas      map[string]*ClientDataArea
    mutex      sync.RWMutex
}

type ClientDataArea struct {
    Name       string
    ID         int
    Size       int
    Definition int
    Data       []byte
    LastUpdate time.Time
}

func NewSharedDataManager(sc *client.Engine) *SharedDataManager {
    return &SharedDataManager{
        sc:    sc,
        areas: make(map[string]*ClientDataArea),
    }
}

func (sdm *SharedDataManager) CreateArea(name string, id int, size int) error {
    // Create client data area
    err := sdm.sc.CreateClientData(
        id,
        size,
        types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG_DEFAULT,
    )
    if err != nil {
        return err
    }
    
    // Map name to ID
    err = sdm.sc.MapClientDataNameToID(name, id)
    if err != nil {
        return err
    }
    
    // Store area info
    sdm.mutex.Lock()
    sdm.areas[name] = &ClientDataArea{
        Name: name,
        ID:   id,
        Size: size,
        Data: make([]byte, size),
    }
    sdm.mutex.Unlock()
    
    return nil
}

func (sdm *SharedDataManager) WriteData(areaName string, offset int, data []byte) error {
    sdm.mutex.RLock()
    area, exists := sdm.areas[areaName]
    sdm.mutex.RUnlock()
    
    if !exists {
        return fmt.Errorf("client data area %s not found", areaName)
    }
    
    // Update local copy
    copy(area.Data[offset:], data)
    area.LastUpdate = time.Now()
    
    // Write to SimConnect
    return sdm.sc.SetClientData(
        area.ID,
        area.Definition,
        types.SIMCONNECT_CLIENT_DATA_SET_FLAG_DEFAULT,
        0,
        len(data),
        uintptr(unsafe.Pointer(&data[0])),
    )
}
```

## Real-World Applications

### Flight Data Recorder

```go
type FlightDataRecorder struct {
    sc           *client.Engine
    file         *os.File
    encoder      *json.Encoder
    recording    bool
    sampleRate   time.Duration
    lastSample   time.Time
    buffer       []FlightDataPoint
    bufferSize   int
    flushTicker  *time.Ticker
}

type FlightDataPoint struct {
    Timestamp     time.Time `json:"timestamp"`
    Altitude      float64   `json:"altitude"`
    Speed         float64   `json:"speed"`
    Heading       float64   `json:"heading"`
    Latitude      float64   `json:"latitude"`
    Longitude     float64   `json:"longitude"`
    VerticalSpeed float64   `json:"vertical_speed"`
    FuelQuantity  float64   `json:"fuel_quantity"`
}

func NewFlightDataRecorder(sc *client.Engine, filename string, sampleRate time.Duration) (*FlightDataRecorder, error) {
    file, err := os.Create(filename)
    if err != nil {
        return nil, err
    }
    
    fdr := &FlightDataRecorder{
        sc:         sc,
        file:       file,
        encoder:    json.NewEncoder(file),
        sampleRate: sampleRate,
        bufferSize: 100,
        buffer:     make([]FlightDataPoint, 0, 100),
        flushTicker: time.NewTicker(time.Second * 10),
    }
    
    go fdr.flushLoop()
    return fdr, nil
}

func (fdr *FlightDataRecorder) StartRecording() error {
    fdr.recording = true
    return fdr.setupDataDefinition()
}

func (fdr *FlightDataRecorder) StopRecording() error {
    fdr.recording = false
    fdr.flushBuffer()
    return fdr.file.Close()
}

func (fdr *FlightDataRecorder) ProcessMessage(msg ParsedMessage) {
    if !fdr.recording || !msg.IsSimObjectData() {
        return
    }
    
    now := time.Now()
    if now.Sub(fdr.lastSample) < fdr.sampleRate {
        return
    }
    
    if data, ok := msg.GetSimObjectData(); ok {
        point := fdr.extractDataPoint(data, now)
        fdr.addToBuffer(point)
        fdr.lastSample = now
    }
}

func (fdr *FlightDataRecorder) addToBuffer(point FlightDataPoint) {
    fdr.buffer = append(fdr.buffer, point)
    
    if len(fdr.buffer) >= fdr.bufferSize {
        fdr.flushBuffer()
    }
}

func (fdr *FlightDataRecorder) flushBuffer() {
    for _, point := range fdr.buffer {
        fdr.encoder.Encode(point)
    }
    fdr.buffer = fdr.buffer[:0]
    fdr.file.Sync()
}

func (fdr *FlightDataRecorder) flushLoop() {
    for range fdr.flushTicker.C {
        if len(fdr.buffer) > 0 {
            fdr.flushBuffer()
        }
    }
}
```

### Aircraft Performance Monitor

```go
type PerformanceMonitor struct {
    sc              *client.Engine
    metrics         map[string]*PerformanceMetric
    alerts          []PerformanceAlert
    thresholds      map[string]Threshold
    calculators     map[string]Calculator
}

type PerformanceMetric struct {
    Name        string
    Value       float64
    Min         float64
    Max         float64
    Average     float64
    SampleCount int64
    LastUpdate  time.Time
}

type PerformanceAlert struct {
    Type      string
    Message   string
    Severity  AlertSeverity
    Timestamp time.Time
}

type AlertSeverity int

const (
    Info AlertSeverity = iota
    Warning
    Critical
)

func (pm *PerformanceMonitor) UpdateMetrics(data *AircraftState) {
    // Update basic metrics
    pm.updateMetric("altitude", data.Altitude)
    pm.updateMetric("speed", data.Speed)
    pm.updateMetric("heading", data.Heading)
    
    // Calculate derived metrics
    pm.calculateDerivedMetrics(data)
    
    // Check thresholds
    pm.checkThresholds()
}

func (pm *PerformanceMonitor) calculateDerivedMetrics(data *AircraftState) {
    // Calculate rate of climb
    if altMetric := pm.metrics["altitude"]; altMetric != nil {
        timeDelta := time.Since(altMetric.LastUpdate).Seconds()
        if timeDelta > 0 {
            rateOfClimb := (data.Altitude - altMetric.Value) / timeDelta
            pm.updateMetric("rate_of_climb", rateOfClimb)
        }
    }
    
    // Calculate acceleration
    if speedMetric := pm.metrics["speed"]; speedMetric != nil {
        timeDelta := time.Since(speedMetric.LastUpdate).Seconds()
        if timeDelta > 0 {
            acceleration := (data.Speed - speedMetric.Value) / timeDelta
            pm.updateMetric("acceleration", acceleration)
        }
    }
}

func (pm *PerformanceMonitor) updateMetric(name string, value float64) {
    metric, exists := pm.metrics[name]
    if !exists {
        metric = &PerformanceMetric{
            Name: name,
            Min:  value,
            Max:  value,
        }
        pm.metrics[name] = metric
    }
    
    // Update statistics
    metric.Value = value
    metric.SampleCount++
    
    if value < metric.Min {
        metric.Min = value
    }
    if value > metric.Max {
        metric.Max = value
    }
    
    // Update rolling average
    metric.Average = (metric.Average*float64(metric.SampleCount-1) + value) / float64(metric.SampleCount)
    metric.LastUpdate = time.Now()
}
```

These advanced patterns and techniques enable the development of sophisticated, high-performance SimConnect applications capable of handling complex real-world scenarios.
