---
title: "Request and ID Management"
description: "ID allocation strategy and conflict prevention."
order: 6
section: "internals"
---

# Manager Request and ID Management

This document explains how the manager package handles SimConnect requests and ID allocation to prevent conflicts and enable request tracking.

## ID Allocation Strategy

The manager uses a **high-number ID reservation strategy** to maximize flexibility for user applications:

### ID Ranges

| Range | Owner | Count | Purpose |
|-------|-------|-------|---------|
| 1 - 999,999,849 | **User Applications** | 999,999,849 | User-defined data definitions and requests |
| 999,999,850 - 999,999,886 | **Manager** | 37 | Custom system event IDs (dynamic allocation) |
| 999,999,887 - 999,999,899 | **Reserved** | 13 | Unallocated buffer for future use |
| 999,999,900 - 999,999,999 | **Manager** | 100 | Internal manager operations (reserved) |

### Why High Numbers for Manager?

1. **User-Friendly**: Allows applications to use IDs starting from 1 without conflict
2. **Defensive**: High numbers are unlikely to be chosen by developers
3. **Clear Boundaries**: Easy to identify and validate ID ownership
4. **Scalable**: Provides >999M IDs for user applications

## Current Manager IDs

### Simulator State System

```go
CameraDefinitionID = 999999900  // Camera/simulator state data definition
CameraRequestID    = 999999901  // Periodic camera state data polling
```

**Purpose**: Continuously polls simulator state (camera, simulation, environment, aircraft telemetry) to update the manager's `SimState`.

**Usage**: Internal to the manager. Not directly accessible to users but affects `OnSimStateChange` notifications.

The manager's simulator state data definition now includes additional environment, simulation, and aircraft telemetry variables which are exposed on `SimState`:

- `SIMULATION RATE` (Number) — internal rate of passing time
- `SIMULATION TIME` (Seconds) — seconds since the simulation started
- `LOCAL TIME` (Seconds) — seconds since local midnight
- `ZULU TIME` (Seconds) — seconds since Zulu (UTC) midnight
- `LOCAL DAY OF MONTH`, `LOCAL MONTH OF YEAR`, `LOCAL YEAR` (Number) — local date
- `ZULU DAY OF MONTH`, `ZULU MONTH OF YEAR`, `ZULU YEAR` (Number) — Zulu date
- `IS IN VR`, `IS USING MOTION CONTROLLERS`, `IS USING JOYSTICK THROTTLE`, `IS IN RTC`, `IS AVATAR`, `IS AIRCRAFT` (Boolean) — environment flags
- `AMBIENT TEMPERATURE` (Celsius), `AMBIENT PRESSURE` (inHg), `AMBIENT WIND VELOCITY` (Knots)
- `AMBIENT WIND DIRECTION` (Degrees), `AMBIENT VISIBILITY` (Meters), `AMBIENT IN CLOUD` (Boolean)
- `AMBIENT PRECIP STATE` (Mask), `BAROMETER PRESSURE` (Millibars), `SEA LEVEL PRESSURE` (Millibars)
- `GROUND ALTITUDE` (Feet), `MAGVAR` (Degrees), `SURFACE TYPE` (Enum)
- `PLANE LATITUDE` (Degrees), `PLANE LONGITUDE` (Degrees), `PLANE ALTITUDE` (Feet), `INDICATED ALTITUDE` (Feet)
- `PLANE HEADING DEGREES TRUE` (Degrees), `PLANE HEADING DEGREES MAGNETIC` (Degrees)
- `PLANE PITCH DEGREES` (Degrees), `PLANE BANK DEGREES` (Degrees)
- `GROUND VELOCITY` (Knots), `AIRSPEED INDICATED` (Knots), `AIRSPEED TRUE` (Knots), `VERTICAL SPEED` (Feet per minute)
- `SMART CAMERA ACTIVE` (Boolean)
- `HAND ANIM STATE` (Number, 0-12 frame IDs), `HIDE AVATAR IN AIRCRAFT` (Boolean), `MISSION SCORE` (Number), `PARACHUTE OPEN` (Boolean)
- `ZULU SUNRISE TIME` (Seconds), `ZULU SUNSET TIME` (Seconds), `TIME ZONE OFFSET` (Seconds)
- `TOOLTIP UNITS` (Enum: 0=Default, 1=Metric, 2=US), `UNITS OF MEASURE` (Enum: 0=English, 1=Metric/feet, 2=Metric/meters)
- `AMBIENT IN SMOKE` (Boolean), `ENV SMOKE DENSITY` (Percent), `ENV CLOUD DENSITY` (Percent)
- `DENSITY ALTITUDE` (Feet), `SEA LEVEL AMBIENT TEMPERATURE` (Celsius)

