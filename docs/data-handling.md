# Data Handling

Advanced SimConnect data management patterns and best practices.

## Data Definitions

### Creating Data Definitions

```go
// Basic SimVar definition
client.AddToDataDefinition(
    definitionID,     // Unique ID for this definition
    "PLANE ALTITUDE", // SimVar name
    "feet",           // Unit
    types.DATATYPE_FLOAT64, // Data type
)
```

### Complex Data Structures

When defining struct for SimConnect data, **order matters**:

```go
// ❌ Wrong - fields not in definition order
type AircraftData struct {
    Altitude  float64
    Longitude float64  // Defined after latitude
    Latitude  float64  // Defined before longitude
}

// ✅ Correct - matches AddToDataDefinition order
type AircraftData struct {
    Altitude  float64  // First definition
    Latitude  float64  // Second definition  
    Longitude float64  // Third definition
}
```

### Data Types Mapping

| Go Type | SimConnect Type | Usage |
|---------|-----------------|-------|
| `float64` | `DATATYPE_FLOAT64` | Most numeric SimVars |
| `int32` | `DATATYPE_INT32` | Boolean flags, enums |
| `string` | `DATATYPE_STRING256` | Text data (rare) |

### Unit Conversion

Many SimVars return radians - convert to degrees:

```go
func radiansToDegrees(radians float64) float64 {
    return radians * 180.0 / math.Pi
}

// In message handler
latitude := radiansToDegrees(data.Latitude)
```

## Request Patterns

### Periodic Data Updates

```go
// Request data every simulation frame
client.RequestDataOnSimObject(
    requestID,
    definitionID, 
    types.SIMOBJECT_TYPE_USER,
    types.PERIOD_SIM_FRAME,
)

// Alternative periods:
// PERIOD_NEVER       - No automatic updates
// PERIOD_ONCE        - Single request
// PERIOD_VISUAL_FRAME - Visual frame rate
// PERIOD_SECOND      - Once per second
```

### On-Demand Requests

```go
// Request data only when needed
client.RequestDataOnSimObject(
    requestID,
    definitionID,
    types.SIMOBJECT_TYPE_USER, 
    types.PERIOD_ONCE,
)
```

## Data Processing

### Safe Type Conversion

```go
case msg.IsSimObjectData():
    if data, ok := msg.GetSimObjectData(); ok {
        // Unsafe direct cast - can panic
        aircraftData := (*AircraftData)(unsafe.Pointer(&data.Data[0]))
        
        // Safe with size check
        if len(data.Data) >= int(unsafe.Sizeof(AircraftData{})) {
            aircraftData := (*AircraftData)(unsafe.Pointer(&data.Data[0]))
            processAircraftData(aircraftData)
        }
    }
```

### Handling Missing Data

```go
// Check for valid data before processing
if aircraftData.Altitude > -1000 && aircraftData.Altitude < 100000 {
    // Valid altitude range
    processAltitude(aircraftData.Altitude)
}
```

## Performance Optimization

### Minimize Data Requests

```go
// ❌ Multiple definitions for related data  
client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.DATATYPE_FLOAT64)
client.AddToDataDefinition(2, "PLANE LATITUDE", "radians", types.DATATYPE_FLOAT64)

// ✅ Single definition for related data
client.AddToDataDefinition(1, "PLANE ALTITUDE", "feet", types.DATATYPE_FLOAT64)
client.AddToDataDefinition(1, "PLANE LATITUDE", "radians", types.DATATYPE_FLOAT64)
```

### Appropriate Update Rates

```go
// ❌ Too frequent for slow-changing data
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

// ✅ Appropriate rate for fuel data
client.RequestDataOnSimObject(reqID, defID, types.SIMOBJECT_TYPE_USER, types.PERIOD_SECOND)
```
