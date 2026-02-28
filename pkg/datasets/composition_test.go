//go:build windows
// +build windows

package datasets

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// helpers

func def(name string) DataDefinition {
	return DataDefinition{Name: name, Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0}
}

func names(ds DataSet) []string {
	out := make([]string, len(ds.Definitions))
	for i, d := range ds.Definitions {
		out[i] = d.Name
	}
	return out
}

// Clone tests

func TestClone_IndependentSlice(t *testing.T) {
	original := DataSet{Definitions: []DataDefinition{def("A"), def("B")}}
	clone := original.Clone()

	// Appending to clone must not grow original.
	clone.Definitions = append(clone.Definitions, def("C"))
	if len(original.Definitions) != 2 {
		t.Fatalf("append to clone grew original: got %d, want 2", len(original.Definitions))
	}
}

func TestClone_IndependentFieldMutation(t *testing.T) {
	original := DataSet{Definitions: []DataDefinition{def("A")}}
	clone := original.Clone()

	clone.Definitions[0].Name = "Z"
	if original.Definitions[0].Name != "A" {
		t.Fatalf("field mutation in clone affected original: got %q, want %q", original.Definitions[0].Name, "A")
	}
}

func TestClone_EmptyDataSet(t *testing.T) {
	original := DataSet{}
	clone := original.Clone()
	if clone.Definitions == nil {
		// nil and empty are both acceptable; just verify length is 0.
	}
	if len(clone.Definitions) != 0 {
		t.Fatalf("clone of empty dataset should have 0 definitions, got %d", len(clone.Definitions))
	}
}

// Merge tests

func TestMerge_ZeroArgs(t *testing.T) {
	result := Merge()
	if len(result.Definitions) != 0 {
		t.Fatalf("Merge() with zero args: expected 0 definitions, got %d", len(result.Definitions))
	}
}

func TestMerge_SingleArg_EquivalentToClone(t *testing.T) {
	ds := DataSet{Definitions: []DataDefinition{def("A"), def("B"), def("C")}}
	result := Merge(ds)

	got := names(result)
	want := []string{"A", "B", "C"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Merge(single): got %v, want %v", got, want)
	}

	// Must be independent.
	result.Definitions[0].Name = "Z"
	if ds.Definitions[0].Name != "A" {
		t.Fatalf("Merge(single) result shares backing array with input")
	}
}

func TestMerge_Deduplication_LastWins(t *testing.T) {
	// [A,B,C] + [B,D] → [A,C,B,D]
	ds1 := DataSet{Definitions: []DataDefinition{def("A"), def("B"), def("C")}}
	ds2 := DataSet{Definitions: []DataDefinition{def("B"), def("D")}}

	result := Merge(ds1, ds2)
	got := names(result)
	want := []string{"A", "C", "B", "D"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Merge dedup last-wins: got %v, want %v", got, want)
	}
}

func TestMerge_NoOverlap_PreservesOrder(t *testing.T) {
	ds1 := DataSet{Definitions: []DataDefinition{def("A"), def("B")}}
	ds2 := DataSet{Definitions: []DataDefinition{def("C"), def("D")}}

	result := Merge(ds1, ds2)
	got := names(result)
	want := []string{"A", "B", "C", "D"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Merge no-overlap: got %v, want %v", got, want)
	}
}

func TestMerge_TripleDuplicate(t *testing.T) {
	// [A,B] + [B] + [B,C] → [A,B,C]  (last B at end)
	ds1 := DataSet{Definitions: []DataDefinition{def("A"), def("B")}}
	ds2 := DataSet{Definitions: []DataDefinition{def("B")}}
	ds3 := DataSet{Definitions: []DataDefinition{def("B"), def("C")}}

	result := Merge(ds1, ds2, ds3)
	got := names(result)
	want := []string{"A", "B", "C"}
	if !equalStringSlices(got, want) {
		t.Fatalf("Merge triple dup: got %v, want %v", got, want)
	}
}

func TestMerge_ResultIsIndependent(t *testing.T) {
	ds1 := DataSet{Definitions: []DataDefinition{def("A")}}
	result := Merge(ds1)
	result.Definitions[0].Name = "Z"
	if ds1.Definitions[0].Name != "A" {
		t.Fatalf("Merge result shares backing array with input")
	}
}

// equalStringSlices is a simple helper; avoid importing reflect in tests.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
