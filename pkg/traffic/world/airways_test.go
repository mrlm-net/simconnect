package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// An airport's airways are read from the sim, seeded from its SID and STAR
// fixes (#799); a fresh cache is used as it is, a stale one used and read
// again, and an airport read longer than maxAge ago is read again while
// the World runs.
func TestAirwayKeeper(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	job, ok := airwayJobOf(p)
	if !ok || len(job.seeds) < 10 {
		t.Fatalf("seeds %d, want the SID and STAR fixes", len(job.seeds))
	}
	if d := nav.Key("VLM", "LK", nav.KindVOR); !containsKey(job.seeds, d) {
		t.Errorf("VLM (the VLM STARs' VOR) not a seed: %v", job.seeds)
	}
	if job.center.Lat < 49.5 || job.center.Lat > 50.6 || job.center.Lon < 13.5 || job.center.Lon > 15.2 {
		t.Errorf("centre %v, not around LKPR", job.center)
	}

	dir := t.TempDir()
	g, err := nav.LoadAirwayGraph("../../nav/testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "airways", "LKPR.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := g.SaveJSON(path); err != nil {
		t.Fatal(err)
	}
	rec := &recFeed{}
	now := time.Now()
	k := newAirwayKeeper(nil, rec, dir, 24*time.Hour, 1, 2, t.Logf)
	k.want(p, now)
	if rec.airways != g.SegmentCount() || len(k.queue) != 0 {
		t.Fatalf("fresh cache: fed %d segments, %d queued; want %d, none", rec.airways, len(k.queue), g.SegmentCount())
	}
	// Stale: used, and read again.
	old := now.Add(-48 * time.Hour)
	os.Chtimes(path, old, old)
	k = newAirwayKeeper(nil, rec, dir, 24*time.Hour, 1, 2, t.Logf)
	rec.airways = 0
	k.want(p, now)
	if rec.airways == 0 || len(k.queue) != 1 {
		t.Fatalf("stale cache: fed %d segments, %d queued; want it used and read again", rec.airways, len(k.queue))
	}
	// Running on: read again after maxAge.
	k.queue = nil
	k.read["LKPR"] = now.Add(-25 * time.Hour)
	if !k.dueAgain(now) {
		t.Error("not read again after maxAge")
	}
}

func containsKey(keys []nav.FixKey, k nav.FixKey) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}