### Event System (manager-reserved IDs)

The manager reserves specific high-number IDs for internal system event subscriptions. These are registered on connection open and used for request tracking; the actual SimConnect system event names are standard (e.g. "Pause", "Sim", "FlightLoaded").

```go
PauseEventID                 = 999999998 // Pause/unpause event subscription
SimEventID                   = 999999997 // Sim start/stop event subscription
FlightLoadedEventID          = 999999996 // Flight file loaded (filename returned)
AircraftLoadedEventID        = 999999995 // Aircraft file loaded/changed (.AIR)
ObjectAddedEventID           = 999999994 // AI object added
ObjectRemovedEventID         = 999999993 // AI object removed
FlightPlanActivatedEventID   = 999999992 // Flight plan activated (filename returned)
CrashedEventID               = 999999991 // Simulator crashed (manager reserved ID)
CrashResetEventID            = 999999990 // Crash reset event (manager reserved ID)
SoundEventID                 = 999999989 // Sound event (manager reserved ID)
ViewEventID                  = 999999988 // View camera event (manager reserved ID)
FlightPlanDeactivatedEventID = 999999987 // Flight plan deactivated
```

The engine's heartbeat subscription also uses an ID in this range: `engine.HEARTBEAT_EVENT_ID = 999999999`.

**Purpose**: These IDs are used to register and track the manager's internal subscriptions to SimConnect system events. Responses may arrive as different `SIMCONNECT_RECV` variants (e.g., `SIMCONNECT_RECV_EVENT`, `SIMCONNECT_RECV_EVENT_FILENAME`, `SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE`).

**Usage**: Internal to the manager. The manager updates `SimState` for Pause/Sim events and provides typed subscription helpers for filename/object events.

### Custom Event System (manager-reserved ID range)

The manager reserves a range of IDs for user-defined custom system events:

```go
CustomEventIDMin = 999999850 // First ID for custom events
CustomEventIDMax = 999999886 // Last ID for custom events (37 slots total)
```

**Purpose**: These IDs are dynamically allocated when users subscribe to custom SimConnect system events by name (e.g., "6Hz", "1sec"). Custom events use the `SubscribeToCustomSystemEvent` and `OnCustomSystemEvent` APIs.

**Usage**: Managed internally by the manager. Custom event subscriptions are cleared by `Stop()`; over a lost connection they are kept and subscribed again with the same IDs on the next one.

## Library default ID ranges

Besides the manager's reserved range, several packages use fixed default IDs when the application does not pass its own. Definition, request and client event IDs are separate SimConnect namespaces; a range only collides with IDs of the same kind. Ranges below are the defaults (base constant plus the count in the code).

