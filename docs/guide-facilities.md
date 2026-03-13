---
title: "Facility Data"
description: "Query airports, VORs, NDBs, and waypoints using the facility data API."
order: 8
section: "client"
---

# Facility Data

SimConnect exposes navigation database information through the facility data API. You can look up individual airports by ICAO code, enumerate all VORs or NDBs in range, subscribe to facilities that enter or leave proximity, and pull sub-tree data such as parking spots, taxiways, and jetways.

> **See also:** [Engine/Client Usage](usage-client.md) for the general client setup, connection lifecycle, and message dispatch loop that facility queries depend on.

## Overview

The facility API covers four top-level types:

| Type | List constant | Description |
|------|--------------|-------------|
| Airport | `SIMCONNECT_FACILITY_LIST_AIRPORT` | Airports, heliports, seaports |
| Waypoint | `SIMCONNECT_FACILITY_LIST_WAYPOINT` | Named enroute waypoints |
| NDB | `SIMCONNECT_FACILITY_LIST_TYPE_NDB` | Non-Directional Beacons |
| VOR | `SIMCONNECT_FACILITY_LIST_TYPE_VOR` | VHF Omnidirectional Ranges |

These constants come from `pkg/types` as `SIMCONNECT_FACILITY_LIST_TYPE`.

Facility data works through the same definition-and-request pattern used for SimObject data: you declare which fields you want, then issue a request. Responses arrive as one or more `SIMCONNECT_RECV_ID_FACILITY_DATA` messages, terminated by a `SIMCONNECT_RECV_ID_FACILITY_DATA_END` message.

## Facility Definition Setup

Before requesting facility data you must declare the fields you want to receive. Use `AddToFacilityDefinition` with string field names.

```go
//go:build windows

client.AddToFacilityDefinition(3000, "OPEN AIRPORT")
client.AddToFacilityDefinition(3000, "LATITUDE")
client.AddToFacilityDefinition(3000, "LONGITUDE")
client.AddToFacilityDefinition(3000, "ALTITUDE")
client.AddToFacilityDefinition(3000, "ICAO")
client.AddToFacilityDefinition(3000, "NAME")
client.AddToFacilityDefinition(3000, "NAME64")
client.AddToFacilityDefinition(3000, "CLOSE AIRPORT")
```

The `OPEN <TYPE>` / `CLOSE <TYPE>` sentinels mark the start and end of a facility block. They are required when building multi-level definitions (for example, an airport that also enumerates its parking spots or taxiways).

**Signature:**

```go
//go:build windows

func (e *Engine) AddToFacilityDefinition(definitionID uint32, fieldName string) error
```

### Using Pre-Built Facility Datasets

The `pkg/datasets/facilities` package provides ready-made definitions for all facility sub-types. Use `RegisterFacilityDataset` instead of calling `AddToFacilityDefinition` manually.

```go
//go:build windows

import "github.com/mrlm-net/simconnect/pkg/datasets/facilities"

client.RegisterFacilityDataset(3000, facilities.NewAirportFacilityDataset())
client.RegisterFacilityDataset(3001, facilities.NewRunwayFacilityDataset())
client.RegisterFacilityDataset(3002, facilities.NewParkingFacilityDataset())
client.RegisterFacilityDataset(3003, facilities.NewFrequencyFacilityDataset())
```

Available constructors:

| Constructor | Fields included |
|-------------|----------------|
| `NewAirportFacilityDataset()` | Name, ICAO, region, position, tower, transition altitude, country, city |
| `NewRunwayFacilityDataset()` | Dimensions, heading, surface type, lighting |
| `NewParkingFacilityDataset()` | Parking spots, gates, ramps |
| `NewFrequencyFacilityDataset()` | COM/NAV frequencies and types |
| `NewTaxiPointFacilityDataset()` | Taxiway intersection points |
| `NewTaxiPathFacilityDataset()` | Taxiway paths and routes |
| `NewTaxiNameFacilityDataset()` | Taxiway names |
| `NewHelipadFacilityDataset()` | Helipad location and properties |
| `NewJetwayFacilityDataset()` | Jetway data |
| `NewDepartureFacilityDataset()` | SID procedures |
| `NewApproachFacilityDataset()` | Approach procedures |
| `NewVORFacilityDataset()` | VOR navaid data |
| `NewNDBFacilityDataset()` | NDB navaid data |
| `NewWaypointFacilityDataset()` | Waypoint data |

