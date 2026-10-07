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

The `pkg/datasets/facilities` package provides ready-made definitions for the facility sub-types. Each constructor returns a `*datasets.FacilityDataSet` (a list of field names) that `RegisterFacilityDataset` adds to a definition in order, instead of calling `AddToFacilityDefinition` manually.

Each dataset carries only its own `OPEN`/`CLOSE` pair. The airport, VOR, NDB and waypoint datasets are top-level; a child such as a runway or parking spot must be wrapped in its parent's block:

```go
//go:build windows

import "github.com/mrlm-net/simconnect/pkg/datasets/facilities"

client.RegisterFacilityDataset(3000, facilities.NewAirportFacilityDataset())

// Runways of an airport: the child dataset inside OPEN/CLOSE AIRPORT
client.AddToFacilityDefinition(3001, "OPEN AIRPORT")
client.RegisterFacilityDataset(3001, facilities.NewRunwayFacilityDataset())
client.AddToFacilityDefinition(3001, "CLOSE AIRPORT")
```

Available constructors:

| Constructor | Fields included |
|-------------|----------------|
| `NewAirportFacilityDataset()` | Position, magnetic variation, name, ICAO, region, tower position, transition altitude and level, closed flag, country, city |
| `NewRunwayFacilityDataset()` | Position, heading, length, width, pattern altitude, slope, surface; per end: ILS, number, designator, threshold, blast pad, overrun, approach lights, VASI |
| `NewStartFacilityDataset()` | Runway start positions |
| `NewPavementFacilityDataset()`, `NewApproachLightsFacilityDataset()`, `NewVASIFacilityDataset()` | Runway sub-records (threshold, blast pad, overrun; approach lights; VASI) |
| `NewParkingFacilityDataset()` | Parking type, name, suffix, number, orientation, heading, radius, position (bias) |
| `NewFrequencyFacilityDataset()` | Frequency type, frequency, name |
| `NewTaxiPointFacilityDataset()` | Taxiway points: type, orientation, position |
| `NewTaxiPathFacilityDataset()` | Taxiway paths: type, widths, runway, edges and centre line, start and end points, name index |
| `NewTaxiNameFacilityDataset()` | Taxiway names |
| `NewHelipadFacilityDataset()` | Helipad position, heading, size, surface, type |
| `NewJetwayFacilityDataset()` | The parking gate, suffix and spot a jetway serves |
| `NewDepartureFacilityDataset()`, `NewArrivalFacilityDataset()` | SIDs and STARs: name and transition counts |
| `NewRunwayTransitionFacilityDataset()`, `NewEnrouteTransitionFacilityDataset()` | SID/STAR transitions |
| `NewApproachFacilityDataset()` | Approaches: type, runway, FAF, missed altitude, LNAV/VNAV/LP/LPV, leg counts |
| `NewApproachTransitionFacilityDataset()` | Approach transitions (IAF, DME arc) |
| `NewApproachLegFacilityDataset()`, `NewFinalApproachLegFacilityDataset()`, `NewMissedApproachLegFacilityDataset()` | Procedure legs: fix, path, altitudes, speed limit, IAF/IF/FAF/MAP flags |
| `NewVORFacilityDataset()` | VOR/DME/ILS/TACAN positions and flags, frequency, range, localizer, glide slope |
| `NewNDBFacilityDataset()` | NDB position, frequency, type, range |
| `NewWaypointFacilityDataset()` | Waypoint position, type, route count, ICAO, region |
| `NewRouteFacilityDataset()` | A waypoint's airways: name, type, next and previous fix |

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

