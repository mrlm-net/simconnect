package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestRunwayClosuresWorld: a runway closed through the World reaches the
// runway limits, the fallback runway without weather skips it, all closed
// is known; opened again, it is back.
func TestRunwayClosuresWorld(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	l := lkprLayout(t)
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	w.CloseRunway("lkpr", "06/24", true)
	if got := w.ClosedRunways("LKPR"); !slices.Equal(got, []string{"06/24"}) {
		t.Fatalf("closed %v", got)
	}
	if lim := w.st.core.runwayLimits(g, airport.Limits{}); !lim.ClosedEnd("24") || lim.ClosedEnd("30") {
		t.Errorf("limits closed %v", lim.Closed)
	}
	cc := &controlCenter{core: w.st.core}
	for _, e := range cc.runwaysInUse(g, false) {
		if e.Name == "06" || e.Name == "24" {
			t.Errorf("fallback runway %s is closed", e.Name)
		}
	}
	if w.st.core.allClosed(l) {
		t.Error("all closed with 12/30 open")
	}
	for _, r := range l.Runways {
		w.CloseRunway("LKPR", r.Name(), true)
	}
	if !w.st.core.allClosed(l) {
		t.Error("not all closed")
	}
	for _, r := range l.Runways {
		w.CloseRunway("LKPR", r.Name(), false)
	}
	if len(w.ClosedRunways("LKPR")) != 0 {
		t.Errorf("still closed: %v", w.ClosedRunways("LKPR"))
	}
}

// TestClosedClearances: on a closed runway nothing lines up, takes off or
// lands; ours on a 3 NM final go around, one at 8 NM is left to approach;
// a crossing stays allowed.
func TestClosedClearances(t *testing.T) {
	c := traffic.RunwayClearances{LineUp: []string{"CSA1"}, Takeoff: []string{"CSA1"}, Land: []string{"DLH2"}, Cross: []string{"OK-TUG"}}
	list := []traffic.RunwayUser{
		{Callsign: "CSA1", Phase: traffic.RunwayHoldingShort},
		{Callsign: "DLH2", Arrival: true, Phase: traffic.RunwayFinal, DistanceNM: 3},
		{Callsign: "AFR3", Arrival: true, Phase: traffic.RunwayFinal, DistanceNM: 8},
	}
	closedClearances(&c, list)
	if len(c.LineUp)+len(c.Takeoff)+len(c.Land) != 0 || !slices.Equal(c.GoAround, []string{"DLH2"}) || !slices.Equal(c.Cross, []string{"OK-TUG"}) {
		t.Errorf("closed: %+v", c)
	}
	if c.Waiting["CSA1"] != "runway closed" {
		t.Errorf("waiting %q", c.Waiting["CSA1"])
	}
}

// TestAlternate: the nearest airport 30 NM or more away with a four-letter
// code.
func TestAlternate(t *testing.T) {
	st := &state{airportRefs: map[string]traffic.AirportRef{
		"LKPR": {ICAO: "LKPR", Position: airport.LatLon{Lat: 50.10, Lon: 14.26}},
		"LKVO": {ICAO: "LKVO", Position: airport.LatLon{Lat: 50.22, Lon: 14.40}}, // 9 NM: too near
		"LKKV": {ICAO: "LKKV", Position: airport.LatLon{Lat: 50.20, Lon: 12.92}}, // 52 NM
		"LKPD": {ICAO: "LKPD", Position: airport.LatLon{Lat: 50.01, Lon: 15.74}}, // 57 NM
		"CZ12": {ICAO: "CZ12", Position: airport.LatLon{Lat: 50.10, Lon: 13.70}}, // digits: a strip
	}}
	q := &sequences{s: &scheduler{st: st}}
	if got := q.alternate("LKPR", airport.LatLon{Lat: 50.10, Lon: 14.26}); got != "LKKV" {
		t.Errorf("alternate %q, want LKKV", got)
	}
}
