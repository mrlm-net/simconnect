//go:build windows
// +build windows

package nav

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

func loadLKPRAirways(t *testing.T) *AirwayGraph {
	t.Helper()
	g, err := LoadAirwayGraph("testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestLKPRGraphSize(t *testing.T) {
	g := loadLKPRAirways(t)
	kinds := map[FixKind]int{}
	types := map[AirwayType]int{}
	for _, f := range g.Fixes {
		kinds[f.Kind]++
	}
	for _, a := range g.Airways {
		types[a.Type]++
	}
	t.Logf("%d fixes %v, %d airways %v, %d segments", len(g.Fixes), kinds, len(g.Airways), types, g.SegmentCount())
	if len(g.Fixes) < 500 || len(g.Fixes) > 5000 {
		t.Errorf("fixes = %d, want a few thousand at most", len(g.Fixes))
	}
	if g.SegmentCount() < len(g.Fixes)/2 {
		t.Errorf("segments = %d for %d fixes", g.SegmentCount(), len(g.Fixes))
	}
	if kinds[KindVOR] == 0 || kinds[KindWaypoint] == 0 {
		t.Errorf("kinds = %v, want VORs and waypoints", kinds)
	}
	voz, ok := g.Fix(Key("VOZ", "LK", KindVOR))
	if !ok || voz.Name != "VOZICE" || voz.Freq != 116.95 {
		t.Errorf("VOZ = %+v", voz)
	}
	// Every segment joins known fixes, and its distance is sane.
	for _, a := range g.Airways {
		for _, s := range a.Segments {
			_, okF := g.Fix(s.From)
			_, okT := g.Fix(s.To)
			if !okF || !okT {
				t.Fatalf("%s: segment %s-%s has an unknown end", a.Name, s.From, s.To)
			}
			if s.DistanceNM <= 0 || s.DistanceNM > 400 {
				t.Errorf("%s: %s-%s is %.1f NM", a.Name, s.From, s.To, s.DistanceNM)
			}
		}
	}
}

func TestLKPRSegmentsFromData(t *testing.T) {
	g := loadLKPRAirways(t)
	// Checked live: TABEM's M725 runs VOZ -> TABEM -> OKF.
	var m725 *Airway
	for i := range g.Airways {
		if g.Airways[i].Name == "M725" {
			m725 = &g.Airways[i]
		}
	}
	if m725 == nil {
		t.Fatal("no M725")
	}
	want := map[string]bool{"VOZ-TABEM": false, "TABEM-OKF": false}
	for _, s := range m725.Segments {
		if k := s.From.Ident + "-" + s.To.Ident; want[k] == false {
			if _, ok := want[k]; ok {
				want[k] = true
			}
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("M725 lacks %s", k)
		}
	}
}

func TestLKPRRoute(t *testing.T) {
	g := loadLKPRAirways(t)
	cases := []struct{ from, to FixKey }{
		{Key("VOZ", "LK", KindVOR), Key("GOLOP", "LK", KindWaypoint)},
		{Key("VOZ", "LK", KindVOR), Key("LOMKI", "LK", KindWaypoint)},
		{Key("VLM", "LK", KindVOR), Key("VENOX", "LK", KindWaypoint)},
	}
	for _, c := range cases {
		steps, err := g.Route(c.from, c.to)
		if err != nil {
			t.Errorf("%s -> %s: %v", c.from, c.to, err)
			continue
		}
		checkFollowsAirways(t, g, steps)
		a, _ := g.Fix(c.from)
		b, _ := g.Fix(c.to)
		direct := dist(a.Position, b.Position)
		total := RouteDistanceNM(steps)
		t.Logf("%s -> %s: %s (%.0f NM, direct %.0f)", c.from.Ident, c.to.Ident, FormatRoute(steps), total, direct)
		if total < direct-0.1 {
			t.Errorf("%s -> %s: %.0f NM, shorter than direct %.0f", c.from, c.to, total, direct)
		}
	}

	// Along one airway the route is that airway: VOZ M725 TABEM M725 OKF.
	steps, err := g.Route(Key("VOZ", "LK", KindVOR), Key("OKF", "LK", KindVOR))
	if err != nil || FormatRoute(steps) != "VOZ M725 OKF" {
		t.Errorf("VOZ -> OKF = %q, %v", FormatRoute(steps), err)
	}

	// Prague's airways are sparse (the TMA breaks them): VOZ to GOLOP by
	// airway is a long detour, so a stretch limit falls back to direct.
	steps, err = g.RouteOrDirect(Key("VOZ", "LK", KindVOR), Key("GOLOP", "LK", KindWaypoint), 2)
	if err != nil || len(steps) != 2 || steps[1].Airway != Direct {
		t.Errorf("VOZ -> GOLOP within 2x = %q, %v", FormatRoute(steps), err)
	}
}

// TestLKPRRouteAbroad routes from VOZ to fixes in Germany and Austria.
func TestLKPRRouteAbroad(t *testing.T) {
	g := loadLKPRAirways(t)
	voz := Key("VOZ", "LK", KindVOR)
	for _, region := range []string{"ED", "LO"} {
		// The loaded fix of the region farthest from VOZ.
		var to Fix
		best := 0.0
		from, _ := g.Fix(voz)
		for _, f := range g.Fixes {
			if f.Region == region && f.Type != 0 && len(g.Edges(f.Key())) > 0 {
				if d := dist(from.Position, f.Position); d > best {
					best, to = d, f
				}
			}
		}
		if to.Ident == "" {
			t.Fatalf("no %s fix", region)
		}
		steps, err := g.Route(voz, to.Key())
		if err != nil {
			t.Fatalf("VOZ -> %s: %v", to.Key(), err)
		}
		checkFollowsAirways(t, g, steps)
		t.Logf("VOZ -> %s (%s): %s, %.0f NM", to.Ident, region, FormatRoute(steps), RouteDistanceNM(steps))
	}
}

func checkFollowsAirways(t *testing.T, g *AirwayGraph, steps []RouteStep) {
	t.Helper()
	if len(steps) < 2 || steps[0].Airway != "" {
		t.Fatalf("bad route %+v", steps)
	}
	for i := 1; i < len(steps); i++ {
		found := false
		for _, e := range g.Edges(steps[i-1].Fix) {
			if e.To == steps[i].Fix && e.Airway == steps[i].Airway {
				found = true
			}
		}
		if !found {
			t.Errorf("step %s %s -> %s is not an airway segment", steps[i].Airway, steps[i-1].Fix, steps[i].Fix)
		}
	}
}

func TestRouteSynthetic(t *testing.T) {
	p := func(lat, lon float64) airport.LatLon { return airport.LatLon{Lat: lat, Lon: lon} }
	w := func(id string, lat, lon float64) Fix {
		return Fix{Ident: id, Region: "XX", Kind: KindWaypoint, Position: p(lat, lon)}
	}
	ref := func(f Fix) *FixRef { return &FixRef{Key: f.Key(), Position: f.Position} }
	a, b, c, d := w("AAAAA", 50, 14), w("BBBBB", 50, 15), w("CCCCC", 50, 16), w("DDDDD", 50.05, 15)
	lone := w("LONE", 45, 10)
	links := map[FixKey][]RouteLink{
		// UA1: A-B-C, UB2: A-D-C (slightly longer), plus a one-fix hop.
		a.Key(): {{Airway: "UA1", Type: AirwayJet, Next: ref(b)}, {Airway: "UB2", Type: AirwayJet, Next: ref(d)}},
		b.Key(): {{Airway: "UA1", Type: AirwayJet, Prev: ref(a), Next: ref(c)}},
		c.Key(): {{Airway: "UA1", Type: AirwayJet, Prev: ref(b)}, {Airway: "UB2", Type: AirwayJet, Prev: ref(d)}},
		d.Key(): {{Airway: "UB2", Type: AirwayJet, Prev: ref(a), Next: ref(c)}},
	}
	g := BuildAirwayGraph([]Fix{a, b, c, d, lone}, links)
	if len(g.Airways) != 2 || g.SegmentCount() != 4 {
		t.Fatalf("airways %d segments %d, want 2 and 4 (duplicates merged)", len(g.Airways), g.SegmentCount())
	}
	steps, err := g.Route(c.Key(), a.Key()) // against the data order
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatRoute(steps); got != "CCCCC UA1 AAAAA" {
		t.Errorf("route = %q", got)
	}
	if _, err := g.Route(a.Key(), lone.Key()); !errors.Is(err, ErrNoRoute) {
		t.Errorf("unconnected: err = %v", err)
	}
	steps, err = g.RouteOrDirect(a.Key(), lone.Key(), 0)
	if err != nil || len(steps) != 2 || steps[1].Airway != Direct {
		t.Errorf("direct = %+v, %v", steps, err)
	}
	if _, err := g.Route(a.Key(), Key("NOPE", "XX", KindWaypoint)); err == nil || errors.Is(err, ErrNoRoute) {
		t.Errorf("unknown fix: err = %v", err)
	}
	if f, _, ok := g.Nearest(p(50.01, 15.01)); !ok || f.Ident != "BBBBB" {
		t.Errorf("nearest = %+v", f)
	}

	// Round trip through JSON.
	var buf bytes.Buffer
	if err := g.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"kind": "W"`) {
		t.Errorf("kind not written as a letter")
	}
	g2, err := ReadAirwayGraph(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if s2, err := g2.Route(c.Key(), a.Key()); err != nil || FormatRoute(s2) != "CCCCC UA1 AAAAA" {
		t.Errorf("after reload: %v %v", FormatRoute(s2), err)
	}
}

func TestParseFixKey(t *testing.T) {
	for in, want := range map[string]FixKey{
		"voz.lk.v": {Ident: "VOZ", Region: "LK", Kind: KindVOR},
		"GOLOP:LK": {Ident: "GOLOP", Region: "LK", Kind: KindWaypoint},
		"OKL:LK:V": {Ident: "OKL", Region: "LK", Kind: KindVOR},
		"PR.LK.N":  {Ident: "PR", Region: "LK", Kind: KindNDB},
		"LOMKI":    {Ident: "LOMKI", Kind: KindWaypoint},
	} {
		got, err := ParseFixKey(in)
		if err != nil || got != want {
			t.Errorf("ParseFixKey(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseFixKey("A.B.X"); err == nil {
		t.Error("bad kind accepted")
	}
}
