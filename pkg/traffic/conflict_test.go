//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// air is an airborne aircraft east/north nm of a reference point.
func air(id uint32, cs string, eastNM, northNM, altFt, hdg, kts, fpm float64, ours bool) TrackedAircraft {
	lat, lon := calc.DisplaceByHeading(50, 14, 90, eastNM*1852)
	lat, lon = calc.DisplaceByHeading(lat, lon, 0, northNM*1852)
	return TrackedAircraft{Observation: Observation{ObjectID: id, Tail: cs, Position: airport.LatLon{Lat: lat, Lon: lon},
		AltFt: altFt, AGLFt: altFt - 1000, GroundKts: kts, Heading: hdg, VSFpm: fpm}, Ours: ours}
}

func ours(a TrackedAircraft, _ ResolutionKind) bool { return a.Ours }

func TestPredictAndResolveConflicts(t *testing.T) {
	cases := []struct {
		name string
		a, b TrackedAircraft
	}{
		// Head-on at FL200, 40 NM apart at 450 kt each: 2m40s to meet.
		{"head-on", air(1, "CSA1", 0, 0, 20000, 90, 450, 0, true), air(2, "DLH2", 40, 0, 20000, 270, 450, 0, true)},
		// Crossing at right angles, both reaching the crossing in 2 min.
		{"crossing", air(1, "CSA1", -14, 0, 15000, 90, 420, 0, true), air(2, "DLH2", 0, -14, 15000, 0, 420, 0, true)},
		// Same direction: 480 kt catching 380 kt from 8 NM behind.
		{"same direction", air(1, "CSA1", -8, 0, 30000, 90, 480, 0, true), air(2, "DLH2", 0, 0, 30000, 90, 380, 0, true)},
	}
	for _, c := range cases {
		all := []TrackedAircraft{c.a, c.b}
		cs := PredictConflicts(all, ConflictOptions{})
		if len(cs) != 1 || cs[0].ClosestNM >= EnrouteSeparationNM {
			t.Errorf("%s: %+v, want one conflict", c.name, cs)
			continue
		}
		r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{})
		if !ok {
			t.Errorf("%s: not resolved", c.name)
			continue
		}
		// Applied, they stay apart.
		moved := all
		for i := range moved {
			if moved[i].ObjectID != r.ObjectID {
				continue
			}
			switch r.Kind {
			case ResolveSpeed:
				moved[i].GroundKts = r.Kts
			case ResolveHeading:
				moved[i].Heading = r.HeadingDeg
			case ResolveLevel:
				moved[i].VSFpm = 1500
				if r.AltFt < moved[i].AltFt {
					moved[i].VSFpm = -1500
				}
			}
		}
		if r.Kind == ResolveLevel {
			// Climbing through: check against the resolution's own model.
			t.Logf("%s: %s %s to %.0f ft — %s", c.name, r.Callsign, r.Kind, r.AltFt, r.Why)
			continue
		}
		if left := PredictConflicts(moved, ConflictOptions{}); len(left) != 0 {
			t.Errorf("%s: %+v still conflicts: %+v", c.name, r, left)
		}
		t.Logf("%s: %s %s %+v — %s", c.name, r.Callsign, r.Kind, r, r.Why)
	}
}

// Diverging, far, or vertically separated: no conflict.
func TestPredictConflictsNone(t *testing.T) {
	for name, all := range map[string][]TrackedAircraft{
		"diverging":     {air(1, "CSA1", 0, 0, 20000, 270, 450, 0, true), air(2, "DLH2", 10, 0, 20000, 90, 450, 0, true)},
		"2000 ft apart": {air(1, "CSA1", 0, 0, 20000, 90, 450, 0, true), air(2, "DLH2", 40, 0, 22000, 270, 450, 0, true)},
		"beyond reach":  {air(1, "CSA1", 0, 0, 20000, 90, 300, 0, true), air(2, "DLH2", 200, 0, 20000, 270, 300, 0, true)},
		"parallel 8 NM": {air(1, "CSA1", 0, 0, 20000, 90, 450, 0, true), air(2, "DLH2", 0, 8, 20000, 90, 450, 0, true)},
		"one on the ground": {air(1, "CSA1", 0, 0, 1000, 90, 150, 0, true), func() TrackedAircraft {
			a := air(2, "DLH2", 1, 0, 1000, 270, 20, 0, true)
			a.OnGround = true
			return a
		}()},
	} {
		if cs := PredictConflicts(all, ConflictOptions{}); len(cs) != 0 {
			t.Errorf("%s: %+v", name, cs)
		}
	}
}

// A climb through another's level ahead: lost within the look-ahead, not now.
func TestPredictConflictsClimb(t *testing.T) {
	all := []TrackedAircraft{air(1, "CSA1", 0, 0, 8000, 90, 300, 2000, true), air(2, "DLH2", 20, 0, 12000, 270, 300, 0, true)}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 || cs[0].In <= 0 || cs[0].In > 3*time.Minute {
		t.Fatalf("climb into traffic: %+v", cs)
	}
}

