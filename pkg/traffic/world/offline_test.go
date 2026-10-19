package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// What the simulator gives about an airport is dumped per airport and
// served back from there without one.
func TestAirportDumpRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: t.TempDir(), AirportDumpDir: dir})
	var raw airport.RawAirport
	b, _ := os.ReadFile("../../airport/testdata/LKPR.json")
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	b, _ = os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	g, err := nav.LoadAirwayGraph("../../nav/testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	f := localFeed{st: w.st}
	w.st.dump("LKPR", dumpLayout, raw)
	f.Procedures(p)
	f.Airways("LKPR", g)
	w.st.core.ils.Lock()
	w.st.core.ils.byICAO = map[string]map[string]ilsInfo{"LKPR": {"24": {Runway: "24", Ident: "OKL", MHz: 109.5}}}
	w.st.core.ils.Unlock()
	w.st.dumpILS()
	for _, name := range []string{dumpLayout, dumpProcedures, dumpAirways, dumpILS} {
		if !exists(filepath.Join(dir, "LKPR", name)) {
			t.Fatalf("%s not dumped", name)
		}
	}

	off := New(Options{DataDir: t.TempDir()})
	icao, err := off.LoadAirportDir(filepath.Join(dir, "LKPR"))
	if err != nil || icao != "LKPR" {
		t.Fatalf("%q %v", icao, err)
	}
	if l, ok := off.st.cache.Layout("LKPR"); !ok || len(l.Runways) != 2 {
		t.Error("layout not served")
	}
	off.st.mu.Lock()
	gotP, okP := off.st.procedures["LKPR"]
	nFix := 0
	if off.st.airways != nil {
		nFix = len(off.st.airways.Fixes)
	}
	off.st.mu.Unlock()
	if !okP || len(gotP.Arrivals) != len(p.Arrivals) {
		t.Error("procedures not served")
	}
	if nFix == 0 {
		t.Error("airways not served")
	}
	if ils := off.st.core.ilsOf("LKPR"); len(ils) != 1 || ils[0].Ident != "OKL" {
		t.Errorf("ILS %+v", ils)
	}
	if _, err := off.LoadAirportDir(t.TempDir()); err == nil {
		t.Error("an empty directory loaded")
	}
}

// The worldwide airport list is dumped once, sorted, and served back.
func TestAirportListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: t.TempDir(), AirportDumpDir: dir})
	w.st.dumpAirports([]traffic.AirportRef{
		{ICAO: "lkpr", Region: "LK", Position: airport.LatLon{Lat: 50.1, Lon: 14.26}, DistanceNM: 3},
		{ICAO: "EGLL", Region: "EG", Position: airport.LatLon{Lat: 51.47, Lon: -0.45}},
	})
	off := New(Options{DataDir: t.TempDir()})
	n, err := off.LoadAirportsFile(filepath.Join(dir, dumpAirportList))
	if err != nil || n != 2 {
		t.Fatalf("%d %v, want 2 airports", n, err)
	}
	off.st.mu.Lock()
	a, ok := off.st.airportRefs["LKPR"]
	off.st.mu.Unlock()
	if !ok || a.Region != "LK" || a.DistanceNM != 0 {
		t.Errorf("LKPR %+v %v", a, ok)
	}
}
