---
title: "SimVar Registry"
description: "pkg/registry — cross-platform typed SimVar metadata catalogue"
section: "packages"
order: 10
---

# SimVar Registry

The `pkg/registry` package is a static, typed metadata store for MSFS SimVar names, accepted unit strings, Go data types, writability, and category information. It gives you a way to validate variable names and units at startup, enumerate all known variables by category or unit, and discover index-aware variables — all without requiring a live SimConnect connection.

The registry contains 104 entries across three categories: `"aircraft"` (46 vars), `"environment"` (16 vars), and `"simulator"` (42 vars). It has no build-tag requirements and no imports from Windows-gated packages, so it compiles and runs on any platform.

## Import

```go
import "github.com/mrlm-net/simconnect/pkg/registry"
```

No `//go:build windows` tag is needed. The package is safe to import in test utilities, validation tools, or code generators running on Linux or macOS.

## SimVarMeta Struct

Every entry in the registry is a `SimVarMeta` value:

```go
type SimVarMeta struct {
    Name        string   // Canonical SimConnect name, uppercase, no :N suffix
    Units       []string // All accepted unit strings, lowercase canonical form
    DefaultUnit string   // Recommended unit for new data definitions
    Type        string   // Go numeric type: "float64", "float32", "int32",
                         //   "int64", "bool", "string", "enum"
    Category    string   // Domain group: "aircraft", "environment", "simulator"
    Writable    bool     // Whether SimConnect accepts SetDataOnSimObject writes
    Indexed     bool     // Whether the variable accepts a :N suffix
    Description string   // Human-readable summary
}
```

| Field | Description |
|-------|-------------|
| `Name` | Canonical SimConnect variable name in upper case without any `:N` index suffix. For example: `"ENG RPM"`, not `"ENG RPM:1"`. |
| `Units` | All unit strings SimConnect accepts for this variable, stored in lowercase. For example: `["feet", "meters"]`. |
| `DefaultUnit` | The primary unit recommended for new data definitions. Always one of the values in `Units`. |
| `Type` | The Go numeric type appropriate for reading this variable. Valid values: `"float64"`, `"float32"`, `"int32"`, `"int64"`, `"bool"`, `"string"`, `"enum"`. |
| `Category` | Domain group. Valid values in this release: `"aircraft"`, `"environment"`, `"simulator"`. |
| `Writable` | `true` if SimConnect accepts writes via `SetDataOnSimObject` for this variable. |
| `Indexed` | `true` if this variable accepts a `:N` suffix to address engine, radio, or other per-instance slots (e.g., `"ENG RPM:1"`). |
| `Description` | Human-readable summary of what the variable represents. |

## Lookup

`Lookup` retrieves a single entry by name. The lookup is case-insensitive, and any `:N` index suffix is stripped before the match.

**Signature:**

```go
func Lookup(name string) (SimVarMeta, bool)
```

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    // Exact name
    if meta, ok := registry.Lookup("PLANE ALTITUDE"); ok {
        fmt.Printf("Name:        %s\n", meta.Name)
        fmt.Printf("DefaultUnit: %s\n", meta.DefaultUnit)
        fmt.Printf("Type:        %s\n", meta.Type)
        fmt.Printf("Writable:    %v\n", meta.Writable)
    }

    // Case-insensitive
    if meta, ok := registry.Lookup("plane altitude"); ok {
        fmt.Println("Found:", meta.Name) // PLANE ALTITUDE
    }

    // :N suffix is stripped automatically
    if meta, ok := registry.Lookup("ENG RPM:2"); ok {
        fmt.Println("Found:", meta.Name)    // ENG RPM
        fmt.Println("Indexed:", meta.Indexed) // true
    }

    // Unknown variable
    if _, ok := registry.Lookup("UNKNOWN VAR"); !ok {
        fmt.Println("Variable not found in registry")
    }
}
```

## All

`All` returns an independent copy of every entry in declaration order. Modifying the returned slice does not affect registry state.

**Signature:**

```go
func All() []SimVarMeta
```

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    vars := registry.All()
    fmt.Printf("Total SimVars: %d\n", len(vars))

    for _, meta := range vars {
        fmt.Printf("%-40s  %-12s  %-11s  writable=%-5v  indexed=%v\n",
            meta.Name,
            meta.DefaultUnit,
            meta.Type,
            meta.Writable,
            meta.Indexed,
        )
    }
}
```

