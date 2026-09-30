# Using Datasets Example

## Overview

This example shows the dataset composition and registry APIs of `pkg/datasets`: discovering registered datasets, getting one from the registry, cloning it, building a dataset with the fluent builder, and merging datasets. It prints each step. The simulator is optional: if it is running, the merged dataset is registered once to show it is ready for use; no SimVar data is requested.

## What It Does

1. **Auto-registers a dataset** — The blank import of `pkg/datasets/traffic` runs its `init()`, which registers `traffic/aircraft` in the `traffic` category
2. **Discovers the registry** — Prints `datasets.List()`, `datasets.Categories()` and `datasets.ListByCategory("traffic")`
3. **Gets a dataset** — `datasets.Get("traffic/aircraft")` returns its constructor; calling it gives a fresh `*DataSet`, whose fields are printed
4. **Clones it** — `Clone()` returns an independent deep copy
5. **Builds datasets** — `datasets.NewBuilder().AddField(...).Build()` makes a supplementary dataset (ambient temperature and wind, plus `PLANE ALTITUDE` with epsilon 0.5 to overlap the traffic set); a second builder shows that `Build()` can be called again after `Remove`
6. **Merges them** — `datasets.Merge(clone, supplementary)`: duplicates are removed last-wins, so the builder's `PLANE ALTITUDE` replaces the traffic one
7. **Registers the result (optional)** — Connects once; if the simulator is running, `client.RegisterDataset(1000, &merged)`, then disconnects and exits. Without the simulator it says so and skips this step

## Prerequisites

- Windows OS (SimConnect is Windows-only)
- Microsoft Flight Simulator 2020/2024 only for the last step

## Running the Example

```bash
go run ./examples/using-datasets
```

The program runs once and exits.

## Expected Output

```
=== Registry ===
All registered datasets: [traffic/aircraft]
All categories: [traffic]
Datasets in 'traffic' category: [traffic/aircraft]

=== traffic/aircraft dataset (20 fields) ===
  [ 0] TITLE                                     unit=                      type=...
  ...

=== Clone (20 fields, independent copy) ===

=== Supplementary dataset (Builder, 4 fields) ===
  [ 0] PLANE ALTITUDE                            epsilon=0.5
  [ 1] AMBIENT TEMPERATURE                       epsilon=0.0
  ...

=== Builder: posWithAlt=3 fields, posOnly=2 fields ===

=== Merged dataset (23 fields) ===
  traffic: 20 + supplementary: 4 - 1 duplicate = 23 expected
  ...

=== SimConnect (optional) ===
Registered merged dataset under define ID 1000

Done.
```

The field counts depend on the traffic dataset's current definition.

## Files

- `main.go` — The example.
- `planes.json` and `plans/` — Not used by `main.go`; left over from an earlier version of this example that spawned AI traffic. Spawning from a `planes.json` is shown in [`ai-traffic`](../ai-traffic).

## See Also

- [Dataset Composition](../../docs/dataset-composition.md) — Registry, builder, clone and merge
- [Datasets](../../docs/usage-datasets.md) — The ready-made datasets
- [`pkg/registry`](../../docs/pkg-registry.md)
