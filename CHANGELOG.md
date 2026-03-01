# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
