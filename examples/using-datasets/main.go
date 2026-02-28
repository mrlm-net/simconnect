//go:build windows
// +build windows

// Package main demonstrates dataset composition and registry APIs.
//
// It shows how to:
//   - Auto-register datasets via blank import side-effect
//   - Discover registered datasets with List, Categories, and ListByCategory
//   - Retrieve a dataset constructor from the registry with Get
//   - Clone a dataset to produce an independent copy
//   - Build a custom dataset with the fluent Builder
//   - Merge datasets with last-wins deduplication
//
// The example connects to the simulator but does NOT request SimVar data.
// Its purpose is to demonstrate the composition APIs clearly; the SimConnect
// connection is included only to show that the merged dataset is ready for
// use with RegisterDataset when a simulator session is available.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/datasets"
	_ "github.com/mrlm-net/simconnect/pkg/datasets/traffic" // side-effect: registers "traffic/aircraft"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("Received interrupt, shutting down...")
		cancel()
	}()

	// --- Registry discovery ---------------------------------------------------
	// The blank import of the traffic package triggered its init(), which called
	// datasets.Register("traffic/aircraft", "traffic", NewAircraftDataset).
	// We can now discover it without knowing its name up front.

	fmt.Println("=== Registry ===")
	fmt.Println("All registered datasets:", datasets.List())
	fmt.Println("All categories:", datasets.Categories())
	fmt.Println("Datasets in 'traffic' category:", datasets.ListByCategory("traffic"))

	// --- Get: retrieve constructor from registry ------------------------------

	ctor, ok := datasets.Get("traffic/aircraft")
	if !ok {
		fmt.Fprintln(os.Stderr, "ERROR: traffic/aircraft not found in registry")
		os.Exit(1)
	}
	trafficDS := ctor() // fresh *DataSet on every call
	fmt.Printf("\n=== traffic/aircraft dataset (%d fields) ===\n", len(trafficDS.Definitions))
	for i, def := range trafficDS.Definitions {
		fmt.Printf("  [%2d] %-40s  unit=%-20s  type=%d\n", i, def.Name, def.Unit, def.Type)
	}

	// --- Clone ----------------------------------------------------------------
	// Clone returns an independent deep copy.  Mutations to the clone do not
	// affect trafficDS and vice versa.

	clonedDS := trafficDS.Clone()
	fmt.Printf("\n=== Clone (%d fields, independent copy) ===\n", len(clonedDS.Definitions))

	// --- Builder --------------------------------------------------------------
	// Build a small supplementary dataset with fields not in the traffic set,
	// plus one overlapping field (PLANE ALTITUDE) to show Merge deduplication.

	supplementary := datasets.NewBuilder().
		AddField("PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.5).          // overlaps traffic
		AddField("AMBIENT TEMPERATURE", "celsius", types.SIMCONNECT_DATATYPE_FLOAT64, 0).    // new
		AddField("AMBIENT WIND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0).    // new
		AddField("AMBIENT WIND DIRECTION", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0). // new
		Build()

	fmt.Printf("\n=== Supplementary dataset (Builder, %d fields) ===\n", len(supplementary.Definitions))
	for i, def := range supplementary.Definitions {
		fmt.Printf("  [%2d] %-40s  epsilon=%.1f\n", i, def.Name, def.Epsilon)
	}

	// Demonstrate repeatable Build and Remove.
	b := datasets.NewBuilder().
		AddField("PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
		AddField("PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0).
		AddField("PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0)

	posWithAlt := b.Build() // snapshot 1: three fields
	b.Remove("PLANE ALTITUDE")
	posOnly := b.Build() // snapshot 2: two fields (PLANE ALTITUDE removed)

	fmt.Printf("\n=== Builder: posWithAlt=%d fields, posOnly=%d fields ===\n",
		len(posWithAlt.Definitions), len(posOnly.Definitions))

	// --- Merge ----------------------------------------------------------------
	// Merge(clonedDS, supplementary) combines the two datasets with last-wins
	// deduplication.  PLANE ALTITUDE appears in both; the supplementary version
	// (epsilon=0.5) wins and shifts to its last-seen position.

	merged := datasets.Merge(clonedDS, supplementary)
	fmt.Printf("\n=== Merged dataset (%d fields) ===\n", len(merged.Definitions))
	fmt.Printf("  traffic: %d + supplementary: %d - 1 duplicate = %d expected\n",
		len(clonedDS.Definitions), len(supplementary.Definitions), len(merged.Definitions))
	for i, def := range merged.Definitions {
		fmt.Printf("  [%2d] %-40s  epsilon=%.1f\n", i, def.Name, def.Epsilon)
	}

	// --- Optional SimConnect registration ------------------------------------
	// Connect to the simulator (if running) and register the merged dataset to
	// show it is ready for real use.  The example exits immediately after
	// printing the connection result — it does not enter a data request loop.

	fmt.Println("\n=== SimConnect (optional) ===")
	client := simconnect.NewClient("GO Example - Dataset Composition",
		engine.WithContext(ctx),
	)

	if err := client.Connect(); err != nil {
		fmt.Printf("Simulator not running (%v) — skipping registration demo\n", err)
	} else {
		const mergedDefID = 1000
		client.RegisterDataset(mergedDefID, &merged)
		fmt.Printf("Registered merged dataset under define ID %d\n", mergedDefID)
		_ = client.Disconnect()
	}

	cancel()
	fmt.Println("\nDone.")
}
