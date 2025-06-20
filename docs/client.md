# Client API Reference

The `client` package provides the core functionality for connecting to and communicating with SimConnect.

## Table of Contents

• [Engine](#engine)  
• [Connection Management](#connection-management)  
• [Message Processing](#message-processing)  
• [Constants](#constants)

## Engine

The `Engine` struct is the main client for SimConnect operations.

### Constructor

#### `New(name string) *Engine`

Creates a new SimConnect client with the specified name.

**Parameters:**
- `name` (string): The application name that will be displayed in SimConnect

**Returns:**
- `*Engine`: A new Engine instance, or `nil` if initialization fails

**Example:**
```go
simClient := client.New("MyFlightApp")
if simClient == nil {
    log.Fatal("Failed to create SimConnect client")
}
```

### Engine Structure

```go
type Engine struct {
    ctx       context.Context    // Context for cancellation
    cancel    context.CancelFunc // Function to cancel the context
    dll       *syscall.LazyDLL   // The DLL handle for SimConnect.dll
    handle    uintptr            // The handle to the SimConnect connection
    name      string             // The name of the SimConnect client
    queue     chan ParsedMessage // Channel for parsed message queueing
    wg        sync.WaitGroup     // WaitGroup to coordinate goroutines
    once      sync.Once          // Ensure cleanup happens only once
    isClosing bool               // Flag to indicate if we're shutting down
    mu        sync.RWMutex       // Mutex to protect isClosing flag
}
```

## Connection Management

### `Connect() error`

Establishes a connection to SimConnect.

**Returns:**
- `error`: Error if connection fails, `nil` on success

**Example:**
```go
if err := simClient.Connect(); err != nil {
    log.Fatal("Failed to connect:", err)
}
```

### `Disconnect() error`

Closes the SimConnect connection and performs cleanup. This method is thread-safe and can be called multiple times.

**Returns:**
- `error`: Error if disconnection fails, `nil` on success

**Example:**
```go
defer simClient.Disconnect()
```

### `Shutdown() error`

Alias for `Disconnect()`. Triggers a graceful shutdown of the SimConnect client.

**Returns:**
- `error`: Error if shutdown fails, `nil` on success

## Message Processing

### `Stream() <-chan ParsedMessage`

Returns a read-only channel that streams parsed messages from SimConnect.

**Returns:**
- `<-chan ParsedMessage`: Channel that receives parsed SimConnect messages

**Example:**
```go
for message := range simClient.Stream() {
    if message.Error != nil {
        fmt.Printf("Error: %v\n", message.Error)
        continue
    }
    
    switch {
    case message.IsSimObjectData():
        fmt.Println("Received sim object data")
    case message.IsEvent():
        fmt.Println("Received event")
    case message.IsException():
        fmt.Println("Received exception")
    }
}
```

## Constants

### `DLL_DEFAULT_PATH`

Default path to the SimConnect DLL.

```go
const DLL_DEFAULT_PATH = "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"
```

### `DEFAULT_STREAM_BUFFER_SIZE`

Default buffer size for the message stream channel.

```go
const DEFAULT_STREAM_BUFFER_SIZE = 100
```

## Thread Safety

The `Engine` is designed to be thread-safe:

- Connection and disconnection operations use `sync.Once` to ensure they only happen once
- The `isClosing` flag is protected by a `sync.RWMutex`
- Message processing uses goroutines with proper synchronization via `sync.WaitGroup`

## Error Handling

All connection-related methods return errors that should be checked:

```go
simClient := client.New("MyApp")
if simClient == nil {
    // Handle creation failure
}

if err := simClient.Connect(); err != nil {
    // Handle connection error
}

// Process messages
for message := range simClient.Stream() {
    if message.Error != nil {
        // Handle message parsing error
        continue
    }
    // Process valid message
}
```

## Best Practices

1. **Always check for nil**: The `New()` function can return `nil` on failure
2. **Use defer for cleanup**: Always use `defer simClient.Disconnect()` after successful connection
3. **Handle message errors**: Check `message.Error` before processing message data
4. **Graceful shutdown**: Use context cancellation or signal handling for clean application shutdown

```go
// Good practice example
func main() {
    simClient := client.New("MyApp")
    if simClient == nil {
        log.Fatal("Failed to create client")
    }
    
    if err := simClient.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    defer simClient.Disconnect()
    
    // Handle shutdown signals
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        log.Println("Shutting down...")
        simClient.Shutdown()
        os.Exit(0)
    }()
    
    // Process messages
    for message := range simClient.Stream() {
        // Handle messages...
    }
}
```
