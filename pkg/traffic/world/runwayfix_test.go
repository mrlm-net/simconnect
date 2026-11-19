package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// rw24MAP is the ILS 24 final's runway fix in p.
func rw24MAP(t *testing.T, p airport.Procedures) airport.LatLon {
	t.Helper()
	for _, a := range p.Approaches {
		if a.Name != "ILS 24" {
			continue
		}
		for _, l := range a.Final {
			if l.FixKind == "R" {
				return l.Position
			}
		}
	}
	t.Fatal("no ILS 24 runway fix")
	return airport.LatLon{}
}

// The World keeps procedures with their runway fixes on the runway ends
// (#1573), whichever of procedures and layout comes first, and dumps them
// so.
func TestWorldPlacesRunwayFixes(t *testing.T) {
	const layout, procs = "../../airport/testdata/LKPR.json", "../../airport/testdata/LKPR-procedures.json"
	var raw airport.RawAirport
	b, _ := os.ReadFile(layout)
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	var thr airport.LatLon
	for _, r := range l.Runways {
		for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if e.Name == "24" {
				thr = e.LandingThreshold()
			}
		}
	}
	var p airport.Procedures
	b, _ = os.ReadFile(procs)
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if at := rw24MAP(t, p); calc.HaversineMeters(at.Lat, at.Lon, thr.Lat, thr.Lon) < 1000 {
		t.Fatalf("testdata RW24 %.0f m from the threshold, the test expects the centre", calc.HaversineMeters(at.Lat, at.Lon, thr.Lat, thr.Lon))
	}
	stored := func(w *World) airport.Procedures {
		w.st.mu.Lock()
		defer w.st.mu.Unlock()
		return w.st.procedures["LKPR"]
	}
	near := func(what string, at airport.LatLon) {
		t.Helper()
		if d := calc.HaversineMeters(at.Lat, at.Lon, thr.Lat, thr.Lon); d > 1 {
			t.Errorf("%s: RW24 %.0f m from the threshold", what, d)
		}
	}

	// Offline, procedures first, then the layout; and the other way round.
	w := New(Options{DataDir: t.TempDir()})
	if _, err := w.LoadProceduresFile(procs); err != nil {
		t.Fatal(err)
	}
	if _, err := w.LoadFile(layout); err != nil {
		t.Fatal(err)
	}
	near("procedures, then layout", rw24MAP(t, stored(w)))
	w = New(Options{DataDir: t.TempDir()})
	if _, err := w.LoadFile(layout); err != nil {
		t.Fatal(err)
	}
	if _, err := w.LoadProceduresFile(procs); err != nil {
		t.Fatal(err)
	}
	near("layout, then procedures", rw24MAP(t, stored(w)))

	// From the simulator, procedures before the layout: kept, then dumped
	// again with the layout.
	dump := t.TempDir()
	w = New(Options{DataDir: t.TempDir(), AirportDumpDir: dump})
	f := localFeed{st: w.st}
	f.Procedures(p)
	w.st.cache.Put(l)
	f.Layout("LKPR", l, nil)
	near("fed", rw24MAP(t, stored(w)))
	var dumped airport.Procedures
	b, _ = os.ReadFile(filepath.Join(dump, "LKPR", dumpProcedures))
	if err := json.Unmarshal(b, &dumped); err != nil {
		t.Fatal(err)
	}
	near("dumped", rw24MAP(t, dumped))
}