| Package / type | What | Definition IDs | Request IDs | Event IDs |
|---|---|---|---|---|
| `airport.Loader` | airport facilities | 7100–7106 (`DefaultLoaderDefinitionBase`, 7 `loaderDefinitions`) | 7200–7311 (`DefaultLoaderRequestBase`, `loaderSlots` 16 × 7) | — |
| `traffic.TaxiController` | taxiing aircraft | 7300–7301 (`DefaultTaxiDefinitionBase`, `taxiDefinitionCount` 2) | 7400–7403 (`DefaultTaxiRequestBase`, `taxiRequestCount` 4) | — |
| `traffic.ArrivalController` | arriving aircraft | 7500–7503 (`DefaultArrivalDefinitionBase`, `arrDef*` 4) | 7600–7603 (`DefaultArrivalRequestBase`, `arrReq*` 4) | — |
| `traffic.Injector` | injected aircraft and tugs | 7700–7709 (`DefaultInjectDefinitionBase`, up to `injDefGearDown`) | 7800–7991 (`DefaultInjectRequestBase`, 2 × `injectMaxAircraft` 96) | 7900–7909 (`DefaultInjectEventBase`, `injectEventCount` 10) |
| `traffic.StandAllocator` | stand scan | 8200 (`DefaultStandDefinitionBase`) | 8300–8301 (`DefaultStandRequestBase`, `standReqAircraft`, `standReqUser`) | — |
| `airport.ProcedureLoader` | SIDs, STARs, approaches | 8400–8402 (`DefaultProcedureDefinitionBase`, `procedureParts` 3) | 8500–8523 (`DefaultProcedureRequestBase`, `procedureSlots` 8 × 3) | — |
| `nav.NavLoader` | waypoints, VORs, NDBs | 8700–8702 (`DefaultNavDefinitionBase`, 3 `navDefinitions`) | 8800–8831 (`DefaultNavRequestBase`, `DefaultNavSlots` 16 × `navParts` 2) | — |
| `traffic.AirportLister` | airports in the reality bubble | — | 8900 (`DefaultAirportListRequestID`) | — |
| `avionics.Radios` | COM and transponder | — | — | 0x7A00–0x7A09 = 31232–31241 (`DefaultEventBase`, `eventCount` 10) |
| `systems.Controls` | doors, chocks, GPU, brake | 0x7B00–0x7B3F = 31488–31551 (`DefaultControlBase`, block of 64) | — | 0x7B00–0x7B3F (same block) |
| `manager` | custom system events | — | — | 999999850–999999886 (`CustomEventIDMin`–`CustomEventIDMax`) |
| `manager` | system events | — | — | 999999987–999999998 (`FlightPlanDeactivatedEventID`–`PauseEventID`) |
| `manager` | simulator state | 999999900 (`CameraDefinitionID`) | 999999901 (`CameraRequestID`) | — |
| `engine` | heartbeat | — | — | 999999999 (`HEARTBEAT_EVENT_ID`) |

`nav.WeatherReader`, `systems.Reader`, `traffic.ProfileReader`, the tugs and fuel trucks take their IDs from the application and have no defaults. Pick application IDs outside all of the ranges above (the examples use 10000/10001 for the weather reader), or move a library type off its defaults (`LoaderWithIDs`, `TaxiWithIDs`, `ArrivalWithIDs`, `InjectorWithIDs`, `StandWithIDs`, `NewNavLoaderWithIDs`, the `reqID` of `NewAirportLister`, the `base` of `avionics.New` and `systems.NewControls`). `ProcedureLoader` always uses its defaults.

## Request Registry

The manager maintains a `RequestRegistry` that tracks all active SimConnect requests. This enables:

- **Request Correlation**: Match responses to the original requests
- **Diagnostics**: View all outstanding requests at any time
- **Conflict Detection**: Validate user IDs before use
- **Cleanup**: Release resources when requests complete

### RequestRegistry API

```go
// Create a registry
registry := NewRequestRegistry()

// Register a request when making a SimConnect call
info := registry.Register(1000, RequestTypeDataDefinition, "My Data Definition")

// Optionally add custom context for tracking
info.Context["purpose"] = "tracking_aircraft"

// Later, check if a response matches a known request
if info, exists := registry.Get(1000); exists {
    // Response is valid for request ID 1000
    purpose := info.Context["purpose"]
}

// When request completes, unregister it
registry.Unregister(1000)

// Check all outstanding requests
pending := registry.GetAll()
fmt.Printf("Outstanding requests: %d\n", registry.Count())

// Clear on disconnect
registry.Clear()
```

### RequestType Constants

```go
RequestTypeDataDefinition  // AddToDataDefinition, ClearDataDefinition
RequestTypeDataRequest     // RequestDataOnSimObject, RequestDataOnSimObjectType
RequestTypeDataSet         // SetDataOnSimObject
RequestTypeEvent           // SubscribeToSystemEvent, UnsubscribeFromSystemEvent
RequestTypeObject          // RequestSimulatorState, RequestFacilities, etc.
RequestTypeCustom          // User-defined or other request types
```