## Single Facility Request

To retrieve data for a specific facility by ICAO code, use `RequestFacilityData`.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error
```

The `region` parameter is a two-character ICAO region code. Pass an empty string when the ICAO code is globally unique (most airports).

### Example: Airport Lookup

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

type AirportData struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
    ICAO      [8]byte
    Name      [32]byte
    Name64    [64]byte
}

const (
    AirportDefID = 3000
    AirportReqID = 123
)

func setupAirportDefinition(client *engine.Engine) {
    client.AddToFacilityDefinition(AirportDefID, "OPEN AIRPORT")
    client.AddToFacilityDefinition(AirportDefID, "LATITUDE")
    client.AddToFacilityDefinition(AirportDefID, "LONGITUDE")
    client.AddToFacilityDefinition(AirportDefID, "ALTITUDE")
    client.AddToFacilityDefinition(AirportDefID, "ICAO")
    client.AddToFacilityDefinition(AirportDefID, "NAME")
    client.AddToFacilityDefinition(AirportDefID, "NAME64")
    client.AddToFacilityDefinition(AirportDefID, "CLOSE AIRPORT")

    // Request LKPR (Prague Vaclav Havel Airport), no region filter
    client.RequestFacilityData(AirportDefID, AirportReqID, "LKPR", "")
}

func handleMessages(client *engine.Engine) {
    for msg := range client.Stream() {
        switch types.SIMCONNECT_RECV_ID(msg.DwID) {
        case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
            fd := msg.AsFacilityData()
            if fd.UserRequestId != AirportReqID {
                continue
            }
            data := engine.CastDataAs[AirportData](&fd.Data)
            fmt.Printf("Airport: %s\n", engine.BytesToString(data.ICAO[:]))
            fmt.Printf("  Name:   %s\n", engine.BytesToString(data.Name64[:]))
            fmt.Printf("  Lat:    %.6f\n", data.Latitude)
            fmt.Printf("  Lon:    %.6f\n", data.Longitude)
            fmt.Printf("  Alt:    %.1f m\n", data.Altitude)

        case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
            fmt.Println("Facility data transfer complete.")
        }
    }
}
```

The Go struct you pass to `CastDataAs` must mirror the field order and types declared in the definition. `float64` maps to `LATITUDE`, `LONGITUDE`, `ALTITUDE`; fixed-byte arrays map to string fields.

### Using RequestFacilityDataEX1

`RequestFacilityDataEX1` is an extended variant that accepts an explicit facility type byte. Use it when you need to disambiguate between facility types that share an ICAO code, or when targeting non-airport facilities.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error
```

The `facilityType` parameter corresponds to `SIMCONNECT_FACILITY_DATA_TYPE` constants from `pkg/types`:

| Constant | Value | Use |
|----------|-------|-----|
| `SIMCONNECT_FACILITY_DATA_AIRPORT` | 0 | Airport root record |
| `SIMCONNECT_FACILITY_DATA_VOR` | 19 | VOR navaid |
| `SIMCONNECT_FACILITY_DATA_NDB` | 20 | NDB navaid |
| `SIMCONNECT_FACILITY_DATA_WAYPOINT` | 21 | Enroute waypoint |

```go
//go:build windows

import "github.com/mrlm-net/simconnect/pkg/types"

