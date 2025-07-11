# Troubleshooting Guide

This document provides solutions to common issues encountered when using the SimConnect Go library.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Connection Problems](#connection-problems)
- [Data Issues](#data-issues)
- [Performance Problems](#performance-problems)
- [Platform-Specific Issues](#platform-specific-issues)
- [Debugging Techniques](#debugging-techniques)

## Installation Issues

### DLL Not Found

**Problem:** `Failed to create SimConnect client - DLL not found`

**Solutions:**
1. **Check DLL Path:** Verify the SimConnect.dll is in the expected location:
   ```
   C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll
   ```

2. **Custom DLL Path:** Use a custom path if the DLL is elsewhere:
   ```go
   sc := client.NewWithDLL("My App", "C:/path/to/SimConnect.dll")
   ```

3. **MSFS SDK Installation:** Ensure Microsoft Flight Simulator SDK is properly installed
4. **Architecture Mismatch:** Ensure you're using the correct DLL architecture (x64)

### Go Module Issues

**Problem:** `module not found` or version conflicts

**Solutions:**
1. **Update Module:**
   ```bash
   go get -u github.com/mrlm-net/simconnect
   go mod tidy
   ```

2. **Clear Module Cache:**
   ```bash
   go clean -modcache
   go mod download
   ```

### Build Platform Issues

**Problem:** `build constraints exclude all Go files`

**Solutions:**
1. **Windows Only:** This library only works on Windows:
   ```bash
   GOOS=windows go build
   ```

2. **Build Tags:** Ensure you're building with Windows build tags:
   ```bash
   go build -tags windows
   ```

## Connection Problems

### Simulator Not Running

**Problem:** `Connection failed: 0x80004005`

**Solutions:**
1. **Start Simulator:** Launch Microsoft Flight Simulator first
2. **Load a Flight:** Ensure you're in an active flight, not just the main menu
3. **SimConnect Enabled:** Verify SimConnect is enabled in simulator settings

### Access Denied

**Problem:** `Connection failed: 0x80070005`

**Solutions:**
1. **Run as Administrator:** Run your application with elevated privileges
2. **Windows Firewall:** Add exception for your application
3. **Antivirus:** Temporarily disable antivirus to test
4. **User Account Control:** Check UAC settings

### Connection Timeouts

**Problem:** Connection attempts time out or hang

**Solutions:**
1. **Network Configuration:** Check Windows network settings
2. **Local Connection:** Ensure using local connection (not network SimConnect)
3. **Port Conflicts:** Check if other applications are using SimConnect
4. **Simulator Mode:** Try different simulator modes (windowed vs fullscreen)

### Version Mismatch

**Problem:** `SIMCONNECT_EXCEPTION_VERSION_MISMATCH`

**Solutions:**
1. **Update SDK:** Use the latest SimConnect SDK version
2. **Check Compatibility:** Verify SDK compatibility with your simulator version
3. **DLL Version:** Ensure using the correct DLL version for your simulator

## Data Issues

### Invalid Variable Names

**Problem:** `SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED`

**Common Issues:**
1. **Typos in Variable Names:**
   ```go
   // ❌ Wrong
   sc.AddToDataDefinition(1, "PLANE_ALTITUDE", "feet", ...)
   
   // ✅ Correct  
   sc.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", ...)
   ```

2. **Case Sensitivity:** SimConnect variable names are case-sensitive
3. **Deprecated Variables:** Some variables may not be available in newer simulators

**Solutions:**
1. **Check Documentation:** Refer to SimConnect SDK documentation for correct variable names
2. **Test Variables:** Test with known working variables first
3. **Variable Verification:** Use simulator debugging tools to verify variable availability

### Data Structure Mismatches

**Problem:** `SIMCONNECT_EXCEPTION_SIZE_MISMATCH`

**Common Causes:**
1. **Struct Field Order:** Fields must match the exact order of AddToDataDefinition calls
2. **Data Type Mismatches:** Go types must match SimConnect types
3. **Padding Issues:** Struct padding can cause size mismatches

**Solutions:**
1. **Verify Field Order:**
   ```go
   // Data definition order must match struct order
   sc.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
   sc.AddToDataDefinition(1, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
   
   type AircraftData struct {
       Altitude float64 // Index 0
       Speed    float64 // Index 1
   }
   ```

2. **Use Correct Data Types:**
   ```go
   // Match SimConnect types exactly
   SIMCONNECT_DATATYPE_FLOAT64 -> float64
   SIMCONNECT_DATATYPE_INT32   -> int32
   SIMCONNECT_DATATYPE_STRING8 -> [8]byte
   ```

### No Data Received

**Problem:** Data requests return no data or zero values

**Solutions:**
1. **Check Request Parameters:**
   ```go
   // Verify object ID and period
   sc.RequestDataOnSimObject(
       requestID,
       definitionID,
       types.SIMCONNECT_OBJECT_ID_USER, // Correct object ID
       types.SIMCONNECT_PERIOD_SIM_FRAME, // Appropriate period
       types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
       0, 1, 0,
   )
   ```

2. **Verify Aircraft State:** Ensure aircraft is spawned and active
3. **Check Filters:** Remove CHANGED flag if data seems missing
4. **Test with Manual Period:** Try `SIMCONNECT_PERIOD_ONCE` for testing

## Performance Problems

### High CPU Usage

**Problem:** Application uses excessive CPU resources

**Solutions:**
1. **Optimize Request Frequency:**
   ```go
   // ❌ Too frequent
   types.SIMCONNECT_PERIOD_SIM_FRAME
   
   // ✅ More reasonable
   types.SIMCONNECT_PERIOD_SECOND
   ```

2. **Use CHANGED Flag:**
   ```go
   // Only receive data when it changes
   types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED
   ```

3. **Limit Data Requests:** Only request data you actually need
4. **Batch Related Data:** Group related variables in single definitions

### Memory Leaks

**Problem:** Memory usage increases over time

**Solutions:**
1. **Proper Cleanup:**
   ```go
   defer sc.Disconnect()
   ```

2. **Message Processing:** Ensure message loop doesn't accumulate data
3. **Context Cancellation:** Use context for proper goroutine cleanup
4. **Resource Management:** Clean up data definitions when no longer needed

### Message Queue Overflow

**Problem:** `Warning: Message queue is full, dropping message`

**Solutions:**
1. **Increase Buffer Size:** Modify the buffer size constant if needed
2. **Process Faster:** Optimize message processing speed
3. **Reduce Data Frequency:** Lower request frequency
4. **Multiple Goroutines:** Use worker pools for heavy processing

## Platform-Specific Issues

### Windows Version Compatibility

**Problem:** Library doesn't work on specific Windows versions

**Requirements:**
- Windows 10 or later (recommended)
- .NET Framework 4.7.2 or later
- Visual C++ Redistributable

**Solutions:**
1. **Update Windows:** Ensure latest Windows updates
2. **Install Dependencies:** Install required redistributables
3. **Compatibility Mode:** Try Windows compatibility mode

### Antivirus False Positives

**Problem:** Antivirus software blocks the application

**Solutions:**
1. **Add Exception:** Add your application to antivirus whitelist
2. **Scan Before Use:** Scan your compiled executable
3. **Code Signing:** Consider code signing for distribution

## Debugging Techniques

### Enable Custom Logging

The library provides basic operational logging. For detailed debugging, implement custom logging in your application:

```go
messageStream := sc.Stream()
for msg := range messageStream {
    // Log all messages for debugging
    log.Printf("Debug: Message Type: %v, Size: %d, Error: %v", 
        msg.MessageType, len(msg.RawData), msg.Error)
    
    if msg.Error != nil {
        log.Printf("Message parsing error: %v", msg.Error)
        continue
    }
    // Process message...
}

### Built-in Debug Methods

The library provides debug methods for performance analysis:

```go
// Request response time statistics
err := sc.RequestResponseTimes(10)
if err != nil {
    log.Printf("Failed to request response times: %v", err)
}

// Get packet correlation information
var packetID uintptr
err = sc.GetLastSentPacketID(uintptr(unsafe.Pointer(&packetID)))
if err != nil {
    log.Printf("Failed to get packet ID: %v", err)
}
```

### Connection Testing

Test basic connection:
```go
func testConnection() {
    sc := client.New("Connection Test")
    if sc == nil {
        log.Fatal("Failed to create client")
    }
    defer sc.Disconnect()
    
    if err := sc.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    
    log.Println("Connection successful!")
    
    // Test message processing
    messageStream := sc.Stream()
    timeout := time.After(5 * time.Second)
    
    for {
        select {
        case msg := <-messageStream:
            if msg.IsOpen() {
                log.Println("Received connection confirmation")
                return
            }
        case <-timeout:
            log.Println("Timeout waiting for connection confirmation")
            return
        }
    }
}
```

### Data Definition Testing

Test individual data definitions:
```go
func testDataDefinition() {
    sc := client.New("Data Test")
    defer sc.Disconnect()
    
    if err := sc.Connect(); err != nil {
        log.Fatal("Connection failed:", err)
    }
    
    // Test simple altitude request
    const ALTITUDE_DEF = 1
    const ALTITUDE_REQUEST = 1
    
    err := sc.AddToDataDefinition(ALTITUDE_DEF, "PLANE ALTITUDE", "feet", 
        types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
    if err != nil {
        log.Fatal("Failed to add data definition:", err)
    }
    
    err = sc.RequestDataOnSimObject(ALTITUDE_REQUEST, ALTITUDE_DEF,
        types.SIMCONNECT_OBJECT_ID_USER,
        types.SIMCONNECT_PERIOD_ONCE,
        types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
        0, 1, 0)
    if err != nil {
        log.Fatal("Failed to request data:", err)
    }
    
    // Wait for response
    messageStream := sc.Stream()
    timeout := time.After(10 * time.Second)
    
    for {
        select {
        case msg := <-messageStream:
            if msg.IsSimObjectData() {
                log.Println("Data definition test successful!")
                return
            } else if msg.IsException() {
                if exc, ok := msg.GetException(); ok {
                    log.Printf("Exception: %d", exc.DwException)
                }
            }
        case <-timeout:
            log.Println("Timeout waiting for data")
            return
        }
    }
}
```

### Message Inspection

Inspect raw messages for debugging:
```go
func inspectMessages(sc *client.Engine) {
    messageStream := sc.Stream()
    
    for msg := range messageStream {
        log.Printf("Message Type: %v", msg.MessageType)
        log.Printf("Message Error: %v", msg.Error)
        log.Printf("Raw Data Length: %d", len(msg.RawData))
        
        if msg.Header != nil {
            log.Printf("Header Size: %d", msg.Header.DwSize)
            log.Printf("Header Version: %d", msg.Header.DwVersion)
        }
    }
}
```

### Common Debug Scenarios

1. **Test without custom DLL path first**
2. **Verify with minimal data requests**
3. **Check message flow with simple event mappings**
4. **Use simulator's built-in SimConnect debugging tools**
5. **Compare with working C++ SimConnect examples**

If you continue experiencing issues after trying these solutions, consider checking the simulator's log files or consulting the Microsoft Flight Simulator SDK documentation for additional troubleshooting steps.
