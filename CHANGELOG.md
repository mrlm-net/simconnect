# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [0.4.3] - 2026-03-05

### Fixed

#### `pkg/types` — wire struct alignment bugs (second pass)

Two additional Go-vs-wire alignment bugs discovered by systematic review, following the same
`#pragma pack(1)` vs Go alignment-padding pattern fixed in v0.4.2.

**`SIMCONNECT_DATA_RACE_RESULT` — critical data corruption on float64 fields**

`FTotalTime float64` and `FPenaltyTime float64` were at wire offset 1060 and 1068 respectively.
The prefix before `FTotalTime` is `DWORD(4) + GUID(16) + 4×char[260](1040) = 1060 bytes`;
`1060 % 8 = 4`, so Go inserts 4 bytes of padding, shifting both fields 4 bytes past their
wire positions. Any cast of a raw SimConnect buffer to this struct would silently produce
garbage values for both timing fields and `DwIsDisqualified`.

Fix: `FTotalTime float64` → `FTotalTimeBytes [8]byte` and `FPenaltyTime float64` →
`FPenaltyTimeBytes [8]byte` (alignment 1, no padding). Decode with
`math.Float64frombits(binary.LittleEndian.Uint64(r.FTotalTimeBytes[:]))`.

Note: This is a **breaking rename** of public fields. Any code reading `.FTotalTime` or
`.FPenaltyTime` directly will fail to compile — this is intentional, as silent misreads
are more dangerous than a compile error.

**`SIMCONNECT_DATA_FACILITY_VOR` — compound misalignment documented**

The VOR struct has two independent misalignment layers that compound:

1. Airport base (already documented on `SIMCONNECT_DATA_FACILITY_AIRPORT`): the
   `ident+region` byte prefix before `Latitude` is not 8-byte aligned, causing Go to
   pad before all float64 fields.
2. VOR-internal (previously undocumented): `Flags DWORD` immediately precedes
   `FLocalizer float64`. `NDB` Go sizeof = 56; `Flags` ends at offset 60; `60 % 8 = 4`;
   Go pads 4 more bytes. `FLocalizer` lands at Go offset 64 vs wire offset 52 (MSFS 2024)
   — a 12-byte total discrepancy affecting all five VOR float64 fields.

Fix: detailed `WARNING` godoc block added to `SIMCONNECT_DATA_FACILITY_VOR` documenting
both misalignment levels and exact Go vs wire offsets. No field changes (struct is only
used via runtime stride arithmetic per the AIRPORT pattern).

### Added

#### `pkg/engine` — MSFS 2024 Input Event API (#143)

Six new methods on the `Client` interface expose the full Input Event lifecycle. This API
is available in MSFS 2024 only — the underlying DLL functions are not present in MSFS 2020.

| Method | Description |
|--------|-------------|
| `EnumerateInputEvents(requestID uint32) error` | Requests a paginated list of all input events known to the simulator; responses arrive as `SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` messages |
| `GetInputEvent(requestID uint32, hash uint64) error` | Requests the current value of a single input event by 64-bit hash; response arrives as `SIMCONNECT_RECV_GET_INPUT_EVENT` |
| `SetInputEventDouble(hash uint64, value float64) error` | Sets an input event value from a Go `float64`; owns the stack-allocated buffer for the synchronous DLL call duration |
| `SetInputEventString(hash uint64, value string) error` | Sets an input event value from a Go `string`; same buffer-safety guarantee as the double variant |
| `SubscribeInputEvent(hash uint64) error` | Subscribes to value-change notifications for an input event; updates arrive as `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` messages |
| `UnsubscribeInputEvent(hash uint64) error` | Cancels a previous subscription |

Four package-level value extractor functions are provided in `pkg/engine` to decode the
inline byte buffers returned by the DLL without exposing `unsafe.Pointer` to callers:

