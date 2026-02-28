//go:build windows
// +build windows

package datasets

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func TestBuilder_EmptyBuild(t *testing.T) {
	b := NewBuilder()
	ds := b.Build()
	if len(ds.Definitions) != 0 {
		t.Fatalf("empty builder Build(): expected 0 definitions, got %d", len(ds.Definitions))
	}
}

func TestBuilder_Add_FluentChaining(t *testing.T) {
	b := NewBuilder().
		Add(def("A")).
		Add(def("B")).
		Add(def("C"))

	ds := b.Build()
	got := names(ds)
	want := []string{"A", "B", "C"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Add chaining: got %v, want %v", got, want)
	}
}

func TestBuilder_AddField_FluentChaining(t *testing.T) {
	b := NewBuilder().
		AddField("Altitude", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.5).
		AddField("Speed", "knots", types.SIMCONNECT_DATATYPE_FLOAT32, 0)

	if b.Len() != 2 {
		t.Fatalf("AddField: expected Len 2, got %d", b.Len())
	}

	ds := b.Build()
	if ds.Definitions[0].Name != "Altitude" {
		t.Fatalf("AddField[0] name: got %q, want %q", ds.Definitions[0].Name, "Altitude")
	}
	if ds.Definitions[0].Epsilon != 0.5 {
		t.Fatalf("AddField[0] epsilon: got %v, want 0.5", ds.Definitions[0].Epsilon)
	}
	if ds.Definitions[1].Name != "Speed" {
		t.Fatalf("AddField[1] name: got %q, want %q", ds.Definitions[1].Name, "Speed")
	}
}

func TestBuilder_Remove_Existing(t *testing.T) {
	b := NewBuilder().Add(def("A")).Add(def("B")).Add(def("C"))
	b.Remove("B")

	got := names(b.Build())
	want := []string{"A", "C"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Remove existing: got %v, want %v", got, want)
	}
}

func TestBuilder_Remove_NonExistent_IsNoop(t *testing.T) {
	b := NewBuilder().Add(def("A")).Add(def("B"))
	b.Remove("Z") // not present

	got := names(b.Build())
	want := []string{"A", "B"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Remove non-existent: got %v, want %v", got, want)
	}
}

func TestBuilder_Remove_OnlyFirstOccurrence(t *testing.T) {
	b := NewBuilder().Add(def("A")).Add(def("A")).Add(def("B"))
	b.Remove("A")

	got := names(b.Build())
	want := []string{"A", "B"} // second A remains
	if !equalStringSlices(got, want) {
		t.Fatalf("Remove first occurrence: got %v, want %v", got, want)
	}
}

func TestBuilder_Build_IsRepeatable(t *testing.T) {
	b := NewBuilder().Add(def("X")).Add(def("Y"))
	ds1 := b.Build()
	ds2 := b.Build()

	if !equalStringSlices(names(ds1), names(ds2)) {
		t.Fatalf("Build() not repeatable: ds1=%v ds2=%v", names(ds1), names(ds2))
	}

	// Both must be independent of each other.
	ds1.Definitions[0].Name = "mutated"
	if ds2.Definitions[0].Name == "mutated" {
		t.Fatalf("two Build() calls share the same backing array")
	}
}

func TestBuilder_Build_AliasSafety(t *testing.T) {
	b := NewBuilder().Add(def("A")).Add(def("B"))
	ds := b.Build()

	// Add more to the builder after building.
	b.Add(def("C"))

	// The previously built DataSet must be unaffected.
	if len(ds.Definitions) != 2 {
		t.Fatalf("prior Build() result affected by subsequent Add: got %d definitions", len(ds.Definitions))
	}
}

func TestBuilder_Len(t *testing.T) {
	b := NewBuilder()
	if b.Len() != 0 {
		t.Fatalf("Len on empty builder: got %d, want 0", b.Len())
	}
	b.Add(def("A")).Add(def("B"))
	if b.Len() != 2 {
		t.Fatalf("Len after two Adds: got %d, want 2", b.Len())
	}
	b.Remove("A")
	if b.Len() != 1 {
		t.Fatalf("Len after Remove: got %d, want 1", b.Len())
	}
}

func TestBuilder_Reset(t *testing.T) {
	b := NewBuilder().Add(def("A")).Add(def("B"))
	b.Reset()

	if b.Len() != 0 {
		t.Fatalf("after Reset Len: got %d, want 0", b.Len())
	}

	ds := b.Build()
	if len(ds.Definitions) != 0 {
		t.Fatalf("Build after Reset: got %d definitions, want 0", len(ds.Definitions))
	}
}

func TestBuilder_Reset_Chaining(t *testing.T) {
	b := NewBuilder().Add(def("A")).Reset().Add(def("B"))
	got := names(b.Build())
	want := []string{"B"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Reset chaining: got %v, want %v", got, want)
	}
}
