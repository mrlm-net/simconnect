package airspace

import (
	"slices"
	"testing"
)

// TestCellsAsScan: Area by the index's cells picks exactly what a scan of
// the whole index does: Czechia, a point, across 180, a box too big for
// cells, the pole.
func TestCellsAsScan(t *testing.T) {
	s := store(t)
	for _, q := range []Box{
		{North: 51, South: 48.5, West: 12, East: 19},
		{North: 50.1008, South: 50.1008, West: 14.26, East: 14.26},
		BoxAround(65, -175, 50000),
		{North: 10, South: 0, West: 170, East: -170},
		{North: 60, South: 20, West: -20, East: 40},
		{North: 90, South: 85, West: -180, East: 180},
	} {
		var want []int32
		for i, e := range s.Index() {
			if e.Box().Overlaps(q) {
				want = append(want, int32(i))
			}
		}
		var got []int32
		for _, i := range s.candidates(q) {
			if s.index[i].Box().Overlaps(q) {
				got = append(got, i)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%+v: %d by cells, %d by a scan", q, len(got), len(want))
		}
	}
}

// TestClassAt (#1257): over LKPR the class is Ruzyne CTR's D to 3,500 ft,
// Prague TMA II's C above; 8 NM south, under the Prague CTA's floor of
// 1,000 ft AGL, no class record holds it.
func TestClassAt(t *testing.T) {
	s := store(t, StorePreferNewerSet(), StoreSkipBroken())
	const ground = 365.0 // LKPR, m
	for _, c := range []struct {
		lat, lon, altM float64
		name           string
		class          BoundaryType
	}{
		{lkpr.Lat, lkpr.Lon, 914, "RUZYNE CTR", ClassD},
		{lkpr.Lat, lkpr.Lon, 1219, "PRAGUE TMA II", ClassC},
		{49.9675, 14.26, 579, "", 0},
		{49.9675, 14.26, 701, "PRAGUE CTA 2", ClassE},
	} {
		a, err := s.ClassAt(c.lat, c.lon, c.altM, ground)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case c.name == "" && a != nil:
			t.Errorf("%v %.0f m: %s, want none", c, c.altM, a.Name)
		case c.name != "" && (a == nil || a.Type != c.class):
			t.Errorf("%v %.0f m: %v, want %s (%s)", c, c.altM, a, c.name, c.class)
		}
	}
}

// TestNoLimits: 2,020 records have both limits 0, 1,890 of them broken;
// Containing never returns one.
func TestNoLimits(t *testing.T) {
	s := store(t)
	all, err := s.Area(Box{North: 90, South: -90, West: -180, East: 180})
	if err != nil {
		t.Fatal(err)
	}
	n, broken := 0, 0
	for _, a := range all {
		if a.NoLimits() {
			n++
			if a.Broken() {
				broken++
			}
		}
	}
	if n != 2020 || broken != 1890 {
		t.Errorf("%d without limits, %d of them broken; want 2,020 and 1,890", n, broken)
	}
}

// TestPreferNewerSameName: PRAGUE CTA 2 (E) comes in two sets with boxes
// 0.04° apart, the older from 1,000 ft MSL, the newer from 1,000 ft AGL:
// with the same name the newer alone is kept.
func TestPreferNewerSameName(t *testing.T) {
	s := store(t, StorePreferNewerSet())
	at, err := s.At(49.9675, 14.26)
	if err != nil {
		t.Fatal(err)
	}
	var e []*Airspace
	for _, a := range at {
		if a.Name == "PRAGUE CTA 2" && a.Type == ClassE {
			e = append(e, a)
		}
	}
	if len(e) != 1 || e[0].MinAltType != AltAGL {
		t.Fatalf("PRAGUE CTA 2 (E) %d times, want once from 1,000 ft AGL", len(e))
	}
}

// TestShapeCached: Store.Shape builds a record's outline once.
func TestShapeCached(t *testing.T) {
	s := store(t)
	at, err := s.At(lkpr.Lat, lkpr.Lon)
	if err != nil || len(at) == 0 {
		t.Fatal(err, len(at))
	}
	a, _ := s.Shape(at[0])
	b, _ := s.Shape(at[0])
	if a == nil || a != b {
		t.Error("shape built twice")
	}
}