| Function | Description |
|----------|-------------|
| `InputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (float64, bool)` | Reads the first 8 bytes of `Value` as a little-endian IEEE 754 `float64`; returns `false` if `EType` is not `SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE` |
| `InputEventValueAsString(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (string, bool)` | Reads `Value` to null terminator as a UTF-8 string; returns `false` if `EType` is not `SIMCONNECT_INPUT_EVENT_TYPE_STRING` |
| `SubscribeInputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (float64, bool)` | Same as above for subscribe receive type |
| `SubscribeInputEventValueAsString(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (string, bool)` | Same as above for subscribe receive type |

Three new `As*` helpers on `*Message` follow the existing nil-guard-then-cast pattern:

| Method | Description |
|--------|-------------|
| `(*Message).AsEnumerateInputEvents() *types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` | Casts dispatch buffer to enumerate response; returns `nil` if message ID does not match |
| `(*Message).AsGetInputEvent() *types.SIMCONNECT_RECV_GET_INPUT_EVENT` | Casts dispatch buffer to get-event response |
| `(*Message).AsSubscribeInputEvent() *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` | Casts dispatch buffer to subscribe notification |

#### `pkg/types` — receive struct fixes and new type (#143)

- `SIMCONNECT_RECV_GET_INPUT_EVENT.Value` corrected from `unsafe.Pointer` to `[260]byte` —
  the original type was incorrect because the bytes are inline in the DLL dispatch buffer,
  not a heap pointer. `[260]byte` covers both DOUBLE (8 bytes, read via
  `math.Float64frombits`) and STRING (up to 32 chars per MSFS 2024 SDK) and is safe for
  the GC.
- `SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT` restructured to flat fields (no embedded
  `SIMCONNECT_RECV`) — confirmed via MSFS 2024 SDK and FlyByWire Rust bindgen output that
  `SimConnect.h` wraps this struct in `#pragma pack(1)`. Go's natural alignment would
  insert 4 bytes of padding before `Hash` (UINT64 at wire offset 12), producing incorrect
  field reads. The flat layout matches the wire format exactly. `Value` corrected from
  `unsafe.Pointer` to `[260]byte` (max 256 chars for STRING per MSFS 2024 SDK).
- `SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS` added — embeds `SIMCONNECT_RECV_LIST_TEMPLATE`
  (28 bytes) with a sentinel `RgData [1]SIMCONNECT_INPUT_EVENT_DESCRIPTOR` field; iterate
  over `DwArraySize` elements via the `AsEnumerateInputEvents()` engine helper using
  unsafe pointer arithmetic, identical to the existing airport/NDB/VOR list patterns.

#### `internal/simconnect` — five new DLL bindings (#143)

`internal/simconnect/inputevent.go` adds raw syscall wrappers for
`SimConnect_EnumerateInputEvents`, `SimConnect_GetInputEvent`, `SimConnect_SetInputEvent`,
`SimConnect_SubscribeInputEvent`, and `SimConnect_UnsubscribeInputEvent`. The 64-bit hash
parameter is passed as `uintptr(hash)` at the syscall boundary (amd64 Windows, no
split-register concern). `SetInputEvent` accepts `unsafe.Pointer` at the internal API
boundary only; the `pkg/engine` typed wrappers above keep `unsafe.Pointer` off the public
`Client` interface entirely.

---

## [0.4.2] - 2026-03-05

### Fixed

#### `pkg/manager` — critical `simStateDataStruct` alignment bug

`simStateDataStruct` mixed `int32` and `float64` fields. SimConnect packs data definition
buffers with no alignment padding; Go inserts padding before `float64` fields not at
8-byte-aligned struct offsets. This produced four silent misalignment gaps:

- Before `Latitude` (+4 bytes): all position and speed fields read from wrong offsets
- Before `MissionScore` (+8 bytes cumulative)
- Before `ZuluSunriseTime` (+12 bytes cumulative): time zone fields corrupted
- Before `EnvSmokeDensity` (+16 bytes cumulative): all extended environment fields corrupted

The bug produced garbage floating-point values (e.g. `Lon = -2.6e+67`) in every Manager
SimState update. Introduced in commit `06e735b` (v0.2.0, 2026-02-08) when `SurfaceType`
(`int32`) was added immediately before the `Latitude` (`float64`) block.

