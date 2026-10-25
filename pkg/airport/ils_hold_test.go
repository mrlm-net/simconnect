package airport

import "testing"

// TestILSHoldShortEntries: at LKPR the entries with an ILS hold-short
// further out on the same taxiway have it (06 at F and E, 24 at Z), and
// in low visibility procedures the route to the entry ends there, else at
// the runway hold-short.
func TestILSHoldShortEntries(t *testing.T) {
	l := loadLKPR(t)
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	stand, perr := l.ParkingIndex("C22")
	if perr != nil {
		t.Fatal("no stand C22")
	}
	for _, c := range []struct {
		end, entry string
	}{{"06", "F"}, {"06", "E"}, {"24", "Z"}} {
		es, err := g.RunwayEntries(c.end)
		if err != nil {
			t.Fatal(err)
		}
		var e *RunwayEntry
		for i := range es {
			if es[i].Taxiway == c.entry {
				e = &es[i]
			}
		}
		if e == nil || e.ILSHoldShort < 0 || !g.Nodes[e.ILSHoldShort].HoldShort.ILS || e.HoldShort == e.ILSHoldShort {
			t.Errorf("%s at %s: entry %+v, want an ILS hold-short behind its hold-short", c.end, c.entry, e)
			continue
		}
		for _, lvp := range []bool{false, true} {
			r, err := g.RouteToRunwayEntry(stand, c.end, c.entry, RouteOptions{LowVisibility: lvp})
			if err != nil {
				t.Fatal(err)
			}
			want := e.HoldShort
			if lvp {
				want = e.ILSHoldShort
			}
			if last := r.Nodes[len(r.Nodes)-1]; last != want {
				t.Errorf("%s at %s, low visibility %v: ends at %d, want %d", c.end, c.entry, lvp, last, want)
			}
		}
	}
	// No ILS hold-short on it (24 at B): its own hold-short either way.
	r, err := g.RouteToRunwayEntry(stand, "24", "B", RouteOptions{LowVisibility: true})
	if err != nil {
		t.Fatal(err)
	}
	if hs := g.Nodes[r.Nodes[len(r.Nodes)-1]].HoldShort; hs == nil || hs.ILS {
		t.Errorf("24 at B in low visibility: ends at %+v, want its runway hold-short", hs)
	}
}