// Other traffic is avoided, never steered; with no aircraft of ours, no
// resolution.
func TestResolveConflictSteersOnlyOurs(t *testing.T) {
	a := air(1, "CSA1", 0, 0, 20000, 90, 450, 0, true)
	b := air(2, "N123", 40, 0, 20000, 270, 450, 0, false)
	all := []TrackedAircraft{a, b}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("%+v", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{})
	if !ok || r.Callsign != "CSA1" {
		t.Errorf("resolution %+v %v, want CSA1 steered", r, ok)
	}
	all[0].Ours = false
	if r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{}); ok {
		t.Errorf("neither ours: %+v", r)
	}
}

// A resolution does not create a conflict with a third aircraft: with the
// level above taken, another way is found.
func TestResolveConflictAvoidsThird(t *testing.T) {
	a := air(1, "CSA1", 0, 0, 20000, 90, 450, 0, true)
	b := air(2, "DLH2", 40, 0, 20000, 270, 450, 0, false)
	third := air(3, "AFR3", 20, 0, 21000, 270, 450, 0, false)
	all := []TrackedAircraft{a, b, third}
	cs := PredictConflicts(all, ConflictOptions{})
	for _, c := range cs {
		if c.A == "CSA1" && c.B == "DLH2" {
			r, ok := ResolveConflict(c, all, ours, ConflictOptions{})
			if !ok || r.Kind == ResolveLevel && r.AltFt == 21000 {
				t.Errorf("resolution %+v %v", r, ok)
			}
			return
		}
	}
	t.Fatalf("no CSA1/DLH2 conflict: %+v", cs)
}

// The rest of a route with a resolution: the change up to the look-ahead,
// then the plan; points behind left out.
func TestResolvedRoute(t *testing.T) {
	a := air(1, "CSA1", 0, 0, 36000, 90, 480, 0, true)
	pt := func(east float64, alt float64) RoutePoint {
		p := air(0, "", east, 0, 0, 0, 0, 0, false).Position
		return RoutePoint{Position: p, AltFt: alt, Kts: 480}
	}
	route := []RoutePoint{pt(-50, 36000), pt(20, 36000), pt(100, 36000), pt(200, 36000)}
	// 5 min at 480 kt: 40 NM.
	lvl := ResolvedRoute(route, a, Resolution{Kind: ResolveLevel, AltFt: 35000}, 5*time.Minute)
	if len(lvl) != 5 || lvl[0].AltFt != 35000 || lvl[1].AltFt != 35000 || lvl[2].AltFt != 35000 || lvl[3].AltFt != 36000 {
		t.Errorf("level: %+v", lvl)
	}
	if d := calc.HaversineNM(a.Position.Lat, a.Position.Lon, lvl[2].Position.Lat, lvl[2].Position.Lon); d < 39 || d > 41 {
		t.Errorf("level ends %.1f NM on, want 40", d)
	}
	spd := ResolvedRoute(route, a, Resolution{Kind: ResolveSpeed, Kts: 430}, 5*time.Minute)
	if spd[1].Kts != 430 || spd[len(spd)-1].Kts != 480 {
		t.Errorf("speed: %+v", spd)
	}
	hdg := ResolvedRoute(route, a, Resolution{Kind: ResolveHeading, HeadingDeg: 110}, 5*time.Minute)
	// Here, 20 NM out on 110°, then back at the first point beyond 40 NM.
	if len(hdg) != 4 || hdg[2] != route[2] {
		t.Errorf("heading: %+v", hdg)
	}
	if b := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, hdg[1].Position.Lat, hdg[1].Position.Lon); b < 109 || b > 111 {
		t.Errorf("heading out %.0f°, want 110", b)
	}
}

// One flight twice (the enroute object and the arrival it is handed over
// to): not a conflict.
func TestPredictConflictsHandover(t *testing.T) {
	all := []TrackedAircraft{air(1, "AFR554", 0, 0, 9000, 90, 250, 0, true), air(2, "AFR554", 0.2, 0, 9000, 90, 250, 0, true)}
	if cs := PredictConflicts(all, ConflictOptions{}); len(cs) != 0 {
		t.Errorf("%+v", cs)
	}
}

// A departure just airborne ahead of an arrival on final at the same
// airport is the tower's (runway separation), for both the watch and the
// conflicts; the same two higher up are not.
func TestTowerPair(t *testing.T) {
	dep := air(1, "AFR1059", 4, 0, 2300, 65, 160, 2000, true)
	arr := air(2, "AFR554", 0, 0, 2800, 65, 140, -700, true)
	dep.Airport, arr.Airport = "LKPR", "LKPR"
	dep.AGLFt, arr.AGLFt = 1100, 1600
	if !TowerPair(dep, arr) {
		t.Fatal("not the tower's")
	}
	if ps := AirborneSeparation([]TrackedAircraft{dep, arr}, EnrouteSeparationNM, VerticalSeparationFt); len(ps) != 1 || ps[0].Loss || !ps[0].Tower {
		t.Errorf("watch: %+v", ps)
	}
	if cs := PredictConflicts([]TrackedAircraft{dep, arr}, ConflictOptions{}); len(cs) != 0 {
		t.Errorf("conflicts: %+v", cs)
	}
	dep.AGLFt, arr.AGLFt = 6000, 6000
	if TowerPair(dep, arr) {
		t.Error("up high: still the tower's")
	}
}
