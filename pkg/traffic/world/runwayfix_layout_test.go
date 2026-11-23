package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A layout that changes (a displaced threshold filled in, then taken out)
// takes the placed runway fixes along: they are placed again from the
// procedures as read, not left at the old threshold, which is within
// reach of the new one (#1646).
func TestWorldRunwayFixesFollowLayoutChange(t *testing.T) {
	const procs = "../../airport/testdata/LKPR-procedures.json"
	var raw airport.RawAirport
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	w := New(Options{DataDir: t.TempDir()})
	if _, err := w.LoadProceduresFile(procs); err != nil {
		t.Fatal(err)
	}
	for _, displaced := range []float32{0, 30, 0} {
		for i, r := range raw.Runways {
			if r.PrimaryNumber == 24 {
				raw.Runways[i].PrimaryDisplacedM = displaced
			}
			if r.SecondaryNumber == 24 {
				raw.Runways[i].SecondaryDisplacedM = displaced
			}
		}
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "layout.json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := w.LoadFile(path); err != nil {
			t.Fatal(err)
		}
		l, _ := w.st.cache.Layout("LKPR")
		var thr airport.LatLon
		for _, r := range l.Runways {
			for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
				if e.Name == "24" {
					thr = e.LandingThreshold()
				}
			}
		}
		w.st.mu.Lock()
		at := rw24MAP(t, w.st.procedures["LKPR"])
		w.st.mu.Unlock()
		if d := calc.HaversineMeters(at.Lat, at.Lon, thr.Lat, thr.Lon); d > 1 {
			t.Errorf("displaced %.0f m: RW24 %.0f m from the landing threshold", displaced, d)
		}
	}
}
