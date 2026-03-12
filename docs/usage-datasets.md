---
title: "Using Datasets"
description: "Pre-built dataset constructors for aircraft, environment, simulator, objects, and traffic data."
order: 4
---

# Using Datasets

The `pkg/datasets` sub-packages provide ready-made `DataSet` definitions paired with companion Go structs. Instead of building data definitions field by field with `AddToDataDefinition`, you pass a constructor's return value to `RegisterDataset` and use `CastDataAs[T]` to decode incoming messages directly into the companion struct.

## How It Works

Every dataset package exposes two things:

1. A **constructor** (`New*Dataset()`) that returns `*datasets.DataSet` — a slice of `DataDefinition` entries that `RegisterDataset` feeds to SimConnect.
2. A **companion struct** (e.g., `PositionDataset`) whose fields map 1-to-1, in order, to those definitions. Pass the struct type to `CastDataAs[T]` to decode a received message.

```go
import (
    "github.com/mrlm-net/simconnect/pkg/datasets/aircraft"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    PosDefID = 1000
    PosReqID = 1001
)

// Register the dataset once, after connecting
client.RegisterDataset(PosDefID, aircraft.NewPositionDataset())

client.RequestDataOnSimObject(
    PosReqID, PosDefID,
    types.SIMCONNECT_OBJECT_ID_USER,
    types.SIMCONNECT_PERIOD_SECOND,
    types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
    0, 0, 0,
)

// Decode in the message loop
for msg := range client.Stream() {
    if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
        data := msg.AsSimObjectData()
        if data.DwRequestID == PosReqID {
            pos := engine.CastDataAs[aircraft.PositionDataset](&data.DwData)
            fmt.Printf("lat=%.4f lon=%.4f alt=%.0fft\n", pos.Latitude, pos.Longitude, pos.Altitude)
        }
    }
}
```

The same pattern applies to every dataset in this document.

## aircraft

Import path: `github.com/mrlm-net/simconnect/pkg/datasets/aircraft`

| Constructor | Companion Struct | Data |
|---|---|---|
| `NewPositionDataset()` | `PositionDataset` | Latitude, longitude, altitude, pitch, bank, true heading, vertical speed, ground speed |
| `NewAirspeedDataset()` | `AirspeedDataset` | IAS, TAS, Mach number |
| `NewEngineDataset()` | `EngineDataset` | Engine RPM (4 engines), throttle lower limit, total fuel quantity, fuel flow for engine 1 |
| `NewControlSurfacesDataset()` | `ControlSurfacesDataset` | Aileron, elevator, rudder position; flaps handle index; gear handle position |

### PositionDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `Latitude` | `PLANE LATITUDE` | degrees |
| `Longitude` | `PLANE LONGITUDE` | degrees |
| `Altitude` | `PLANE ALTITUDE` | feet |
| `Pitch` | `PLANE PITCH DEGREES` | degrees |
| `Bank` | `PLANE BANK DEGREES` | degrees |
| `HeadingTrue` | `PLANE HEADING DEGREES TRUE` | degrees |
| `VerticalSpeed` | `VERTICAL SPEED` | feet per minute |
| `GroundSpeed` | `GROUND VELOCITY` | knots |

### AirspeedDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `AirspeedIndicated` | `AIRSPEED INDICATED` | knots |
| `AirspeedTrue` | `AIRSPEED TRUE` | knots |
| `AirspeedMach` | `AIRSPEED MACH` | mach |

### EngineDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `EngRPM1` | `ENG RPM:1` | rpm |
| `EngRPM2` | `ENG RPM:2` | rpm |
| `EngRPM3` | `ENG RPM:3` | rpm |
| `EngRPM4` | `ENG RPM:4` | rpm |
| `ThrottleLowerLimit` | `THROTTLE LOWER LIMIT` | percent |
| `FuelTotalQuantity` | `FUEL TOTAL QUANTITY` | gallons |
| `FuelFlowGPH1` | `ENG FUEL FLOW GPH:1` | gallons per hour |

### ControlSurfacesDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `AileronPosition` | `AILERON POSITION` | position |
| `ElevatorPosition` | `ELEVATOR POSITION` | position |
| `RudderPosition` | `RUDDER POSITION` | position |
| `FlapsHandleIndex` | `FLAPS HANDLE INDEX` | number |
| `GearHandlePos` | `GEAR HANDLE POSITION` | bool (0.0=up, 1.0=down) |

## environment

Import path: `github.com/mrlm-net/simconnect/pkg/datasets/environment`

| Constructor | Companion Struct | Data |
|---|---|---|
| `NewWeatherDataset()` | `WeatherDataset` | Temperature, pressure, wind direction and velocity, visibility, precipitation rate and state |
| `NewTimeDataset()` | `TimeDataset` | Local time, Zulu time, simulation rate, Zulu day/month/year |

### WeatherDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `Temperature` | `AMBIENT TEMPERATURE` | celsius |
| `Pressure` | `AMBIENT PRESSURE` | millibars |
| `WindDirection` | `AMBIENT WIND DIRECTION` | degrees |
| `WindVelocity` | `AMBIENT WIND VELOCITY` | knots |
| `Visibility` | `AMBIENT VISIBILITY` | meters |
| `PrecipRate` | `AMBIENT PRECIP RATE` | millimeters of water |
| `PrecipState` | `AMBIENT PRECIP STATE` | mask (2=None, 4=Rain, 8=Snow) |

