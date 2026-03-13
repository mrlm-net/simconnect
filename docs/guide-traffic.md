---
title: "AI Traffic"
description: "Inject and manage AI aircraft using the Engine client AI object API."
order: 7
section: "client"
---

# AI Traffic

This guide covers AI aircraft injection using the Engine client's `AICreate*` methods — thin, typed wrappers over the raw SimConnect DLL. Every call maps directly to a single SimConnect API function with no extra abstraction.

> **Scope:** This guide covers the raw Engine-layer `AICreate*` / `AIRemove*` / `AIRelease*` API. Manager-layer wrappers (`TrafficParked`, `TrafficEnroute`, `TrafficNonATC`, fleet tracking) and the `pkg/traffic.Fleet` abstraction are out of scope — see [traffic-guide.md](traffic-guide.md) for those.

## Aircraft Kinds

SimConnect exposes four creation APIs, each suited to a different role:

| Kind | Method | When to use |
|------|--------|-------------|
| **Parked ATC** | `AICreateParkedATCAircraft` | Static ground traffic at a known airport. SimConnect places the aircraft at a free parking spot. No position required. |
| **Enroute ATC** | `AICreateEnrouteATCAircraft` | Aircraft mid-flight following an MSFS `.pln` flight plan. SimConnect positions the aircraft along the route at the given progress fraction. |
| **Non-ATC** | `AICreateNonATCAircraft` | Aircraft at an explicit world position. Required when you need precise gate/apron placement or airborne injection outside the ATC system. |
| **Simulated Object** | `AICreateSimulatedObject` | Non-aircraft objects: ground vehicles, boats, or any container title that is not an aircraft. |

All four return an object ID asynchronously via `SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID`.

## The Create-Acknowledge Pattern

Every `AICreate*` call is fire-and-forget from the caller's perspective. You supply a `requestID` that you chose; SimConnect responds with `SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID` carrying both your `requestID` and the live `objectID`. Store that `objectID` — it is what you need for every subsequent operation on the object.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const reqCreate uint32 = 1000

// Submit the create call — objectID is not yet known.
client.AICreateParkedATCAircraft("FSLTL A320 Air France SL", "N12345", "EGLL", reqCreate)

// In the message loop, wait for the acknowledgement.
for msg := range client.Stream() {
    switch types.SIMCONNECT_RECV_ID(msg.DwID) {

    case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
        assigned := msg.AsAssignedObjectID()
        if uint32(assigned.DwRequestID) == reqCreate {
            objectID := uint32(assigned.DwObjectID)
            fmt.Printf("Aircraft spawned: objectID=%d\n", objectID)
            // objectID is now valid for AIRemoveObject, AIReleaseControl, etc.
        }
    }
}
```

`AsAssignedObjectID()` returns `nil` when the message is not of that type, so the switch-case guard is sufficient — no additional nil-check is needed inside the `case` block.

## Parked ATC Aircraft

`AICreateParkedATCAircraft` places an aircraft at a free parking spot at the given airport. SimConnect selects the spot; you do not control the exact gate or ramp position.

**Signature:**

```go
func (e *Engine) AICreateParkedATCAircraft(
    szContainerTitle string, // Aircraft model container title (exact match required)
    szTailNumber     string, // Tail / registration number shown in cockpit and ATC
    szAirportID      string, // ICAO airport code
    RequestID        uint32, // Your request ID — returned in ASSIGNED_OBJECT_ID
) error
```

**Example — spawn a parked aircraft at Prague:**

```go
//go:build windows

package main

import (
    "context"
    "fmt"

    simconnect "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const (
    reqPark uint32 = 1000
)

func main() {
    ctx := context.Background()
    client := simconnect.NewClient("ParkedTraffic", engine.WithContext(ctx))
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    // Queue the create call — SimConnect places the aircraft at a free spot.
    client.AICreateParkedATCAircraft(
        "FSLTL A320 Air France SL", // container title
        "AFR123",                   // tail number
        "LKPR",                     // ICAO
        reqPark,
    )

    for msg := range client.Stream() {
        switch types.SIMCONNECT_RECV_ID(msg.DwID) {

        case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
            a := msg.AsAssignedObjectID()
            if uint32(a.DwRequestID) == reqPark {
                fmt.Printf("Parked: objectID=%d\n", uint32(a.DwObjectID))
            }

        case types.SIMCONNECT_RECV_ID_EXCEPTION:
            ex := msg.AsException()
            fmt.Printf("SimConnect exception %d at index %d\n",
                ex.DwException, ex.DwIndex)
        }
    }
}
```

> **Note:** The container title must exactly match a model installed in the simulator. The `EnumerateSimObjectsAndLiveries` API, covered below, lets you discover valid titles at runtime.

## Enroute ATC Aircraft

`AICreateEnrouteATCAircraft` injects an aircraft into the ATC system mid-flight, following an MSFS `.pln` flight plan. SimConnect positions the aircraft at the given fractional position along the route.

**Signature:**

```go
func (e *Engine) AICreateEnrouteATCAircraft(
    szContainerTitle  string,  // Aircraft model container title
    szTailNumber      string,  // Tail / registration number
    iFlightNumber     uint32,  // ATC flight number (numeric part)
    szFlightPlanPath  string,  // Absolute path to an MSFS .pln file
    dFlightPlanPosition float64, // Route progress: 0.0 = start, 1.0 = destination
    bTouchAndGo      bool,    // Whether the aircraft performs touch-and-go landings
    RequestID        uint32,  // Your request ID
) error
```

**Example — inject an aircraft at the beginning of a plan:**

```go
//go:build windows