Because `All` returns a copy, each call allocates a new slice. Call it once and reuse the result if you need to iterate multiple times in a hot path.

## Validate

`Validate` checks whether a given unit string is valid for a named variable. It returns `nil` on success and a descriptive error on failure. Both the variable name and unit comparisons are case-insensitive. A `:N` index suffix is stripped from the name before lookup.

**Signature:**

```go
func Validate(name, unit string) error
```

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    // Valid combination — returns nil
    err := registry.Validate("PLANE ALTITUDE", "feet")
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("OK")
    }

    // Invalid unit — error lists accepted values
    err = registry.Validate("PLANE ALTITUDE", "knots")
    if err != nil {
        fmt.Println(err)
        // registry: unit "knots" not valid for PLANE ALTITUDE; valid units: feet, meters
    }

    // Unknown variable
    err = registry.Validate("UNKNOWN VAR", "feet")
    if err != nil {
        fmt.Println(err)
        // registry: unknown SimVar: UNKNOWN VAR
    }

    // Validate before registering a data definition
    const varName = "AIRSPEED INDICATED"
    const unit = "knots"
    if err := registry.Validate(varName, unit); err != nil {
        panic("invalid SimVar configuration: " + err.Error())
    }
    // safe to proceed with client.AddToDataDefinition(defID, varName, unit, ...)
}
```

`Validate` is most useful as a startup guard. Call it during initialization to catch unit typos before the application reaches the simulator.

## ByUnit

`ByUnit` returns all entries whose `Units` slice contains the given unit string. The comparison is case-insensitive. Returns `nil` if no entries match.

**Signature:**

```go
func ByUnit(unit string) []SimVarMeta
```

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    // Find all SimVars that accept "degrees" as a unit
    degreeVars := registry.ByUnit("degrees")
    fmt.Printf("SimVars accepting 'degrees': %d\n", len(degreeVars))

    for _, meta := range degreeVars {
        fmt.Printf("  %-45s  default=%-10s  category=%s\n",
            meta.Name,
            meta.DefaultUnit,
            meta.Category,
        )
    }

    // Case-insensitive
    knotsVars := registry.ByUnit("Knots")
    fmt.Printf("\nSimVars accepting 'knots': %d\n", len(knotsVars))
    for _, meta := range knotsVars {
        fmt.Printf("  %s\n", meta.Name)
    }
}
```

## ByCategory

`ByCategory` returns all entries in a given domain category. The comparison is case-insensitive. Returns `nil` if no entries match.

**Signature:**

```go
func ByCategory(category string) []SimVarMeta
```

