package airport

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// LKPR's procedures as a chart: SIDs from the far end of their runway,
// STARs ending in vectors where they do, approaches per entry, the fixes
// with their roles.
func TestCharts(t *testing.T) {
	l := loadLKPR(t)
	var p Procedures
	b, err := os.ReadFile("testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	c := Charts(l, p)
	if len(c.SIDs) != len(p.Departures) || len(c.STARs) != len(p.Arrivals) || len(c.Approaches) < len(p.Approaches) {
		t.Fatalf("%d SIDs, %d STARs, %d approaches", len(c.SIDs), len(c.STARs), len(c.Approaches))
	}
	for _, s := range c.SIDs {
		for _, path := range s.Paths {
			if len(path.Points) < 2 {
				t.Errorf("%s: a path of %d points", path.Label, len(path.Points))
			}
		}
		marked := false
		for _, path := range s.Paths {
			marked = marked || len(path.Marks) > 0
		}
		if marked && s.To == "" { // OMNI: no fixes at all
			t.Errorf("SID %s has no last fix", s.Name)
		}
	}
	roles := 0
	for _, f := range c.Fixes {
		if slices.Contains(f.Roles, "FAF") || slices.Contains(f.Roles, "IAF") {
			roles++
		}
	}
	if roles == 0 {
		t.Error("no IAF or FAF among the fixes")
	}
	if start, _ := l.DepartureStart("24"); start == (LatLon{}) {
		t.Error("no departure start for 24")
	}
}