Requests a list of facilities of a given type. SimConnect returns results in batches; each batch is one `SIMCONNECT_RECV_ID_AIRPORT_LIST` (or equivalent) message carrying your request ID. No facility definition is needed. `RequestFacilitiesListEX1` takes the same arguments and calls `SimConnect_RequestFacilitiesList_EX1`.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestFacilitiesList(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
func (e *Engine) RequestFacilitiesListEX1(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
```

### RequestAllFacilities

`RequestAllFacilities` (MSFS 2024) is similar and also needs no definition. Note the argument order: the list type comes first, then the request ID. Use it for broad database dumps.

**Signature:**

```go
//go:build windows

func (e *Engine) RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
```

### Example: Enumerating All Airports

The list entries are packed on the wire, so a Go struct with doubles does not match them. On MSFS 2024 each list type decodes its entries for you: `list.Entries()` on `*SIMCONNECT_RECV_AIRPORT_LIST`, `_WAYPOINT_LIST`, `_NDB_LIST` and `_VOR_LIST` (sizes `types.FacilityAirportSize` 36, `FacilityWaypointSize` 40, `FacilityNDBSize` 44, `FacilityVORSize` 80 bytes), or `types.DecodeFacilityAirport` and its siblings for one entry. `Entries` reads within the message's `DwSize`, so a short message gives fewer entries.

```go
if list := msg.AsAirportList(); list != nil {
    for _, a := range list.Entries() {
        fmt.Printf("%s lat=%.4f lon=%.4f\n", engine.BytesToString(a.Ident[:]), a.Latitude, a.Longitude)
    }
}
```

`Entries` assumes the MSFS 2024 sizes. To handle other entry sizes (MSFS 2020's airports are 33 bytes), derive the stride from the message size by hand:

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

> **Note:** The `SIMCONNECT_DATA_FACILITY_AIRPORT` struct in `pkg/types` has alignment padding that differs from the SimConnect wire format. Never cast a multi-entry list buffer directly to this struct. Use `Entries`, the `Decode*` functions or runtime stride arithmetic as shown above. See the comments in `pkg/types/facility.go` and `pkg/types/lists.go` for details.

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

Jetway responses arrive as `SIMCONNECT_RECV_ID_JETWAY_DATA` messages (`types.SIMCONNECT_RECV_JETWAY_DATA`, packed `SIMCONNECT_JETWAY_DATA` entries of `types.JetwayDataSize` 160 bytes). There is no `As*` helper for them; cast `msg.SIMCONNECT_RECV` to `*types.SIMCONNECT_RECV_JETWAY_DATA` and read the entries with its `Entries()` (or `types.DecodeJetwayData` for one), never by casting the entries. For jetway records inside an airport, add `NewJetwayFacilityDataset()` to a `RequestFacilityData` definition instead; those arrive as `FACILITY_DATA` messages with `Type` equal to `SIMCONNECT_FACILITY_DATA_JETWAY`.

## Filters

Filters narrow the records returned for a facility definition. They are applied per-definition, not per-request.

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
// fd.IsListItem          — non-zero (DWORD) when the record is part of a child list (e.g., a parking spot)
// fd.ItemIndex           — zero-based index within the child list
// fd.ListSize            — total items in the child list
// fd.Data                — opaque DWORD; pass to engine.CastDataAs[YourStruct](&fd.Data)
```

The `Type` field maps to `SIMCONNECT_FACILITY_DATA_TYPE` constants in `pkg/types/facility.go`. The complete set includes `SIMCONNECT_FACILITY_DATA_AIRPORT`, `SIMCONNECT_FACILITY_DATA_RUNWAY`, `SIMCONNECT_FACILITY_DATA_TAXI_PARKING`, `SIMCONNECT_FACILITY_DATA_JETWAY`, `SIMCONNECT_FACILITY_DATA_VOR`, `SIMCONNECT_FACILITY_DATA_NDB`, `SIMCONNECT_FACILITY_DATA_WAYPOINT`, and others.

## See Also

- [Engine/Client Usage](usage-client.md) — Connection lifecycle, stream setup, and data casting
- [Manager Usage](usage-manager.md) — Auto-reconnect wrapper; the manager exposes the same facility methods (`pkg/manager/facilities.go`)
- [Airport Layout & Taxi Routing](airport-layout.md) — `pkg/airport` loader built on these requests
- [`examples/read-facility`](../examples/read-facility) — Single airport lookup
- [`examples/airport-details`](../examples/airport-details) — Multi-definition airport inspection including parking and taxiways
- [`examples/all-facilities`](../examples/all-facilities) — Full airport enumeration with stride arithmetic