Fix: all fields in `simStateDataStruct` changed to `float64`. SimConnect automatically
converts integer SimVars to `float64` when `SIMCONNECT_DATATYPE_FLOAT64` is requested,
so no data is lost. All `AddToDataDefinition` calls in `simstate_registration.go` updated
to match (`SIMCONNECT_DATATYPE_INT32` → `SIMCONNECT_DATATYPE_FLOAT64`). Cast sites in
`dispatch-simstate.go` updated with `int32(stateData.X)` where the public `SimState`
field is `int32`.

#### `pkg/types` — wire struct alignment fixes

- `SIMCONNECT_RECV_SYSTEM_STATE.FFloat float64` → `FFloatBytes [8]byte` — `float64`
  (alignment 8) after `SIMCONNECT_RECV` (12 B) + `DwRequestID` (4 B) + `WInteger` (4 B)
  = 20 bytes total; Go would pad to offset 24. `[8]byte` (alignment 1) places the field
  at wire-correct offset 20. Use `engine.SystemStateFloat64(recv)` to decode.
- `SIMCONNECT_JETWAY_DATA.ParkingIndex`, `Status`, `Door` changed from `int` (8 bytes on
  64-bit Go) to `uint32` (4 bytes, matching the SDK DWORD). The previous `int` fields
  doubled the size of each, corrupting all subsequent field offsets.

### Added

#### `pkg/engine` — value extractor helpers

- `SystemStateFloat64(recv *types.SIMCONNECT_RECV_SYSTEM_STATE) float64` — decodes
  `FFloatBytes [8]byte` via `binary.LittleEndian`; eliminates manual bit conversion at
  call sites.
- `SubscribeInputEventHash(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) uint64` —
  decodes `HashBytes [8]byte` at wire offset 12; callers no longer need to write
  `binary.LittleEndian.Uint64(recv.HashBytes[:])` directly.

---

## [0.4.1] - 2026-03-01

### Fixed

