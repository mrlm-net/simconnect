# Set Variables Example

## Overview

This example demonstrates how to read and write simulator variables (SimVars) in Microsoft Flight Simulator using the SimConnect SDK. It shows how to request data from the simulator and dynamically change camera views by setting the `CAMERA STATE` variable.

## What It Does

1. **Connects to SimConnect** - Establishes connection to the simulator
2. **Defines data structures** - Maps SimConnect variables to Go structs
3. **Requests camera data** - Periodically reads current camera state and substate
4. **Sets camera views** - Cycles through different camera angles automatically:
   - External View (3)
   - Cockpit View (2)
   - Drone View (4)
5. **Demonstrates write operations** - Shows how to use `SetDataOnSimObject` to modify simulator state
6. **Handles shutdown** - Responds to Ctrl+C interrupt signals cleanly

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 running
- SimConnect SDK installed
- An aircraft loaded in the simulator (to see camera changes)

## Running the Example

```bash
cd examples/set-variables
go run main.go
```

## Expected Output

```
✅ Connected to SimConnect...
⏳ Sleeping for 2 seconds...
✈️  Ready for takeoff!
🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)
📡 Received SIMCONNECT_RECV_OPEN message!
  => Received SimObject data event
&{CameraState:2 CameraSubstate:0}
🔄 Setting camera state to EXTERNAL VIEW (3)
  => Received SimObject data event
&{CameraState:3 CameraSubstate:0}
🔄 Setting camera state to COCKPIT VIEW (2)
🔄 Setting camera state to DRONE VIEW (4)
🔄 Setting camera state to COCKPIT VIEW (2)
🔄 Setting camera state to EXTERNAL VIEW (3)
🛑 Finished setting camera states, exiting...
🔌 Context cancelled, disconnecting...
```

## Code Explanation

### Data Structure

The example uses a custom struct to map SimConnect variables:

```go
type CameraData struct {
    CameraState    int32
    CameraSubstate int32
}
```

The fields must match the order of `AddToDataDefinition` calls.

### Camera State Values

- `2` - Cockpit View
- `3` - External View
- `4` - Drone View

### Reading Variables

```go
client.AddToDataDefinition(1000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
client.AddToDataDefinition(1000, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
client.RequestDataOnSimObject(1000, 1000, types.SIMCONNECT_OBJECT_ID_USER, 
    types.SIMCONNECT_PERIOD_SECOND, 0, 0, 0, 0)
```

This requests camera data every second for the user's aircraft.

### Writing Variables

```go
value := int32(3) // External view
client.SetDataOnSimObject(
    2000,                            // definitionID
    types.SIMCONNECT_OBJECT_ID_USER, // objectID (user's aircraft)
    0,                               // flags (default)
    1,                               // arrayCount (single value)
    uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of int32)
    unsafe.Pointer(&value),          // pDataSet (pointer to value)
)
```

### Timing

The example uses goroutines with `time.Sleep()` to schedule camera changes:
- 15s - Switch to External View
- 20s - Switch to Cockpit View
- 25s - Switch to Drone View
- 30s - Switch to Cockpit View
- 35s - Switch to External View
- 50s - Exit program

## Use Cases

This pattern can be used to:
- Control aircraft systems (autopilot, lights, etc.)
- Set navigation data (heading, altitude targets)
- Modify environmental conditions
- Change aircraft position or attitude
- Trigger simulator actions programmatically

## Notes

- Use `unsafe.Pointer` when passing data to `SetDataOnSimObject`
- Ensure data definitions match between read and write operations
- The `cbUnitSize` parameter must accurately reflect the size of the data being set
- Camera changes are visible immediately in the simulator