> **Note:** `PrecipState` is stored as `float64` to preserve struct alignment. Cast to `uint32` before bit-testing: `state := uint32(w.PrecipState)`.

### TimeDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `LocalTime` | `LOCAL TIME` | seconds since midnight |
| `ZuluTime` | `ZULU TIME` | seconds since midnight |
| `SimulationRate` | `SIMULATION RATE` | number |
| `ZuluDay` | `ZULU DAY OF MONTH` | number |
| `ZuluMonth` | `ZULU MONTH OF YEAR` | number |
| `ZuluYear` | `ZULU YEAR` | number |

## simulator

Import path: `github.com/mrlm-net/simconnect/pkg/datasets/simulator`

| Constructor | Companion Struct | Data |
|---|---|---|
| `NewSimStateDataset()` | `SimStateDataset` | Sim on ground, surface type, user sim flag, total weight, crash flag |
| `NewCameraDataset()` | `CameraDataset` | Camera state, camera substate, camera view type index |

### SimStateDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `SimOnGround` | `SIM ON GROUND` | bool (as float64) |
| `SurfaceType` | `SURFACE TYPE` | enum |
| `IsUserSim` | `IS USER SIM` | bool (as float64) |
| `TotalWeight` | `TOTAL WEIGHT` | pounds |
| `CrashFlag` | `CRASH FLAG` | enum bitmask |

> **Note:** `CrashFlag` is stored as `float64`. Cast to `uint32` before bit-testing.

### CameraDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `CameraState` | `CAMERA STATE` | enum |
| `CameraSubstate` | `CAMERA SUBSTATE` | enum |
| `CameraViewType` | `CAMERA VIEW TYPE INDEX:0` | number |

Camera state values correspond to the constants in `pkg/manager/state-enums.go` (e.g., `CameraStateCockpit = 2`, `CameraStateDrone = 4`). See [Manager Usage](usage-manager.md#camera-states) for the full list.

## objects

Import path: `github.com/mrlm-net/simconnect/pkg/datasets/objects`

| Constructor | Companion Struct | Data |
|---|---|---|
| `NewSimObjectPositionDataset()` | `SimObjectPositionDataset` | Latitude, longitude, altitude, pitch, bank, true heading, ground speed, sim-on-ground flag |

### SimObjectPositionDataset fields

| Field | SimVar | Unit |
|---|---|---|
| `Latitude` | `PLANE LATITUDE` | degrees |
| `Longitude` | `PLANE LONGITUDE` | degrees |
| `Altitude` | `PLANE ALTITUDE` | feet |
| `Pitch` | `PLANE PITCH DEGREES` | degrees |
| `Bank` | `PLANE BANK DEGREES` | degrees |
| `HeadingTrue` | `PLANE HEADING DEGREES TRUE` | degrees |
| `GroundSpeed` | `GROUND VELOCITY` | knots |
| `SimOnGround` | `SIM ON GROUND` | bool (as float64) |

Use this dataset with `RequestDataOnSimObjectType` to query all nearby objects in a single request:

```go
import "github.com/mrlm-net/simconnect/pkg/datasets/objects"

const (
    ObjDefID = 2000
    ObjReqID = 2001
)

client.RegisterDataset(ObjDefID, objects.NewSimObjectPositionDataset())

client.RequestDataOnSimObjectType(
    ObjReqID,
    ObjDefID,
    25000, // 25 km radius
    types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT,
)

for msg := range client.Stream() {
    if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
        data := msg.AsSimObjectDataByType()
        if data.DwRequestID == ObjReqID {
            obj := engine.CastDataAs[objects.SimObjectPositionDataset](&data.DwData)
            fmt.Printf("objectID=%d lat=%.4f lon=%.4f\n", data.DwObjectID, obj.Latitude, obj.Longitude)
        }
    }
}
```

## traffic

Import path: `github.com/mrlm-net/simconnect/pkg/datasets/traffic`

| Constructor | Companion Struct | Data |
|---|---|---|
| `NewAircraftDataset()` | `AircraftDataset` | Title, category, livery, position, attitude, speed, surface state, ATC identifiers, wing span |

This is the original reference implementation. `AircraftDataset` combines identity strings (`[128]byte`, `[32]byte`) with float64 motion fields and int32 boolean/enum fields, demonstrating the mixed-type pattern for datasets that include SimConnect string variables.

## Alignment note

All numeric fields in the companion structs use `float64`, even for variables that logically carry boolean or enum values. This is intentional: using a uniform 8-byte type for every numeric field eliminates Go struct padding between fields. The byte layout of the struct must match the byte layout that SimConnect writes into the data block, and that layout is determined entirely by the order and type of entries in the `DataSet.Definitions` slice. If you add, remove, or reorder fields in either the struct or the constructor, `CastDataAs[T]` will silently misalign — the compiler cannot catch this.