// Request VOR data explicitly
client.RequestFacilityDataEX1(
    VorDefID,
    VorReqID,
    "BCN",
    "",
    byte(types.SIMCONNECT_FACILITY_DATA_VOR),
)
```

## Bulk Enumeration

### RequestFacilitiesList

Requests a snapshot of all known facilities of a given type. SimConnect returns results in batches; each batch is one `SIMCONNECT_RECV_ID_AIRPORT_LIST` (or equivalent) message.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
```

### RequestAllFacilities

`RequestAllFacilities` (MSFS 2024) is similar but takes an explicit request ID and does not require a prior definition. Use it for broad database dumps.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
```

### Example: Enumerating All Airports

```go
//go:build windows

package main

import (
    "fmt"
    "unsafe"

    "github.com/mrlm-net/simconnect/pkg/engine"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const AllAirportsReqID = 2000

func requestAllAirports(client *engine.Engine) {
    client.RequestAllFacilities(types.SIMCONNECT_FACILITY_LIST_AIRPORT, AllAirportsReqID)
}

func handleAirportList(msg engine.Message) {
    list := msg.AsAirportList()
    if list == nil {
        return
    }

    fmt.Printf("Packet %d of %d — %d airports\n",
        list.DwEntryNumber, list.DwOutOf, list.DwArraySize)

    if list.DwArraySize == 0 {
        return
    }

    // Calculate actual wire stride from message size and entry count.
    // Do not cast SIMCONNECT_DATA_FACILITY_AIRPORT directly — the Go struct
    // has alignment padding that does not match the SimConnect wire format.
    headerSize := unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{})
    dataSize := uintptr(msg.Size) - headerSize
    stride := dataSize / uintptr(list.DwArraySize)

    // Derive field byte offsets for this simulator version.
    var latOff, lonOff, altOff uintptr
    switch stride {
    case 33: // MSFS 2020: ident[6]+region[3]+3xfloat64
        latOff, lonOff, altOff = 9, 17, 25
    case 36, 40, 41: // MSFS 2024: ident[9]+region[3]+3xfloat64
        latOff, lonOff, altOff = 12, 20, 28
    default:
        fmt.Printf("Unknown entry stride %d bytes — skipping batch\n", stride)
        return
    }

    dataStart := unsafe.Pointer(
        uintptr(unsafe.Pointer(list)) + headerSize,
    )
    for i := uint32(0); i < uint32(list.DwArraySize); i++ {
        entry := unsafe.Pointer(uintptr(dataStart) + uintptr(i)*stride)

        var ident [6]byte
        copy(ident[:], (*[6]byte)(entry)[:])

        lat := *(*float64)(unsafe.Pointer(uintptr(entry) + latOff))
        lon := *(*float64)(unsafe.Pointer(uintptr(entry) + lonOff))
        alt := *(*float64)(unsafe.Pointer(uintptr(entry) + altOff))

        fmt.Printf("  %s  lat=%.4f lon=%.4f alt=%.1fm\n",
            engine.BytesToString(ident[:]), lat, lon, alt)
    }
}
```

> **Note:** The `SIMCONNECT_DATA_FACILITY_AIRPORT` struct in `pkg/types` has alignment padding that differs from the SimConnect wire format. Never cast a multi-entry list buffer directly to this struct. Use runtime stride arithmetic as shown above. See the inline comment in `pkg/types/facility.go` for details.

### Message Helpers for List Responses

Use these typed accessors instead of casting manually:

| Method | Returns | Message ID |
|--------|---------|-----------|
| `msg.AsAirportList()` | `*SIMCONNECT_RECV_AIRPORT_LIST` | `SIMCONNECT_RECV_ID_AIRPORT_LIST` |
| `msg.AsVORList()` | `*SIMCONNECT_RECV_VOR_LIST` | `SIMCONNECT_RECV_ID_VOR_LIST` |
| `msg.AsNDBList()` | `*SIMCONNECT_RECV_NDB_LIST` | `SIMCONNECT_RECV_ID_NDB_LIST` |
| `msg.AsWaypointList()` | `*SIMCONNECT_RECV_WAYPOINT_LIST` | `SIMCONNECT_RECV_ID_WAYPOINT_LIST` |
| `msg.AsFacilityList()` | `*SIMCONNECT_RECV_FACILITIES_LIST` | Any of the above |

`AsFacilityList()` matches any of the four list message types and returns the base struct that contains the pagination fields (`DwRequestID`, `DwArraySize`, `DwEntryNumber`, `DwOutOf`).

## Subscriptions

Subscriptions keep you informed as facilities enter and leave the simulator's active radius, without polling.

### SubscribeToFacilities

Registers a subscription for a facility type. SimConnect sends batches as the user's aircraft moves and new facilities come into range.

**Signature:**

```go
//go:build windows

