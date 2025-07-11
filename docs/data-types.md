# Data Types Reference

This document provides a comprehensive reference for all data types used in the SimConnect Go library, including how SimConnect types map to Go types.

## Table of Contents

- [Basic Data Types](#basic-data-types)
- [SimConnect Enumerations](#simconnect-enumerations)
- [Structure Types](#structure-types)
- [Type Conversions](#type-conversions)
- [Units and Precision](#units-and-precision)

## Basic Data Types

### SIMCONNECT_DATATYPE

SimConnect defines specific data types for variables. These map to Go types as follows:

| SimConnect Type | Go Type | Description | Example Use |
|----------------|---------|-------------|-------------|
| `SIMCONNECT_DATATYPE_INVALID` | - | Invalid/undefined type | Error conditions |
| `SIMCONNECT_DATATYPE_INT32` | `int32` | 32-bit signed integer | Counters, IDs |
| `SIMCONNECT_DATATYPE_INT64` | `int64` | 64-bit signed integer | Large counters |
| `SIMCONNECT_DATATYPE_FLOAT32` | `float32` | 32-bit floating point | Basic measurements |
| `SIMCONNECT_DATATYPE_FLOAT64` | `float64` | 64-bit floating point | High precision values |
| `SIMCONNECT_DATATYPE_STRING8` | `[8]byte` | 8-character string | Short identifiers |
| `SIMCONNECT_DATATYPE_STRING32` | `[32]byte` | 32-character string | Names, codes |
| `SIMCONNECT_DATATYPE_STRING64` | `[64]byte` | 64-character string | Descriptions |
| `SIMCONNECT_DATATYPE_STRING128` | `[128]byte` | 128-character string | Long descriptions |
| `SIMCONNECT_DATATYPE_STRING256` | `[256]byte` | 256-character string | Paths, URLs |
| `SIMCONNECT_DATATYPE_STRING260` | `[260]byte` | 260-character string | Windows file paths |
| `SIMCONNECT_DATATYPE_STRINGV` | `[]byte` | Variable length string | Dynamic content |

### Common Go Type Mappings

```go
// Aircraft data structure example
type AircraftData struct {
    Altitude    float64 // SIMCONNECT_DATATYPE_FLOAT64
    Speed       float64 // SIMCONNECT_DATATYPE_FLOAT64
    Heading     float64 // SIMCONNECT_DATATYPE_FLOAT64
    Gear        int32   // SIMCONNECT_DATATYPE_INT32 (0/1 boolean)
    Flaps       int32   // SIMCONNECT_DATATYPE_INT32
}
```

## SimConnect Enumerations

### Object Types

```go
type SIMCONNECT_SIMOBJECT_TYPE uint32

const (
    SIMCONNECT_SIMOBJECT_TYPE_USER     SIMCONNECT_SIMOBJECT_TYPE = 0
    SIMCONNECT_SIMOBJECT_TYPE_ALL      SIMCONNECT_SIMOBJECT_TYPE = 1
    SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT SIMCONNECT_SIMOBJECT_TYPE = 2
    SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER SIMCONNECT_SIMOBJECT_TYPE = 3
    SIMCONNECT_SIMOBJECT_TYPE_BOAT     SIMCONNECT_SIMOBJECT_TYPE = 4
    SIMCONNECT_SIMOBJECT_TYPE_GROUND   SIMCONNECT_SIMOBJECT_TYPE = 5
)
```

### Data Request Periods

```go
type SIMCONNECT_PERIOD uint32

const (
    SIMCONNECT_PERIOD_NEVER           SIMCONNECT_PERIOD = 0
    SIMCONNECT_PERIOD_ONCE            SIMCONNECT_PERIOD = 1
    SIMCONNECT_PERIOD_VISUAL_FRAME    SIMCONNECT_PERIOD = 2
    SIMCONNECT_PERIOD_SIM_FRAME       SIMCONNECT_PERIOD = 3
    SIMCONNECT_PERIOD_SECOND          SIMCONNECT_PERIOD = 4
)
```

### Data Request Flags

```go
type SIMCONNECT_DATA_REQUEST_FLAG uint32

const (
    SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT = 0
    SIMCONNECT_DATA_REQUEST_FLAG_CHANGED = 1
    SIMCONNECT_DATA_REQUEST_FLAG_TAGGED  = 2
)
```

## Structure Types

### Base Message Structure

```go
type SIMCONNECT_RECV struct {
    DwSize    uint32                 // Size of the structure
    DwVersion uint32                 // Version of the structure
    DwID      SIMCONNECT_RECV_ID     // Message type identifier
}
```

### Simulation Object Data

```go
type SIMCONNECT_RECV_SIMOBJECT_DATA struct {
    SIMCONNECT_RECV                   // Inherits base structure
    DwRequestID        uint32         // Request identifier
    DwObjectID         uint32         // Object identifier
    DwDefineID         uint32         // Data definition identifier
    DwFlags            uint32         // Data flags
    Dwentrynumber      uint32         // Entry number
    DwoutOf            uint32         // Total entries
    DwDefineCount      uint32         // Number of defined variables
    DwData             [1]uint32      // Variable length data array
}
```

### Event Data

```go
type SIMCONNECT_RECV_EVENT struct {
    SIMCONNECT_RECV           // Inherits base structure
    DwGroupID    uint32       // Group identifier
    DwEventID    uint32       // Event identifier
    DwData       uint32       // Event data
}
```

### Exception Data

```go
type SIMCONNECT_RECV_EXCEPTION struct {
    SIMCONNECT_RECV           // Inherits base structure
    DwException  uint32       // Exception code
    DwSendID     uint32       // Send identifier
    DwIndex      uint32       // Parameter index (if applicable)
}
```

## Type Conversions

### String Handling

SimConnect strings are fixed-length byte arrays, often null-terminated:

```go
// Convert SimConnect string to Go string
func simConnectStringToGo(data [260]byte) string {
    // Find null terminator
    length := 0
    for i, b := range data {
        if b == 0 {
            length = i
            break
        }
    }
    return string(data[:length])
}

// Convert Go string to SimConnect format
func goStringToSimConnect(s string, size int) []byte {
    result := make([]byte, size)
    copy(result, []byte(s))
    return result
}
```

### Coordinate Conversions

Many SimConnect values are in radians but need to be displayed in degrees:

```go
func radiansToDegrees(radians float64) float64 {
    return radians * 180.0 / math.Pi
}

func degreesToRadians(degrees float64) float64 {
    return degrees * math.Pi / 180.0
}

// Example usage
latitude := radiansToDegrees(aircraftData.Latitude)
longitude := radiansToDegrees(aircraftData.Longitude)
```

### Boolean Values

SimConnect uses numeric values for boolean flags:

```go
// Convert SimConnect boolean to Go bool
func simConnectBool(value float64) bool {
    return value != 0
}

// Example usage
isOnGround := simConnectBool(aircraftData.OnGround)
gearDown := simConnectBool(aircraftData.GearPosition)
```

## Units and Precision

### Common Units

| Variable Type | Common Units | Conversion Notes |
|--------------|--------------|------------------|
| **Distance** | `feet`, `meters`, `nautical miles` | Default: feet |
| **Speed** | `knots`, `feet per second`, `meters per second` | Default: knots |
| **Angle** | `degrees`, `radians` | Default: radians |
| **Weight** | `pounds`, `kilograms` | Default: pounds |
| **Temperature** | `celsius`, `fahrenheit`, `kelvin` | Default: celsius |
| **Pressure** | `inches of mercury`, `millibars` | Default: inHg |
| **Time** | `seconds`, `minutes`, `hours` | Default: seconds |

### Unit Examples

```go
// Define aircraft position data with specific units
sc.AddToDataDefinition(POSITION_DEF, "PLANE LATITUDE", "degrees", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
sc.AddToDataDefinition(POSITION_DEF, "PLANE LONGITUDE", "degrees", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
sc.AddToDataDefinition(POSITION_DEF, "PLANE ALTITUDE", "feet", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)

// Speed data with different units
sc.AddToDataDefinition(SPEED_DEF, "GROUND VELOCITY", "knots", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
sc.AddToDataDefinition(SPEED_DEF, "VERTICAL SPEED", "feet per minute", 
    types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
```

### Precision Considerations

- **Float32 vs Float64**: Use `float64` for high-precision calculations
- **Integer Types**: Use `int32` for counters and flags
- **String Sizes**: Choose appropriate string sizes to avoid truncation
- **Update Frequency**: Balance precision needs with performance

### Working with Raw Data

When working with the raw data from `SIMCONNECT_RECV_SIMOBJECT_DATA`:

```go
func extractDataFromMessage(msg *types.SIMCONNECT_RECV_SIMOBJECT_DATA, dataStruct interface{}) error {
    // Calculate data start position
    dataStart := unsafe.Sizeof(*msg) - unsafe.Sizeof(msg.DwData)
    dataPtr := unsafe.Pointer(uintptr(unsafe.Pointer(msg)) + dataStart)
    
    // Copy data to your structure
    dataSize := unsafe.Sizeof(dataStruct)
    copy((*[1 << 30]byte)(unsafe.Pointer(&dataStruct))[:dataSize], 
         (*[1 << 30]byte)(dataPtr)[:dataSize])
    
    return nil
}
```

This reference provides the foundation for understanding how SimConnect data maps to Go types and how to work with them effectively in your applications.
