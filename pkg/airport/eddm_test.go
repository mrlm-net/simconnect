package airport

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestEDDMEntryA4: at EDDM an unnamed connector joins taxiway A4 to runway
// 08L/26R; the entry must still be offered as "08L at A4" and routable.
func TestEDDMEntryA4(t *testing.T) {
	b, err := os.ReadFile("testdata/EDDM-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := g.RunwayEntries("08L")
	var names []string
	for _, e := range entries {
		names = append(names, e.Taxiway)
	}
	t.Logf("08L entries: %v", names)
	found := false
	for _, n := range names {
		found = found || n == "A4"
	}
	if !found {
		t.Fatalf("08L entries %v, want A4", names)
	}
	for _, p := range l.Parking {
		if p.Radius < 20 {
			continue
		}
		r, err := g.RouteToRunwayEntry(p.Index, "08L", "A4", RouteOptions{})
		if err != nil {
			continue
		}
		t.Logf("%s → 08L at A4: %.0f m via %v", p.Label(), r.Length, r.Taxiways)
		return
	}
	t.Fatal("no stand routes to 08L at A4")
}

// TestEndEntrances: the entrances side by side at a runway's end, each
// with its holding point (LKPR 12: L and D have none), one per
// taxiway, intersections further in left out (LKPR 24: A and Z, not B or
// E; EDDM 08L: A1, A2, A3).
func TestEndEntrances(t *testing.T) {
	b, err := os.ReadFile("testdata/EDDM-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	eddm, err := BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		g    *Graph
		end  string
		want []string
	}{{lkprGraph(t), "24", []string{"A", "Z"}}, {lkprGraph(t), "12", nil}, {lkprGraph(t), "06", []string{"F"}},
		{eddm, "08L", []string{"A1", "A2", "A3"}}, {eddm, "26L", []string{"B14", "B15", "B13"}}} {
		es, err := c.g.EndEntrances(c.end)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, e := range es {
			got = append(got, e.Taxiway)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s %s: %v, want %v", c.g.Layout.ICAO, c.end, got, c.want)
		}
	}
}