func (e *Engine) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
```

```go
//go:build windows

const AirportSubReqID = 5000

client.SubscribeToFacilities(types.SIMCONNECT_FACILITY_LIST_AIRPORT, AirportSubReqID)
```

Responses arrive as `SIMCONNECT_RECV_ID_AIRPORT_LIST` messages. Handle them with `msg.AsAirportList()`.

### SubscribeToFacilitiesEX1

The extended variant uses two separate request IDs — one for facilities coming into range, one for facilities going out of range. This makes it straightforward to maintain a live set of nearby facilities.

**Signature:**

```go
//go:build windows

func (e *Engine) SubscribeToFacilitiesEX1(
    listType            types.SIMCONNECT_FACILITY_LIST_TYPE,
    newElemInRangeRequestID  uint32,
    oldElemOutRangeRequestID uint32,
) error
```

```go
//go:build windows

const (
    VorInRangeReqID  = 5010
    VorOutRangeReqID = 5011
)

client.SubscribeToFacilitiesEX1(
    types.SIMCONNECT_FACILITY_LIST_TYPE_VOR,
    VorInRangeReqID,
    VorOutRangeReqID,
)
```

In your dispatch loop, check `msg.DwRequestID` (inside the list struct) against each ID to determine whether the batch represents facilities entering or leaving range.

### UnsubscribeToFacilitiesEX1

Removes one or both sides of an EX1 subscription independently.

**Signature:**

```go
//go:build windows

func (e *Engine) UnsubscribeToFacilitiesEX1(
    listType              types.SIMCONNECT_FACILITY_LIST_TYPE,
    unsubscribeNewInRange bool,
    unsubscribeOldOutRange bool,
) error
```

```go
//go:build windows

// Stop receiving out-of-range notifications, keep in-range
client.UnsubscribeToFacilitiesEX1(
    types.SIMCONNECT_FACILITY_LIST_TYPE_VOR,
    false, // keep newInRange
    true,  // remove oldOutRange
)
```

## Jetway Data

`RequestJetwayData` retrieves jetway state for specific gate indexes at an airport.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error
```

`indexes` is a pointer to the first element of an `int32` slice listing which jetway gate indexes to query. `arrayCount` is the number of indexes in that slice.

```go
//go:build windows

// Query jetwavs at gate indexes 0, 1, and 2 of EGLL
gates := []int32{0, 1, 2}
client.RequestJetwayData("EGLL", uint32(len(gates)), &gates[0])
```

Jetway responses arrive as `SIMCONNECT_RECV_ID_FACILITY_DATA` messages with `Type` equal to `SIMCONNECT_FACILITY_DATA_JETWAY`. Use `msg.AsFacilityData()` and cast `Data` to your jetway struct, or use `NewJetwayFacilityDataset()` to set up the definition first.

## Filters

Filters narrow the fields returned within a facility definition. They are applied per-definition, not per-request.

### AddFacilityDataDefinitionFilter

Adds a filter to a facility data definition. Only facilities matching the filter condition are included in the response.

**Signature:**

```go
//go:build windows

func (e *Engine) AddFacilityDataDefinitionFilter(
    definitionID  uint32,
    filterPath    string,
    filterData    unsafe.Pointer,
    filterDataSize uint32,
) error
```

