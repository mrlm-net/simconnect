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

// TestAirwayRoute: the airways along a flight's way are taken from a
// fresh cache at once; else queued once, with a corridor and the request
// cap by length, every asker told when the crawl ends; without seeds,
// nothing to wait for.
func TestAirwayRoute(t *testing.T) {
	dir := t.TempDir()
	rec := &recFeed{}
	now := time.Now()
	k := newAirwayKeeper(nil, rec, dir, 24*time.Hour, 1, 2, t.Logf)
	corr := nav.Corridor{From: airport.LatLon{Lat: 50.1, Lon: 14.26}, To: airport.LatLon{Lat: 38.78, Lon: -9.13}, HalfWidthNM: routeCorridorHalfNM}
	seed := nav.Key("VLM", "LK", nav.KindVOR)

	none := airwayRoute{key: "LKPR-LPPT", corridor: corr, done: make(chan struct{})}
	k.route(none, now)
	select {
	case <-none.done:
	default:
		t.Error("no seeds: still waited for")
	}

	a := airwayRoute{key: "LKPR-LPPT", seeds: []nav.FixKey{seed}, corridor: corr, done: make(chan struct{})}
	b := airwayRoute{key: "lkpr-lppt", seeds: []nav.FixKey{seed}, corridor: corr, done: make(chan struct{})}
	k.route(a, now)
	k.route(b, now)
	if len(k.queue) != 1 || len(k.queue[0].done) != 2 || k.queue[0].opts == nil || k.queue[0].opts.Corridor == nil {
		t.Fatalf("queue %+v, want one corridor crawl both wait for", k.queue)
	}
	if n := k.queue[0].opts.MaxRequests; n <= nav.DefaultCrawlMaxRequests || n > routeCrawlMaxRequest {
		t.Errorf("request cap %d for ~1200 NM", n)
	}
	// Its end (nothing found here) tells both.
	job := k.queue[0]
	k.queue = nil
	job.crawler = nav.NewAirwayCrawler(nav.NewNavLoaderWithIDs(nil, 1, 2, 1), *job.opts)
	k.cur = &job
	k.finish(now)
	for _, d := range []chan struct{}{a.done, b.done} {
		select {
		case <-d:
		default:
			t.Error("an asker not told the crawl ended")
		}
	}

	// A fresh cache: fed at once.
	g, err := nav.LoadAirwayGraph("../../nav/testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "airways"), 0o755)
	if err := g.SaveJSON(k.path("route-LKPR-LPPT")); err != nil {
		t.Fatal(err)
	}
	c := airwayRoute{key: "LKPR-LPPT", seeds: []nav.FixKey{seed}, corridor: corr, done: make(chan struct{})}
	k.route(c, now)
	select {
	case <-c.done:
	default:
		t.Error("a fresh cache: still waited for")
	}
	if rec.airways != g.SegmentCount() || len(k.queue) != 0 {
		t.Errorf("fresh cache: fed %d segments, %d queued", rec.airways, len(k.queue))
	}
}
