# Message Parsing Example

Demonstrates proper SimConnect message handling patterns and application lifecycle management.

## What it demonstrates

- **Signal Handling**: Proper Ctrl+C and system signal processing
- **Graceful Shutdown**: Clean resource cleanup and connection termination
- **Message Loop Patterns**: Robust message processing with timeout handling
- **Error Recovery**: Exception handling and connection state management
- **Application Lifecycle**: Complete startup and shutdown sequence

## How to run

```bash
cd examples/message-parsing
go run main.go
```

The example runs for 30 seconds automatically, or until Ctrl+C is pressed.

## Key Features

- **Automatic Timeout**: Demonstrates time-based shutdown (30 seconds)
- **Signal Handling**: Graceful response to interrupt signals
- **Message Classification**: Proper handling of different message types
- **Resource Cleanup**: Ensures SimConnect connection is properly closed
- **Error Logging**: Comprehensive error reporting and debugging

## Key code patterns

```go
// Signal handling setup
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

// Timeout handling
timeout := time.After(30 * time.Second)

// Message processing with multiple exit conditions
select {
case <-done:           // Signal received
case <-timeout:        // Timeout reached  
case msg := <-stream:  // Message received
}

// Proper cleanup
defer func() {
    simClient.Disconnect()
    fmt.Println("Resources cleaned up")
}()
```

## Message Types Handled

- **Open Messages**: Connection confirmation
- **Quit Messages**: SimConnect shutdown signals  
- **Exception Messages**: Error conditions and recovery
- **Timeout Events**: Application lifecycle management

## Requirements

- Running MSFS (connection will be established but no specific aircraft needed)
- Used as foundation pattern for other applications