- `pkg/convert/position.go` — aligned variable name `w` → `W` in `LatLonToOffset` to match `OffsetToLatLon` naming convention (#217)
- `examples/locate-airport/README.md` — replaced stale `haversineMeters()` references with `calc.HaversineMeters()` following promotion in #219 (#217)

---

## [0.4.0] - 2026-03-01

### Added

#### `pkg/traffic` — traffic guide and updated example (#38)

- `docs/traffic-guide.md` — MVP guide covering all three aircraft kinds, the async
  create→acknowledge lifecycle, waypoint helpers with flag reference, fleet management
  API, manager integration, and known limitations (no ground routing yet)
- `examples/simconnect-traffic/main.go` — rewritten to use `pkg/traffic`: parked spawn,
  non-ATC spawn with pushback→taxi→takeoff waypoint chain, periodic fleet status log,
  and graceful `Fleet.RemoveAll` on shutdown
- Website sidebar now includes Traffic and Datasets navigation sections

---

## [0.3.13] - 2026-03-01

### Added

#### `pkg/traffic` — new AI traffic abstraction package (epic #27 / #36, #37)

| API | Description |
|-----|-------------|
| `NewFleet(client engine.Client) *Fleet` | Creates a thread-safe aircraft fleet bound to an engine client |
| `(*Fleet).RequestParked(opts ParkedOpts, reqID uint32) error` | Queues a parked ATC aircraft creation; resolves asynchronously via `Acknowledge` |
| `(*Fleet).RequestEnroute(opts EnrouteOpts, reqID uint32) error` | Queues an enroute ATC aircraft creation along a flight plan |
| `(*Fleet).RequestNonATC(opts NonATCOpts, reqID uint32) error` | Queues a non-ATC aircraft creation at an explicit position |
| `(*Fleet).Acknowledge(reqID, objectID uint32) (*Aircraft, bool)` | Promotes a pending creation to a tracked `Aircraft` handle; call from `ASSIGNED_OBJECT_ID` handler |
| `(*Fleet).Remove(objectID, reqID uint32) error` | Removes an aircraft from the simulation and the fleet |
| `(*Fleet).ReleaseControl(objectID, reqID uint32) error` | Releases simulator AI control; required before `SetWaypoints` |
| `(*Fleet).SetWaypoints(objectID, defID uint32, wps []SIMCONNECT_DATA_WAYPOINT) error` | Assigns a waypoint chain to a non-ATC aircraft |
| `(*Fleet).SetFlightPlan(objectID uint32, planPath string, reqID uint32) error` | Assigns a flight plan to an ATC aircraft |
| `(*Fleet).Get(objectID uint32) (*Aircraft, bool)` | Returns the tracked `Aircraft` for a given ObjectID |
| `(*Fleet).List() []*Aircraft` | Snapshot of all active aircraft |
| `(*Fleet).Len() int` | Number of active (acknowledged) aircraft |
| `(*Fleet).RemoveAll(reqIDBase uint32) error` | Removes all tracked aircraft |
| `(*Fleet).Clear()` | Resets fleet state without issuing removal calls (use on disconnect) |
| `(*Fleet).SetClient(client engine.Client)` | Swaps the engine client and clears stale state (call on reconnect) |
| `PushbackWaypoint(lat, lon, altFt, ktsSpeed float64)` | Waypoint with `ON_GROUND \| REVERSE \| SPEED_REQUESTED` flags |
| `TaxiWaypoint(lat, lon, altFt, ktsSpeed float64)` | Waypoint with `ON_GROUND \| SPEED_REQUESTED` flags |
| `LineupWaypoint(lat, lon, altFt float64)` | Runway threshold waypoint at 5 kts |
| `ClimbWaypoint(lat, lon, altAGL, ktsSpeed, throttlePct float64)` | Airborne waypoint with `SPEED_REQUESTED \| THROTTLE_REQUESTED \| COMPUTE_VERTICAL_SPEED \| ALTITUDE_IS_AGL` |
| `TakeoffClimb(rwyLat, rwyLon, hdgDeg float64) []SIMCONNECT_DATA_WAYPOINT` | Standard 3-WP climb chain from runway threshold (1.5 nm / 5 nm / 12 nm) |

#### `pkg/manager` — traffic delegation methods (#37)

| API | Description |
|-----|-------------|
| `Fleet() *traffic.Fleet` | Returns the manager's internal fleet; reset on each reconnect |
| `TrafficParked(opts traffic.ParkedOpts, reqID uint32) error` | Delegates to `Fleet().RequestParked` |
| `TrafficEnroute(opts traffic.EnrouteOpts, reqID uint32) error` | Delegates to `Fleet().RequestEnroute` |
| `TrafficNonATC(opts traffic.NonATCOpts, reqID uint32) error` | Delegates to `Fleet().RequestNonATC` |
| `TrafficRemove(objectID, reqID uint32) error` | Delegates to `Fleet().Remove` |
| `TrafficReleaseControl(objectID, reqID uint32) error` | Delegates to `Fleet().ReleaseControl` |
| `TrafficSetWaypoints(objectID, defID uint32, wps []SIMCONNECT_DATA_WAYPOINT) error` | Delegates to `Fleet().SetWaypoints` |
| `TrafficSetFlightPlan(objectID uint32, planPath string, reqID uint32) error` | Delegates to `Fleet().SetFlightPlan` |

---

## [0.3.12] - 2026-03-01

### Added

#### `pkg/datasets` — composition helpers (epic #26 / #32)

| API | Description |
|-----|-------------|
| `(DataSet).Clone() DataSet` | Deep copy of a dataset; returned value has an independent backing slice |
| `Merge(...DataSet) DataSet` | Combines multiple datasets; last definition wins on duplicate `Name` (position shifts to last occurrence) |
| `NewBuilder() *Builder` | Fluent builder for incremental dataset construction |
| `(*Builder).Add(def DataDefinition) *Builder` | Appends a pre-built definition |
| `(*Builder).AddField(name, unit string, dataType, epsilon) *Builder` | Convenience wrapper that constructs a `DataDefinition` inline |
| `(*Builder).Remove(name string) *Builder` | Removes first definition matching `Name` |
| `(*Builder).Build() DataSet` | Returns a new `DataSet`; backing slice is independent of the builder |
| `(*Builder).Len() int` | Number of pending definitions |
| `(*Builder).Reset() *Builder` | Clears the builder (severs backing array) |

#### `pkg/datasets` — global registry (epic #26 / #34)

| API | Description |
|-----|-------------|
| `Register(name, category string, constructor func() *DataSet)` | Registers a named dataset constructor; panics on empty name or category; silently overwrites duplicates |
| `Get(name string) (func() *DataSet, bool)` | Retrieves a constructor by name |
| `List() []string` | Sorted list of all registered names |
| `Categories() []string` | Sorted list of distinct categories |
| `ListByCategory(category string) []string` | Sorted names in a given category; nil if category unknown |

`pkg/datasets/traffic` now auto-registers `"traffic/aircraft"` (category `"traffic"`) via `init()` — import it blank (`_ "github.com/mrlm-net/simconnect/pkg/datasets/traffic"`) to activate.

#### Documentation (epic #26 / #35)

- New guide: `docs/dataset-composition.md` — covers `Clone`, `Merge`, `Builder`, and the global registry with runnable end-to-end snippet.
- `examples/using-datasets/main.go` rewritten to demonstrate blank-import auto-registration, `List`, `Categories`, `Get`, `Clone`, `Builder`, and `Merge`.

---

## [0.3.11] - 2026-02-28

### Fixed

- **Sponsorship links** — appended `?currency=EUR` to all Revolut URLs in `.github/FUNDING.yml`, `README.md`, and the marketing homepage.

---

## [0.3.10] - 2026-02-28

### Added

- **Sponsorship infrastructure** — `.github/FUNDING.yml` activates the GitHub Sponsor button (Revolut custom URL); `README.md` gains a `## Sponsoring` section listing what sponsorship covers; the marketing homepage gains a full-width CTA section. No Go code or API changes. Closes #172, #173, #174.

---

## [0.3.6] - 2026-02-22

### Fixed

- **`pkg/types/receiver.go`** — `SIMCONNECT_RECV_VOR_LIST` had a wrong struct layout copy-pasted from `SIMCONNECT_RECV_SYSTEM_STATE`. It now correctly embeds `SIMCONNECT_RECV_FACILITIES_LIST` with `RgData []SIMCONNECT_DATA_FACILITY_VOR`, matching the SDK and the pattern of `SIMCONNECT_RECV_NDB_LIST` / `SIMCONNECT_RECV_AIRPORT_LIST`. `AsVORList()` previously returned a misinterpreted pointer. Closes #189.
- **`pkg/datasets/facilities/ndb.go`** — `NewNDBFacilityDataset()` was missing `ICAO` and `REGION` fields; NDB identifiers were silently absent from dataset responses. Closes #190.
- **`pkg/datasets/facilities/vor.go`** — `NewVORFacilityDataset()` was missing `ICAO` and `REGION` fields. Closes #191.
- **`pkg/datasets/facilities/waypoint.go`** — `NewRouteFacilityDataset()` PREV block was missing `PREV_LATITUDE` and `PREV_LONGITUDE`; the NEXT block had both but PREV did not. Closes #192.

### Added

#### `pkg/convert`

| Function | File | Description |
|----------|------|-------------|
| `NMToStatuteMiles` | distance.go | NM → statute miles |
| `StatuteMilesToNM` | distance.go | Statute miles → NM |
| `KilometersToStatuteMiles` | distance.go | km → statute miles |
| `StatuteMilesToKilometers` | distance.go | Statute miles → km |
| `StatuteMilesToMeters` | distance.go | Statute miles → m |
| `MetersToStatuteMiles` | distance.go | m → statute miles |
| `KnotsToFeetPerSecond` | speed.go | knots → ft/s (SimConnect body-axis velocity unit) |
| `FeetPerSecondToKnots` | speed.go | ft/s → knots |
| `NormalizeAngle` | angle.go | Normalises angle to (-180, 180] |
| `AngleDifference` | angle.go | Shortest signed rotation from → to in (-180, 180] |

Closes #193, #194, #195.

#### `pkg/calc`

| Function | File | Description |
|----------|------|-------------|
| `AlongTrackMeters` | crosstrack.go | Signed along-track distance from A toward B for point D; positive = ahead, negative = behind |
| `HaversineKM` | haversine.go | Great-circle distance in kilometres |

Closes #196, #197.

---

## [0.3.5] - 2026-02-22

### Added

#### `pkg/calc`

New aviation math functions extending the cross-track and wind correction capabilities of the package.

| Function | Signature | Description |
|----------|-----------|-------------|
| `CrossTrackMeters` | `(latA, lonA, latB, lonB, latD, lonD float64) float64` | Great-circle cross-track distance in meters; positive values indicate the point is to the right of the track |
| `WindCorrectionAngle` | `(windDir, windSpeed, tas, course float64) float64` | Wind correction angle in degrees; returns 0 for near-zero true airspeed |
| `TrueToMagnetic` | `(trueHeading, magVar float64) float64` | Converts a true heading to magnetic; positive `magVar` is easterly; result is normalised to [0, 360) |
| `MagneticToTrue` | `(magneticHeading, magVar float64) float64` | Inverse of `TrueToMagnetic` |
| `CrosswindComponent` | `(windDir, windSpeed, runwayHeading float64) float64` | Signed crosswind component; wrapper over `HeadwindCrosswind` |
| `HeadwindComponent` | `(windDir, windSpeed, runwayHeading float64) float64` | Headwind (positive) / tailwind (negative) component; wrapper over `HeadwindCrosswind` |

Closes #133, #134, #135, #138.

#### `pkg/convert`

Three new conversion files covering temperature, pressure, and weight/volume domains.

**`temperature.go`**

| Function | Converts |
|----------|---------|
| `CelsiusToFahrenheit` | °C → °F |
| `FahrenheitToCelsius` | °F → °C |
| `CelsiusToKelvin` | °C → K |
| `KelvinToCelsius` | K → °C |
| `FahrenheitToKelvin` | °F → K |
| `KelvinToFahrenheit` | K → °F |

**`pressure.go`**

| Function | Converts |
|----------|---------|
| `InHgToMillibar` | inHg → mbar |
| `MillibarToInHg` | mbar → inHg |
| `InHgToHectopascal` | inHg → hPa |
| `HectopascalToInHg` | hPa → inHg |
| `InHgToPascal` | inHg → Pa |
| `PascalToInHg` | Pa → inHg |

**`weight.go`**

| Function | Converts |
|----------|---------|
| `PoundsToKilograms` | lb → kg |
| `KilogramsToPounds` | kg → lb |
| `USGallonsToLiters` | US gal → L |
| `LitersToUSGallons` | L → US gal |

Closes #136, #139, #140, #141, #142.

### Fixed

- Closed stale chore issues that were completed as part of v0.3.4: removal of `//go:build windows` build tags from `pkg/calc` (#132) and all existing `pkg/convert` files (#137). No code changes in this release for these items.

---

## [0.3.4] - 2026-02-22

### Added

#### `pkg/convert`

| File | Functions |
|------|-----------|
| `angle.go` _(new)_ | `DegreesToRadians`, `RadiansToDegrees`, `NormalizeHeading` |
| `distance` | `NMToKilometers`, `KilometersToNM`, `KilometersToMeters`, `MetersToKilometers` |
| `speed` | `KnotsToMetersPerSecond`, `MetersPerSecondToKnots`, `FeetPerMinuteToMetersPerSecond`, `MetersPerSecondToFeetPerMinute` |
| `altitude` | `FeetPerMinuteToFeetPerSecond`, `FeetPerSecondToFeetPerMinute` |
| `position` | Pole guard in `OffsetToLatLon` — prevents division singularity at ±90° latitude |

#### `pkg/calc`

| File | Functions |
|------|-----------|
| `haversine.go` | `HaversineNM` — great-circle distance in nautical miles |
| `bearing.go` | `BearingDegrees` — initial great-circle bearing in [0, 360) |
| `wind.go` _(new)_ | `HeadwindCrosswind` — decomposes wind into headwind/crosswind components relative to a runway heading |

### Fixed

#### `pkg/convert`

- **`IsICAOCode`** now correctly accepts `R` and `S` prefixes — Japan (`RJTT`, `RJAA`), Korea (`RKSI`), Philippines (`RPLL`), and South America (`SBGR`, `SCEL`, `SKBO`, `SEQM`) were previously rejected. Dead code and contradictory guard logic cleaned up.
- **Mach KPH constant corrected** — `KilometersPerHourToMach`/`MachToKilometersPerHour` now derive from `mach1Knots * 1.852`, restoring mathematical closure: `kts → mach → kph` now equals `kts → kph` exactly.

### Other Changes

- `//go:build windows` removed from `pkg/calc` and `pkg/convert` — both are pure math packages with no DLL dependency (closes #132, #137).
- `pkg/calc/main.go` restructured into per-topic files (`haversine.go`, `bearing.go`, `wind.go`).
- Test files split into per-file structure in both packages.

Closes #183, #184, #185, #187.

---

## [0.3.3] - 2026-02-21

### Fixed

- **fix(website):** Scope `overflow-hidden` per marketing section instead of the root layout to restore sticky table-of-contents on documentation pages (#175).

  The initial approach (`overflow-x-hidden` on the root `<div>`) still broke `position:sticky` on the docs sidebar. The correct fix moves overflow containment to each individual marketing section, keeping the root layout clean.

> Patch release — no API or library changes. Go import paths and SDK behaviour are unchanged.

---

## [0.3.2] - 2026-02-20

### Fixed

- **Corrected field offsets** for 40/41-byte airport entry strides in `pkg/types/facility.go` and facility examples — offsets are now `12/20/28` (same as 36-byte stride), not the incorrect `16/24/32` introduced in v0.3.1.
- MSFS 2024 uses `char Ident[9]` (not `char[6]`), so layout is `ident[9] + region[3] = 12 bytes` before doubles with no alignment padding needed. Extra bytes in 41-byte stride are trailing data after altitude, not prefix padding.
- Removed incorrect `airportWire8` struct from all facility examples.
- Added MSFS 2024 ident size documentation to `pkg/types/facility.go`.

Affected examples: `read-facilities`, `all-facilities`, `subscribe-facilities`, `locate-airport`. Closes #119.

---

## [0.3.1] - 2026-02-20

### Fixed

- **Fixed airport facility entry alignment** — SimConnect (MSFS 2024) reports 41-byte entries in `SIMCONNECT_RECV_AIRPORT_LIST` responses but the parsing code used hardcoded offsets for a 36-byte layout. This caused garbled ICAO codes and invalid coordinates for entries 2+ in multi-entry batches. Replaced hardcoded offsets with a runtime switch on `actualEntrySize` supporting 33/36/40/41-byte layouts using `unsafe.Offsetof` for correct field positions (#117).
- Added stride warning comment to `SIMCONNECT_DATA_FACILITY_AIRPORT` in `pkg/types/facility.go`.

Affected examples: `read-facilities`, `all-facilities`, `subscribe-facilities`, `locate-airport`. Closes #118.

---

## [0.3.0] - 2026-02-20

Initial v0.3 milestone release. See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.3.0) for full notes.

---

## [0.2.1] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.2.1).

---

## [0.2.0] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.2.0).

---

## [0.1.2] - 2026-02-08

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.2).

---

## [0.1.1] - 2026-01-27

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.1).

---

## [0.1.0] - 2026-01-18

See [GitHub Release](https://github.com/mrlm-net/simconnect/releases/tag/v0.1.0).
