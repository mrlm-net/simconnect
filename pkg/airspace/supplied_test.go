package airspace

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func ids(list []*Airspace) []uint64 {
	out := make([]uint64, len(list))
	for i, a := range list {
		out[i] = a.ID
	}
	return out
}

// TestSuppliedStore: the LKPR records of the tiles, fed in as a slice (as
// the simulator's JSON, each listed twice as a nearest search may) or
// fetched, give the same Area, At and ClassAt as the tile store, with
// StorePreferNewerSet and StoreSkipBroken alike (#1260); a fetch store asks
// once for a box and not again for one inside it.
func TestSuppliedStore(t *testing.T) {
	area := BoxAround(lkpr.Lat, lkpr.Lon, 60000)
	inner := BoxAround(lkpr.Lat, lkpr.Lon, 20000)
	opts := []StoreOption{StorePreferNewerSet(), StoreSkipBroken()}
	tiles := store(t, opts...)
	want, err := store(t).Area(area) // every record, broken and older sets too
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(append(slices.Clone(want), want...))
	if err != nil {
		t.Fatal(err)
	}
	var decoded []Airspace
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	records := make([]*Airspace, len(decoded))
	for i := range decoded {
		records[i] = &decoded[i]
	}
	calls := 0
	fetch := func(box Box) ([]*Airspace, error) {
		calls++
		return store(t).Area(box)
	}
	for name, s := range map[string]*Store{"supplied": NewStore(records, opts...), "fetched": NewFetchStore(fetch, opts...)} {
		for _, q := range []Box{area, inner} {
			w, _ := tiles.Area(q)
			got, err := s.Area(q)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !slices.Equal(ids(got), ids(w)) {
				t.Errorf("%s Area: %d records, the tiles %d", name, len(got), len(w))
			}
		}
		w, _ := tiles.At(lkpr.Lat, lkpr.Lon)
		got, _ := s.At(lkpr.Lat, lkpr.Lon)
		if len(w) == 0 || !slices.Equal(ids(got), ids(w)) {
			t.Errorf("%s At LKPR: %v, the tiles %v", name, ids(got), ids(w))
		}
		wc, _ := tiles.ClassAt(lkpr.Lat, lkpr.Lon, 600, 380)
		gc, _ := s.ClassAt(lkpr.Lat, lkpr.Lon, 600, 380)
		if wc == nil || gc == nil || wc.ID != gc.ID {
			t.Errorf("%s ClassAt LKPR 600 m: %v, the tiles %v", name, gc, wc)
		}
	}
	if calls != 1 {
		t.Errorf("fetched %d times for a box and one inside it, want once", calls)
	}
	failing := NewFetchStore(func(Box) ([]*Airspace, error) { return nil, errors.New("no simulator") })
	if _, err := failing.Area(inner); err == nil {
		t.Error("a failed fetch answered")
	}
}