Valid categories: `"aircraft"`, `"environment"`, `"simulator"`.

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    // List all aircraft variables
    aircraftVars := registry.ByCategory("aircraft")
    fmt.Printf("Aircraft SimVars: %d\n", len(aircraftVars))

    for _, meta := range aircraftVars {
        fmt.Printf("  %-40s  %-10s  writable=%v\n",
            meta.Name,
            meta.DefaultUnit,
            meta.Writable,
        )
    }

    // Environment variables
    envVars := registry.ByCategory("environment")
    fmt.Printf("\nEnvironment SimVars: %d\n", len(envVars))

    // Simulator variables
    simVars := registry.ByCategory("simulator")
    fmt.Printf("Simulator SimVars: %d\n", len(simVars))
}
```

| Category | Count | Contents |
|----------|-------|----------|
| `"aircraft"` | 46 | Position, attitude, speed, altitude, engine, controls, gear, fuel, avionics |
| `"environment"` | 16 | Weather, wind, temperature, pressure, visibility, precipitation |
| `"simulator"` | 42 | Simulation state, camera, time, realism, units, VR, avatar |

## Indexed Variables

Some SimVars address per-instance data — such as individual engines, radios, or throttle levers — via a `:N` integer suffix. The registry represents these with `Indexed: true` on the base name (no suffix).

Indexed variables in this release:

| Name | Description |
|------|-------------|
| `ENG RPM` | Engine RPM for engine N (`:1`–`:4`) |
| `ENG FUEL FLOW GPH` | Fuel flow in gallons/hour for engine N |
| `GENERAL ENG THROTTLE LEVER POSITION` | Throttle lever position for engine N |
| `FUEL SELECTED QUANTITY` | Quantity in fuel tank N |
| `COM ACTIVE FREQUENCY` | Active frequency for COM radio N |
| `NAV OBS` | OBS setting for NAV radio N |
| `CAMERA VIEW TYPE INDEX` | Camera view type index N |

To look up an indexed variable, pass the name with any `:N` suffix — `Lookup` strips it automatically:

```go
package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
)

func main() {
    // Look up by base name
    if meta, ok := registry.Lookup("ENG RPM"); ok {
        fmt.Println("Indexed:", meta.Indexed) // true
    }

    // :N suffix is stripped — same result
    if meta, ok := registry.Lookup("ENG RPM:3"); ok {
        fmt.Println("Name:", meta.Name)        // ENG RPM
        fmt.Println("Indexed:", meta.Indexed)  // true
    }

    // Validate a specific engine unit before registering
    for engine := 1; engine <= 4; engine++ {
        varName := fmt.Sprintf("ENG RPM:%d", engine)
        if err := registry.Validate(varName, "rpm"); err != nil {
            fmt.Printf("Engine %d: %v\n", engine, err)
        }
    }
}
```

When building a data definition for an indexed variable, pass the full `:N` name to `AddToDataDefinition` — SimConnect requires the suffix to identify the correct instance. The registry validates the base name and units; SimConnect resolves the index at runtime.

```go
import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/registry"
    "github.com/mrlm-net/simconnect/pkg/types"
)

const EngineDefID = 2000

// Validate all four engine slots at startup
for i := 1; i <= 4; i++ {
    name := fmt.Sprintf("ENG RPM:%d", i)
    if err := registry.Validate(name, "rpm"); err != nil {
        panic(err)
    }
    client.AddToDataDefinition(EngineDefID, name, "rpm",
        types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
}
```

## Cross-Platform Usage

The `pkg/registry` package has no build tags and no imports from Windows-gated packages. It compiles and runs correctly on Linux, macOS, and Windows. This makes it useful in contexts where the full SimConnect stack is unavailable:

- **Validation tools** — check SimVar configurations in CI pipelines on any OS
- **Code generators** — enumerate variables to scaffold struct definitions
- **Test utilities** — assert that dataset fields use valid unit strings without requiring a Windows environment

```go
// This import works on all platforms — no //go:build windows needed
import "github.com/mrlm-net/simconnect/pkg/registry"

// Safe to call on Linux/macOS in tests, build tools, or generators
func validateDatasetConfig(vars []struct{ Name, Unit string }) error {
    for _, v := range vars {
        if err := registry.Validate(v.Name, v.Unit); err != nil {
            return err
        }
    }
    return nil
}
```

All five exported functions (`Lookup`, `All`, `Validate`, `ByUnit`, `ByCategory`) are safe for concurrent use. Package state is initialized once in `init()` and is never mutated after that.

## See Also

- [Using Datasets](usage-datasets.md) — Pre-built dataset constructors for aircraft, environment, and simulator data
- [Client Usage](usage-client.md) — `AddToDataDefinition` and `RegisterDataset` API
- [Manager Usage](usage-manager.md) — Manager-level dataset registration
