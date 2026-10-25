package traffic

import (
	"fmt"
	"strings"
	"testing"
)

// TestStandAreasSchengen: at LKPR an A320 from London takes a Terminal 1
// gate (A or B), one from Paris a Terminal 2 gate (C), each as long as
// one is free; with all of its own taken, the other pier's.
func TestStandAreasSchengen(t *testing.T) {
	g := lkprGraph(t)
	pier := func(s int) string { return g.Layout.Parking[s].Label()[:1] }
	a := NewStandAllocator(&eventClient{}, g)
	req := func(cs, other string) StandRequirements {
		return StandRequirements{Owner: cs, Airline: "AFR", HalfSpan: 17.9, Runway: "24", Other: other}
	}
	if s, err := a.Assign(req("BAW1", "EGLL")); err != nil || !strings.Contains("AB", pier(s)) {
		t.Errorf("from EGLL: stand %v (%v), want a Terminal 1 gate", g.Layout.Parking[max(s, 0)].Label(), err)
	}
	if s, err := a.Assign(req("AFR1", "LFPG")); err != nil || pier(s) != "C" {
		t.Errorf("from LFPG: stand %v (%v), want a Terminal 2 gate", g.Layout.Parking[max(s, 0)].Label(), err)
	}
	if !Schengen("LKPR") || !Schengen("GCLP") || Schengen("EGLL") || Schengen("LCLK") || Schengen("EKVG") {
		t.Error("Schengen prefixes")
	}
	// Every C gate taken: a Paris flight then goes to another stand, a
	// Terminal 1 gate only once nothing outside an area is free.
	b := NewStandAllocator(&eventClient{}, g)
	for i, p := range g.Layout.Parking {
		if p.Label()[:1] == "C" {
			b.Occupy(i, fmt.Sprint("X", i), 17.9)
		}
	}
	s, err := b.Assign(req("AFR2", "LFPG"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains("AB", pier(s)) {
		t.Errorf("C full: %s, want a stand outside Terminal 1 first", g.Layout.Parking[s].Label())
	}
}
