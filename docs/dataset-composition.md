---
title: "Dataset Composition"
description: "Clone, Merge, Builder, and Registry APIs for constructing and combining datasets."
order: 5
section: "datasets"
---

# Dataset Composition

The `pkg/datasets` package provides four complementary APIs for constructing and combining datasets without touching raw `DataDefinition` slices directly: `Clone`, `Merge`, `Builder`, and the global `Registry`.

## Clone

`Clone` returns an independent deep copy of a `DataSet` value.

```go
import "github.com/mrlm-net/simconnect/pkg/datasets"

original := datasets.DataSet{
    Definitions: []datasets.DataDefinition{
        {Name: "PLANE LATITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64},
        {Name: "PLANE LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64},
    },
}

clone := original.Clone()
```

### Immutability contract

`DataSet` is a value type — it holds a slice header, not a pointer. Assigning one `DataSet` to another copies the header (pointer, length, capacity) but not the backing array, so both variables still share the same underlying memory. `Clone` allocates a fresh backing array and copies all elements into it, breaking that shared reference.

After `Clone`, appending to or modifying a field in `clone.Definitions` has no effect on `original`, and vice versa.

### When to use Clone

Use `Clone` when you need to hand a dataset to code that may mutate it (for example, passing it to a library that appends or reorders definitions) while keeping the source dataset unchanged. Also use it when storing a snapshot of a dataset for later comparison or rollback.

Note: because `Clone` is a value receiver method, you can call it on any `DataSet` — including ones returned from constructor functions — without a pointer:

```go
ds := traffic.NewAircraftDataset()    // *DataSet
snapshot := ds.Clone()                // independent copy
```

## Merge

`Merge` combines any number of `DataSet` arguments into a single new `DataSet`, with last-wins deduplication by `Name`.

```go
merged := datasets.Merge(baseDS, extensionDS)
```

### Deduplication and position shift

When the same `Name` appears in more than one input dataset, the earlier occurrence is removed and the later occurrence is placed at the position it was last seen. Relative order among surviving (non-duplicate) entries is preserved.

Example:

```
Input A: [PLANE LATITUDE, PLANE LONGITUDE, PLANE ALTITUDE]
Input B: [PLANE LONGITUDE, AIRSPEED INDICATED]

Result:  [PLANE LATITUDE, PLANE ALTITUDE, PLANE LONGITUDE, AIRSPEED INDICATED]
```

`PLANE LONGITUDE` from `A` is removed because it reappears in `B`; the `B` copy occupies the position of its last occurrence (after `PLANE ALTITUDE`).

### Zero and single argument behaviour

- `Merge()` with zero arguments returns an empty `DataSet`.
- `Merge(ds)` with a single argument is equivalent to `ds.Clone()` — it returns a fresh independent copy.

### Independence guarantee

The returned `DataSet` always owns its own backing array. Mutations to the result do not affect any of the input datasets.

```go
a := datasets.DataSet{Definitions: []datasets.DataDefinition{
    {Name: "PLANE LATITUDE",  Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64},
    {Name: "PLANE LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64},
    {Name: "PLANE ALTITUDE",  Unit: "feet",    Type: types.SIMCONNECT_DATATYPE_FLOAT64},
}}
b := datasets.DataSet{Definitions: []datasets.DataDefinition{
    {Name: "PLANE LONGITUDE",   Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64},
    {Name: "AIRSPEED INDICATED", Unit: "knots",  Type: types.SIMCONNECT_DATATYPE_FLOAT64},
}}

result := datasets.Merge(a, b)
// result.Definitions: [PLANE LATITUDE, PLANE ALTITUDE, PLANE LONGITUDE, AIRSPEED INDICATED]
```

## Builder

`Builder` provides a fluent API for constructing `DataSet` values incrementally. It is useful when you want to assemble a dataset programmatically or extend an existing one without mutating the source.

```go
import "github.com/mrlm-net/simconnect/pkg/datasets"
import "github.com/mrlm-net/simconnect/pkg/types"

ds := datasets.NewBuilder().
    AddField("PLANE LATITUDE",  "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
    AddField("PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
    AddField("PLANE ALTITUDE",  "feet",    types.SIMCONNECT_DATATYPE_FLOAT64, 0).
    Build()
```

### AddField vs Add

`AddField` is the convenience form — it takes individual parameters and constructs the `DataDefinition` internally:

```go
b.AddField("PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0)
```

`Add` accepts a pre-built `DataDefinition` and is useful when you already have a value or want to share a definition across multiple builders:

```go
latDef := datasets.DataDefinition{
    Name: "PLANE LATITUDE", Unit: "degrees",
    Type: types.SIMCONNECT_DATATYPE_FLOAT64,
}
b.Add(latDef)
```

Both methods return the receiver for chaining.

### Remove

`Remove` removes the first definition whose `Name` matches the argument. If no match is found, it is a no-op — it does not panic:

```go
b.Remove("PLANE ALTITUDE")     // removes if present, ignored if absent
```

### Repeatable Build

`Build` is non-destructive. Each call takes an independent snapshot of the builder's current state, so you can call `Build` multiple times to produce independent datasets at different stages:

```go
b := datasets.NewBuilder().
    AddField("PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
    AddField("PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0)

posOnly := b.Build()    // [PLANE LATITUDE, PLANE LONGITUDE]

b.AddField("PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0)
withAlt := b.Build()    // [PLANE LATITUDE, PLANE LONGITUDE, PLANE ALTITUDE]

// posOnly is unchanged — it still has only two fields
```