## Best Practices for Users

### 1. Choose Your ID Range

Pick a sub-range within 1-999,999,849 for your application:

```go
const (
    // Aircraft data definitions
    AircraftPositionDef = 1000
    AircraftVelocityDef = 1001
    
    // Environment data definitions
    WeatherDataDef = 2000
    WindDataDef    = 2001
)
```

### 2. Validate Before Use

Use the provided validation functions:

```go
if !manager.IsValidUserID(myID) || myID >= manager.CustomEventIDMin {
    return fmt.Errorf("invalid user ID: %d (reserved for manager)", myID)
}
```

`IsValidUserID` accepts IDs up to 999,999,899 (`IDRange.UserMax`), so it does not catch the custom-event range 999,999,850–999,999,886. Keep your IDs at or below 999,999,849.

### 3. Document Your ID Assignments

Keep a clear mapping of your IDs:

```go
// MyApp IDs
// 1000-1099: Aircraft data
// 2000-2099: Environment data
// 3000-3099: Traffic data
```

### 4. Use the Request Registry (Optional)

If you need to track user-initiated requests, access the manager's registry via the `Client()` method:

```go
client := mgr.Client()
// Note: Current implementation doesn't expose registry to users
// This is a future enhancement point
```

## ID Validation Helpers

The manager provides utility functions to validate IDs:

```go
// Check if an ID is reserved for manager use
if manager.IsManagerID(999999900) {
    fmt.Println("This ID is reserved for the manager")
}

// Check if an ID is within the user range
if manager.IsValidUserID(1000) {
    fmt.Println("This ID is available for user requests")
}
```

## Conflict Resolution

If your application accidentally uses a manager-reserved ID:

1. **On Connection**: The manager will register its internal requests (999999900, 999999901, 999999998)
2. **Potential Conflict**: If you use the same ID, your definition will be overwritten
3. **Detection**: Check the manager logs or use `IsManagerID()` in validation

**Prevention**: Always validate your IDs against the manager's reserved range before use.

## Future Enhancements

Potential improvements for request management:

1. **User Request Tracking**: Expose registry interface to users for tracking custom requests
2. **ID Allocation Pool**: Automatic ID assignment for user requests
3. **Request Timeout Handling**: Automatic cleanup of abandoned requests
4. **Exception Correlation**: Link failed requests to exception messages
5. **Configurable Manager Range**: Allow users to specify different ID ranges for the manager

## Internal Implementation Details

### Registration Points

Manager registers internal requests at these points:

1. **On OPEN (via `processMessage` → `registerSimStateSubscriptions`)**:
    - Simulator State Definition (999999900) — registers camera state, simulation/time variables, date fields, IS_* flags, environment SimVars, aircraft telemetry, and extended variables
    - Simulator State Request (999999901)
    - Pause Event (999999998)
    - Crashed/CrashReset/Sound event subscriptions (manager reserved IDs listed above)
    - Custom system events kept from a lost connection, subscribed again with their IDs (`resubscribeCustomEvents`)

2. **On a lost connection (via `connectionLost`, the stream closed after QUIT or a drop)**:
   - The engine disconnected: the SimConnect handle closed
   - All requests cleared via `requestRegistry.Clear()`
   - Custom system events kept

3. **On Stop (via `disconnect`)**:
   - All requests cleared via `requestRegistry.Clear()`
   - Custom system events cleared, their ID allocator reset

### Request Types Used by Manager

- `RequestTypeEvent`: Pause event subscription
- `RequestTypeDataDefinition`: Camera state definition
- `RequestTypeDataRequest`: Camera state periodic request

### Cleanup Strategy

When the manager stops (`disconnect`):
1. Clear the simulator state data definition (if its request was submitted)
2. Disconnect the engine
3. Clear custom system events and reset their ID allocator
4. Clear all entries in request registry
5. Reset `cameraDataRequestPending` flag

When the connection is lost (`connectionLost`): the same, except that the data definition is not cleared (the link is gone) and the custom system events are kept, to be subscribed again with the same IDs on the next OPEN. An application need not subscribe to them again after a reconnect; each `SubscribeToCustomSystemEvent` call returns a subscription of its own.
