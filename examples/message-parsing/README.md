# Message Parsing Example

A comprehensive example demonstrating message processing, graceful shutdown handling, and proper resource management with SimConnect.

## Features

- Parse and display different SimConnect message types
- Graceful shutdown handling with signal processing
- Proper resource cleanup and connection management
- Message type identification and categorization
- Error handling and exception processing

## Usage

```bash
go run main.go
```

The program will:
1. Connect to SimConnect and start message processing
2. Display detailed information about each received message
3. Show message types, data, and any errors
4. Handle shutdown signals (Ctrl+C) gracefully
5. Clean up resources on exit

## What it demonstrates

- **Message Type Processing**: Handling all major SimConnect message types
- **Signal Handling**: Proper response to system signals (SIGINT, SIGTERM)
- **Resource Management**: Clean connection establishment and teardown
- **Error Processing**: Comprehensive error and exception handling
- **Graceful Shutdown**: Coordinated shutdown with timeout handling

## Message Types Shown

The example processes and displays information for:
- **Connection Messages**: Open, quit, and connection state changes
- **Data Messages**: Simulation object data and system information
- **Event Messages**: Simulator events and notifications
- **Exception Messages**: Error conditions and debugging information
- **System Messages**: General system state and status updates

## Key Concepts

- **Message Loop**: Processing SimConnect messages in a continuous loop
- **Type Safety**: Safe message type casting and validation
- **Concurrency**: Using goroutines for signal handling and shutdown coordination
- **Timeout Handling**: Preventing indefinite blocking during shutdown
- **Clean Architecture**: Separation of concerns for maintainable code