### Aliasing safety

`Build` uses `append([]DataDefinition{}, b.definitions...)` internally, which always allocates a new backing array. The returned `DataSet` never shares memory with the builder or any previously built `DataSet`, so caller mutations are isolated.

### Reset

`Reset` removes all definitions from the builder and returns the receiver for chaining. Use it to reuse a builder across multiple construction cycles without allocating a new one:

```go
b := datasets.NewBuilder()

b.AddField("PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0)
snapshot1 := b.Build()

b.Reset().
    AddField("AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0)
snapshot2 := b.Build()
// snapshot1 and snapshot2 are independent; snapshot1 is unaffected by Reset
```

### Len

`Len` returns the number of definitions currently held in the builder. It is useful for guard checks before calling `Build`:

```go
if b.Len() == 0 {
    return fmt.Errorf("dataset builder is empty")
}
ds := b.Build()
```

## Registry

The global registry allows sub-packages to advertise named datasets via `init()` side-effects. Callers discover available datasets at runtime through the registry API.

### Naming convention

Dataset names follow the `"<category>/<descriptor>"` format, for example `"traffic/aircraft"`. The category groups related datasets and is used by `ListByCategory`. The name uniquely identifies a dataset across the entire process.

### Registration via import side-effect

Sub-packages call `datasets.Register` in their `init()` function. Importing the sub-package triggers `init()`, which registers the dataset. No explicit registration call is needed in application code:

```go
import (
    "github.com/mrlm-net/simconnect/pkg/datasets"
    _ "github.com/mrlm-net/simconnect/pkg/datasets/traffic"  // side-effect: registers "traffic/aircraft"
)
```

The blank import `_` is the Go idiom for importing a package solely for its `init()` side-effects.

### Get

`Get` retrieves the constructor function for a named dataset. The constructor returns a fresh `*DataSet` on every call:

```go
ctor, ok := datasets.Get("traffic/aircraft")
if !ok {
    log.Fatal("traffic/aircraft not registered")
}
ds := ctor()   // fresh *DataSet
```

Always check `ok` before calling the constructor. A missing name returns `nil, false`.

### List and Categories

`List` returns the names of all registered datasets sorted alphabetically. `Categories` returns the distinct category names sorted alphabetically. Both are useful for introspection and tooling:

```go
fmt.Println("Registered datasets:", datasets.List())
fmt.Println("Categories:", datasets.Categories())
```

### ListByCategory

`ListByCategory` narrows the listing to a single category. Returns `nil` if no datasets are registered under that category:

```go
trafficDatasets := datasets.ListByCategory("traffic")
for _, name := range trafficDatasets {
    fmt.Println(name)
}
```

### Concurrent safety

All registry operations (`Register`, `Get`, `List`, `Categories`, `ListByCategory`) are safe for concurrent use. The registry uses a `sync.RWMutex` internally: reads (Get, List, Categories, ListByCategory) acquire a read lock; writes (Register) acquire an exclusive write lock.

### Panics on empty name or category

`Register` panics if either `name` or `category` is an empty string. This is an intentional fail-fast guard against misconfigured `init()` calls that would otherwise silently produce unqueryable entries:

```go
// panics: name must not be empty
datasets.Register("", "traffic", constructor)

// panics: category must not be empty
datasets.Register("traffic/aircraft", "", constructor)
```

### Overwrite behaviour

Registering the same name twice silently overwrites the previous entry. This allows test code and alternative implementations to replace a built-in dataset without ceremony.

## End-to-end example

The following snippet ties together all four APIs: import side-effect, registry discovery, `Clone`, `Builder`, and `Merge`.

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/mrlm-net/simconnect/pkg/datasets"
    _ "github.com/mrlm-net/simconnect/pkg/datasets/traffic" // registers "traffic/aircraft"
    "github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
    // 1. Discover what is registered.
    fmt.Println("Registered datasets:", datasets.List())
    fmt.Println("Categories:", datasets.Categories())

    // 2. Retrieve the traffic/aircraft constructor from the registry.
    ctor, ok := datasets.Get("traffic/aircraft")
    if !ok {
        panic("traffic/aircraft not registered")
    }
    trafficDS := ctor() // fresh *DataSet from the constructor

    // 3. Clone to get an independent copy before extending.
    extended := trafficDS.Clone()

    // 4. Build a minimal position dataset using the fluent builder.
    posDS := datasets.NewBuilder().
        AddField("PLANE LATITUDE",  "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
        AddField("PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
        AddField("PLANE ALTITUDE",  "feet",    types.SIMCONNECT_DATATYPE_FLOAT64, 0).
        Build()

    // 5. Merge the cloned traffic dataset with the position dataset.
    //    Duplicate names (PLANE LATITUDE, PLANE LONGITUDE, PLANE ALTITUDE)
    //    are deduplicated with last-wins: the posDS versions replace the
    //    traffic dataset's versions, shifting to the end.
    merged := datasets.Merge(extended, posDS)

    fmt.Printf("Traffic fields:  %d\n", len(trafficDS.Definitions))
    fmt.Printf("Position fields: %d\n", len(posDS.Definitions))
    fmt.Printf("Merged fields:   %d\n", len(merged.Definitions))
}
```

## See Also

- [Engine/Client Usage](usage-client.md) — `RegisterDataset` for submitting a dataset to SimConnect
- [Manager Usage](usage-manager.md) — dataset registration in the managed connection lifecycle