package main

import (
    "fmt"
    "path/filepath"

    simconnect "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const reqEnroute uint32 = 1001

func spawnEnroute(client engine.Client, planDir string) {
    client.AICreateEnrouteATCAircraft(
        "FSLTL A320 Air France SL",
        "AFR456",
        456,
        filepath.Join(planDir, "LKPREDDN_MFS_NoProc.pln"),
        0.0,   // inject at the start of the plan
        false, // not touch-and-go
        reqEnroute,
    )
}

func handleMessages(client engine.Client) {
    for msg := range client.Stream() {
        switch types.SIMCONNECT_RECV_ID(msg.DwID) {

        case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
            a := msg.AsAssignedObjectID()
            if uint32(a.DwRequestID) == reqEnroute {
                fmt.Printf("Enroute aircraft: objectID=%d\n", uint32(a.DwObjectID))
            }
        }
    }
}

func main() {
    client := simconnect.NewClient("EnrouteTraffic")
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    spawnEnroute(client, `C:\MSFS-TEST-PLANS`)
    handleMessages(client)
}
```

> **Note:** The flight plan path must be an absolute Windows path to an MSFS-compatible `.pln` file. Relative paths are not supported by the SimConnect API.

## Non-ATC Aircraft at an Explicit Position

`AICreateNonATCAircraft` gives you full control over initial position, heading, altitude, and airspeed. Use this when you need to place an aircraft at a specific gate, apron stand, or airborne location that is not managed by ATC.

**Signature:**

```go
func (e *Engine) AICreateNonATCAircraft(
    szContainerTitle string,
    szTailNumber     string,
    initPos          types.SIMCONNECT_DATA_INITPOSITION,
    RequestID        uint32,
) error
```

**`SIMCONNECT_DATA_INITPOSITION` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Latitude` | `float64` | Decimal degrees, positive = north |
| `Longitude` | `float64` | Decimal degrees, positive = east |
| `Altitude` | `float64` | Feet MSL (not meters) |
| `Pitch` | `float64` | Degrees, positive = nose up |
| `Bank` | `float64` | Degrees, positive = right wing down |
| `Heading` | `float64` | True degrees (0–360) |
| `OnGround` | `DWORD` | 1 = on ground, 0 = airborne |
| `Airspeed` | `DWORD` | Knots; use `SIMCONNECT_DATA_INITPOSITION_AIRSPEED_CRUISE` (-1) for cruise speed |

**Example — spawn an aircraft at a specific gate:**

```go
//go:build windows

package main

import (
    "fmt"

    simconnect "github.com/mrlm-net/simconnect"
    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const reqNonATC uint32 = 1002

func main() {
    client := simconnect.NewClient("NonATCTraffic")
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Disconnect()

    // Precise gate coordinates obtained from facility data.
    client.AICreateNonATCAircraft(
        "FSLTL A320 Air France SL",
        "N99001",
        types.SIMCONNECT_DATA_INITPOSITION{
            Latitude:  50.1006, // gate lat (decimal degrees)
            Longitude: 14.2601, // gate lon
            Altitude:  1247,    // feet MSL (Prague elevation)
            Heading:   245,     // facing apron
            OnGround:  1,       // parked
            Airspeed:  0,
        },
        reqNonATC,
    )

    for msg := range client.Stream() {
        if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
            a := msg.AsAssignedObjectID()
            if uint32(a.DwRequestID) == reqNonATC {
                fmt.Printf("Non-ATC aircraft: objectID=%d\n", uint32(a.DwObjectID))
            }
        }
    }
}
```

## EX1 Variants — Livery Selection

The `EX1` variants of all three aircraft creation methods add a `szLivery` parameter that lets you specify an exact livery by name rather than accepting the model's default.

```go
func (e *Engine) AICreateParkedATCAircraftEX1(
    szContainerTitle string,
    szLivery         string, // Exact livery name; empty string = default livery
    szTailNumber     string,
    szAirportID      string,
    RequestID        uint32,
) error

func (e *Engine) AICreateEnrouteATCAircraftEX1(
    szContainerTitle    string,
    szLivery            string,
    szTailNumber        string,
    iFlightNumber       uint32,
    szFlightPlanPath    string,
    dFlightPlanPosition float64,
    bTouchAndGo        bool,
    RequestID          uint32,
) error

func (e *Engine) AICreateNonATCAircraftEX1(
    szContainerTitle string,
    szLivery         string,
    szTailNumber     string,
    initPos          types.SIMCONNECT_DATA_INITPOSITION,
    RequestID        uint32,
) error
```

**Example — parked aircraft with an explicit livery:**

```go
//go:build windows

client.AICreateParkedATCAircraftEX1(
    "Boeing 787-10 Asobo",
    "House",   // livery name as returned by EnumerateSimObjectsAndLiveries
    "BOE787",
    "KSEA",
    1003,
)
```

Pass an empty string (`""`) for `szLivery` to use the model default. The `EX1` variants are MSFS 2024 additions; on MSFS 2020 they are available from SimConnect SDK version 11 onwards.

## Object Management

### Removing an Object

`AIRemoveObject` destroys an AI object and removes it from the simulation. The `requestID` is for your own correlation; no meaningful acknowledgement is returned for removal.

```go
//go:build windows

if err := client.AIRemoveObject(objectID, reqRemove); err != nil {
    // handle DLL call failure
}
```

Call this during shutdown to clean up all spawned objects before disconnecting.

### Releasing Control

`AIReleaseControl` transfers ownership of an AI object from your add-on back to the simulator's AI system. After release, the simulator drives the object autonomously. You can still send waypoints via `SetDataOnSimObject` before releasing if you want to set an initial route.

```go
//go:build windows

// Release AI control — simulator takes over.
client.AIReleaseControl(objectID, reqRelease)
```

This is the correct sequence when you want the aircraft to taxi and depart autonomously after providing a waypoint set.

### Assigning a Flight Plan

`AISetAircraftFlightPlan` assigns or replaces the flight plan on an existing AI aircraft. The object must already exist (use the `objectID` from `ASSIGNED_OBJECT_ID`).

```go
//go:build windows

client.AISetAircraftFlightPlan(
    objectID,
    `C:\MSFS-TEST-PLANS\LKPREDDN_MFS_NoProc.pln`,
    reqFlightPlan,
)
```

> **Note:** `AISetAircraftFlightPlan` does not return an `ASSIGNED_OBJECT_ID`. The `requestID` parameter is present for symmetry with the rest of the AI API but SimConnect does not dispatch a response message for this call.

## Model Discovery

Before spawning aircraft you need valid container titles and livery names. `EnumerateSimObjectsAndLiveries` requests the full list of installed models for a given object type.

**Requesting the enumeration:**

```go
//go:build windows

const reqEnum uint32 = 9000

client.EnumerateSimObjectsAndLiveries(reqEnum, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
```

**Receiving the results:**

SimConnect returns one or more `SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST` messages. Each contains an array of `SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY` entries.

```go
//go:build windows

case types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST:
    list := msg.AsSimObjectAndLiveryEnumeration()
    // list is *types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST
    for _, entry := range list.RgData {
        title := engine.BytesToString(entry.AircraftTitle[:])
        livery := engine.BytesToString(entry.LiveryName[:])
        fmt.Printf("Title: %q  Livery: %q\n", title, livery)
    }
```

`AsSimObjectAndLiveryEnumeration()` returns `nil` when the message is not of that type. The `RgData` slice is populated from the wire data and contains all entries in the current batch. SimConnect may send multiple messages for large model sets; collect them all before using the results.

**Object type constants** for `EnumerateSimObjectsAndLiveries`:

| Constant | Value | Description |
|----------|-------|-------------|
| `SIMCONNECT_SIMOBJECT_TYPE_ALL` | 1 | All installed objects |
| `SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT` | 2 | Fixed-wing aircraft only |
| `SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER` | 3 | Helicopters only |
| `SIMCONNECT_SIMOBJECT_TYPE_BOAT` | 4 | Boats and watercraft |
| `SIMCONNECT_SIMOBJECT_TYPE_GROUND` | 5 | Ground vehicles |

## See Also

- [Engine/Client Usage](usage-client.md) — Connection lifecycle, data definitions, message handling
- [Client Configuration](config-client.md) — Configuration options for the Engine client
- [traffic-guide.md](traffic-guide.md) — Higher-level `pkg/traffic.Fleet` abstraction
- [examples/ai-traffic](../examples/ai-traffic) — Parked and enroute aircraft injection example
- [examples/manage-traffic](../examples/manage-traffic) — Full departure sequence with facility data, waypoints, and cleanup