The `filterPath` is a dot-separated path to the field being filtered (for example `"TYPE"`). `filterData` points to the value to match against; `filterDataSize` is its byte size.

```go
//go:build windows

import "unsafe"

// Filter parking spots to heavy gates only
parkingType := uint32(10) // SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY
client.AddFacilityDataDefinitionFilter(
    ParkingDefID,
    "TYPE",
    unsafe.Pointer(&parkingType),
    uint32(unsafe.Sizeof(parkingType)),
)
```

### ClearAllFacilityDataDefinitionFilters

Removes all filters from a definition, restoring unfiltered results.

**Signature:**

```go
//go:build windows

func (e *Engine) ClearAllFacilityDataDefinitionFilters(definitionID uint32) error
```

```go
//go:build windows

client.ClearAllFacilityDataDefinitionFilters(ParkingDefID)
```

## Message Helpers Reference

The `engine.Message` struct exposes these facility-related cast methods:

| Method | Return type | When to use |
|--------|------------|-------------|
| `AsFacilityData()` | `*SIMCONNECT_RECV_FACILITY_DATA` | Each record from `RequestFacilityData` or `RequestFacilityDataEX1` |
| `AsFacilityDataEnd()` | `*SIMCONNECT_RECV_FACILITY_DATA_END` | Signals the end of a `RequestFacilityData` response sequence |
| `AsFacilityList()` | `*SIMCONNECT_RECV_FACILITIES_LIST` | Base struct for any list message; contains pagination fields |
| `AsAirportList()` | `*SIMCONNECT_RECV_AIRPORT_LIST` | Airport enumeration batches |
| `AsVORList()` | `*SIMCONNECT_RECV_VOR_LIST` | VOR enumeration batches |
| `AsNDBList()` | `*SIMCONNECT_RECV_NDB_LIST` | NDB enumeration batches |
| `AsWaypointList()` | `*SIMCONNECT_RECV_WAYPOINT_LIST` | Waypoint enumeration batches |

Each method returns `nil` if the message type does not match, so nil-checking is safe in a type-switch fallthrough path.

### SIMCONNECT_RECV_FACILITY_DATA Fields

```go
//go:build windows

fd := msg.AsFacilityData()
// fd.UserRequestId       — matches the requestID you passed to RequestFacilityData
// fd.UniqueRequestId     — internal SimConnect identifier for this record
// fd.ParentUniqueRequestId — identifier of the parent record (for nested types)
// fd.Type                — SIMCONNECT_FACILITY_DATA_TYPE (airport, runway, parking, etc.)
// fd.IsListItem          — true when the record is part of a child list (e.g., a parking spot)
// fd.ItemIndex           — zero-based index within the child list
// fd.ListSize            — total items in the child list
// fd.Data                — opaque DWORD; pass to engine.CastDataAs[YourStruct](&fd.Data)
```

The `Type` field maps to `SIMCONNECT_FACILITY_DATA_TYPE` constants in `pkg/types/facility.go`. The complete set includes `SIMCONNECT_FACILITY_DATA_AIRPORT`, `SIMCONNECT_FACILITY_DATA_RUNWAY`, `SIMCONNECT_FACILITY_DATA_TAXI_PARKING`, `SIMCONNECT_FACILITY_DATA_JETWAY`, `SIMCONNECT_FACILITY_DATA_VOR`, `SIMCONNECT_FACILITY_DATA_NDB`, `SIMCONNECT_FACILITY_DATA_WAYPOINT`, and others.

## See Also

- [Engine/Client Usage](usage-client.md) — Connection lifecycle, stream setup, and data casting
- [Manager Usage](usage-manager.md) — Auto-reconnect wrapper with facility helper methods
- [`examples/read-facility`](../examples/read-facility) — Single airport lookup
- [`examples/airport-details`](../examples/airport-details) — Multi-definition airport inspection including parking and taxiways
- [`examples/all-facilities`](../examples/all-facilities) — Full airport enumeration with stride arithmetic
