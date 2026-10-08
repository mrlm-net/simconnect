package traffic

import (
	"strings"
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

// A climbing departure below level traffic ahead is stopped on its way, as
// high as the vertical minimum below the traffic allows ("stop climb at 6000
// feet" under 7000), never slowed or sent back down (#657).
func TestResolveStopsClimb(t *testing.T) {
	dep := air(1, "RYR1", 0, 0, 3000, 90, 200, 2000, true)
	arr := air(2, "DLH2", 20, 0, 7000, 270, 250, 0, false)
	all := []TrackedAircraft{dep, arr}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("conflicts %+v, want one", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{})
	if !ok || r.Kind != ResolveLevel || !r.Stop || r.AltFt != 6000 {
		t.Fatalf("%+v %v, want stop climb at 6000 ft", r, ok)
	}
	if got, want := Resolved(PosDeparture, r, dep.AltFt, dep.Heading, dep.GroundKts).Text, "RYR1, stop climb at 6000 feet, due traffic"; !strings.HasPrefix(got, want) {
		t.Errorf("%q, want %q", got, want)
	}
	// The loss is under the vertical minimum, and the stop keeps it (#657).
	if c := cs[0]; c.LossFt >= VerticalSeparationFt || c.LossNM >= c.MinNM || c.AAltFt == 0 && c.BAltFt == 0 {
		t.Errorf("loss %.1f NM %.0f ft (alts %.0f/%.0f), want under %.1f NM and %.0f ft", c.LossNM, c.LossFt, c.AAltFt, c.BAltFt, c.MinNM, VerticalSeparationFt)
	}
	if r.KeepsFt != VerticalSeparationFt {
		t.Errorf("the stop keeps %.0f ft, want %.0f", r.KeepsFt, VerticalSeparationFt)
	}
	if got, want := ContinueLevel(PosDeparture, "RYR1", 24000, true).Text, "RYR1, climb to flight level 240"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	if rb, _ := Readback(ContinueLevel(PosDeparture, "RYR1", 24000, true)); rb.Text != "Climb to flight level 240, RYR1" {
		t.Errorf("readback %q", rb.Text)
	}
}

// Crossing traffic is parted by altitude, traffic on the same route by
// speed first, then a shortcut (or a leg extended), altitude last.
func TestResolveOrderByGeometry(t *testing.T) {
	// Crossing at right angles at FL150.
	a, b := air(1, "CSA1", -14, 0, 15000, 90, 420, 0, true), air(2, "DLH2", 0, -14, 15000, 0, 420, 0, false)
	all := []TrackedAircraft{a, b}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("crossing: %+v", cs)
	}
	if r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{}); !ok || r.Kind != ResolveLevel {
		t.Errorf("crossing: %+v %v, want a level", r, ok)
	}
	// In trail: 480 kt catching 380 kt from 8 NM behind.
	a, b = air(1, "CSA1", -8, 0, 30000, 90, 480, 0, true), air(2, "DLH2", 0, 0, 30000, 90, 380, 0, false)
	all = []TrackedAircraft{a, b}
	cs = PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("in trail: %+v", cs)
	}
	if r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{}); !ok || r.Kind != ResolveSpeed {
		t.Errorf("in trail: %+v %v, want speed", r, ok)
	}
	// The leader ours, the one catching up from behind not, and no speed
	// change allowed: the leader takes a shortcut along its route (a fix
	// 30 NM ahead, then one 20 NM off to the left): direct to the far fix.
	a, b = air(1, "CSA1", -8, 0, 30000, 90, 440, 0, false), air(2, "DLH2", 0, 0, 30000, 90, 400, 0, true)
	all = []TrackedAircraft{a, b}
	cs = PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("leader: %+v", cs)
	}
	next := air(0, "", 30, 0, 0, 0, 0, 0, false).Position
	far := air(0, "", 60, 20, 0, 0, 0, 0, false).Position
	o := ConflictOptions{DirectFixes: func(TrackedAircraft) []DirectFix {
		return []DirectFix{{Ident: "NEXT", Position: next}, {Ident: "FAR", Position: far}}
	}}
	noSpeed := func(x TrackedAircraft, k ResolutionKind) bool { return x.Ours && k != ResolveSpeed }
	r, ok := ResolveConflict(cs[0], all, noSpeed, o)
	if !ok || r.Kind != ResolveDirect || r.Fix != "FAR" {
		t.Fatalf("leader: %+v %v, want direct FAR", r, ok)
	}
	if got := Resolved(PosCenter, r, 30000, 90, 400).Text; got != "DLH2, cleared direct to FAR" {
		t.Errorf("%q", got)
	}
	route := []RoutePoint{{Position: next, AltFt: 30000, Kts: 400}, {Position: far, AltFt: 28000, Kts: 400}, {Position: air(0, "", 90, 20, 0, 0, 0, 0, false).Position, AltFt: 24000, Kts: 380}}
	if got := ResolvedRoute(route, b, r, 5*time.Minute); len(got) != 3 || got[1].AltFt != 28000 {
		t.Errorf("route %+v, want here, FAR, on", got)
	}
}

// One of ours on its STAR turning away is predicted along its route, not
// straight on into the other (live, CSA786 stopped at 8000 ft for KLM130
// whose STAR turned away).
func TestPredictAlongRoute(t *testing.T) {
	// KLM1 heads east at 8000 ft towards CSA2 coming west at 8500 ft;
	// KLM1's route turns north 4 NM ahead.
	klm := air(1, "KLM1", 0, 0, 8000, 90, 250, 0, true)
	csa := air(2, "CSA2", 20, 0, 8500, 270, 250, 0, false)
	all := []TrackedAircraft{klm, csa}
	if cs := PredictConflicts(all, ConflictOptions{}); len(cs) != 1 {
		t.Fatalf("straight on: %+v, want one conflict", cs)
	}
	turn := air(0, "", 4, 0, 0, 0, 0, 0, false).Position
	north := air(0, "", 4, 30, 0, 0, 0, 0, false).Position
	o := ConflictOptions{Route: func(a TrackedAircraft) []airport.LatLon {
		if a.ObjectID == 1 {
			return RouteAhead(a.Position, []airport.LatLon{turn, north})
		}
		return nil
	}}
	if cs := PredictConflicts(all, o); len(cs) != 0 {
		t.Errorf("along its route: %+v, want none", cs)
	}
}

// A direct is cleared only when the path itself stays clear: towards
// traffic it is not, turning away it is.
func TestPathClear(t *testing.T) {
	me := air(1, "PHGVV", 0, 0, 3000, 90, 200, 500, true)
	other := air(2, "TVS440", 15, 0, 4000, 270, 220, 0, true)
	all := []TrackedAircraft{me, other}
	into := []airport.LatLon{air(0, "", 30, 0, 0, 0, 0, 0, false).Position}
	away := []airport.LatLon{air(0, "", 0, -30, 0, 0, 0, 0, false).Position}
	if PathClear(me, into, all, ConflictOptions{}) {
		t.Error("direct towards TVS440: clear, want not")
	}
	if !PathClear(me, away, all, ConflictOptions{}) {
		t.Error("direct away from TVS440: not clear, want clear")
	}
}

// TestStopClimbBelowTraffic: a departure climbing toward traffic level at
// 10000 ft is stopped at 9000 ft, 1000 ft below it, not at the next thousand
// above its own level (live, THY1463 and TVS524 stopped at 4000 ft, #657).
func TestStopClimbBelowTraffic(t *testing.T) {
	dep := air(1, "THY1463", 0, 0, 3000, 90, 210, 1600, true)
	tra := air(2, "BAW1413", 30, 0, 10000, 270, 250, 0, false)
	all := []TrackedAircraft{dep, tra}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("conflicts %+v, want one", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{})
	if !ok || r.Kind != ResolveLevel || !r.Stop || r.AltFt != 9000 {
		t.Fatalf("%+v %v, want stop climb at 9000 ft", r, ok)
	}
	if r.KeepsFt != VerticalSeparationFt {
		t.Errorf("keeps %.0f ft, want %.0f", r.KeepsFt, VerticalSeparationFt)
	}
}

// pt is a point east/north nm of the reference point at altFt.
func pt(eastNM, northNM, altFt float64) RoutePoint {
	lat, lon := calc.DisplaceByHeading(50, 14, 90, eastNM*1852)
	lat, lon = calc.DisplaceByHeading(lat, lon, 0, northNM*1852)
	return RoutePoint{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: altFt}
}

// TestTrackAlongProfile: along a profile the altitude moves toward the next
// point's at the rate that makes it by the point (or the vertical speed now
// if faster), and stays there past it (#657).
func TestTrackAlongProfile(t *testing.T) {
	a := air(1, "A", 0, 0, 3000, 90, 240, 0, true) // 4 NM a minute
	o := ConflictOptions{Profile: func(TrackedAircraft) []RoutePoint {
		return []RoutePoint{pt(8, 0, 5000), pt(20, 0, 0), pt(40, 0, 5000)}
	}}
	tr := trackFor(a, o)
	for _, c := range []struct {
		d    time.Duration
		want float64
	}{{time.Minute, 4000}, {2 * time.Minute, 5000}, {4 * time.Minute, 5000}, {12 * time.Minute, 5000}} {
		if _, _, alt := tr.at(c.d); alt < c.want-50 || alt > c.want+50 {
			t.Errorf("at %s: %.0f ft, want %.0f", c.d, alt, c.want)
		}
	}
	// Faster than needed: at its vertical speed, level at the altitude early.
	a.VSFpm = 3000
	if _, _, alt := trackFor(a, o).at(time.Minute); alt < 4950 || alt > 5050 {
		t.Errorf("climbing 3000 fpm: %.0f ft after a minute, want 5000", alt)
	}
	// Stopped below it: not past the stop.
	tr.level = 4000
	if _, _, alt := tr.at(3 * time.Minute); alt != 4000 {
		t.Errorf("stopped at 4000: %.0f ft", alt)
	}
}

// TestConflictsAlongProfile: a departure climbing now but levelling off on
// its SID below level traffic is no conflict; an arrival level now but
// descending on its STAR onto a level departure is (live, TVS524 and
// BAW1413 stopped for, #657).
func TestConflictsAlongProfile(t *testing.T) {
	dep := air(1, "TVS524", 0, 0, 3000, 90, 210, 2000, true)
	tra := air(2, "BAW1413", 30, 0, 10000, 270, 250, 0, false)
	all := []TrackedAircraft{dep, tra}
	if cs := PredictConflicts(all, ConflictOptions{}); len(cs) != 1 {
		t.Fatalf("climbing on at 2000 fpm: %+v, want a conflict", cs)
	}
	sid := ConflictOptions{Profile: func(a TrackedAircraft) []RoutePoint {
		if a.ObjectID == 1 {
			return []RoutePoint{pt(10, 0, 6000), pt(40, 0, 6000)}
		}
		return nil
	}}
	if cs := PredictConflicts(all, sid); len(cs) != 0 {
		t.Errorf("levelling at 6000 ft under 10000: %+v, want none", cs)
	}

	lvl := air(1, "TVS524", 0, 0, 6000, 90, 210, 0, true)
	arr := air(2, "BAW1413", 30, 0, 10000, 270, 250, 0, true)
	all = []TrackedAircraft{lvl, arr}
	if cs := PredictConflicts(all, ConflictOptions{}); len(cs) != 0 {
		t.Fatalf("both level 4000 ft apart: %+v, want none", cs)
	}
	star := ConflictOptions{Profile: func(a TrackedAircraft) []RoutePoint {
		if a.ObjectID == 2 {
			return []RoutePoint{pt(20, 0, 6000), pt(-10, 0, 4000)}
		}
		return nil
	}}
	if cs := PredictConflicts(all, star); len(cs) != 1 {
		t.Errorf("descending on the STAR through 6000 ft: %+v, want a conflict", cs)
	}
}

// TestAirborneSeparationTerminal: in the terminal area (both at an airport
// below 10000 ft) the lateral minimum is 3 NM, 5 NM elsewhere.
func TestAirborneSeparationTerminal(t *testing.T) {
	a := air(1, "CSA1", 0, 0, 8000, 90, 250, 0, true)
	b := air(2, "CSA2", 4, 0, 8500, 270, 250, 0, true)
	if ps := AirborneSeparationFor([]TrackedAircraft{a, b}, ConflictOptions{}); len(ps) != 1 || !ps[0].Loss || ps[0].MinNM != EnrouteSeparationNM {
		t.Errorf("en route 4 NM, 500 ft: %+v, want a loss under 5 NM", ps)
	}
	a.Airport, b.Airport = "LKPR", "LKPR"
	if ps := AirborneSeparationFor([]TrackedAircraft{a, b}, ConflictOptions{}); len(ps) != 1 || ps[0].Loss || ps[0].MinNM != TerminalSeparationNM {
		t.Errorf("terminal 4 NM, 500 ft: %+v, want no loss under 3 NM", ps)
	}
}

// TestStopClimbUnderDescendingTraffic: traffic descending toward the
// climber: the highest level that stays clear of it all the look-ahead,
// not the next thousand on the climber's way (#657 review).
func TestStopClimbUnderDescendingTraffic(t *testing.T) {
	dep := air(1, "DEP", 0, 0, 3000, 90, 210, 1500, true)
	oth := air(2, "OTH", 30, 0, 12000, 270, 250, -1000, false)
	all := []TrackedAircraft{dep, oth}
	cs := PredictConflicts(all, ConflictOptions{})
	if len(cs) != 1 {
		t.Fatalf("conflicts %+v, want one", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, ConflictOptions{})
	if !ok || r.Kind != ResolveLevel || !r.Stop || r.AltFt <= 4000 {
		t.Fatalf("%+v %v, want a stop above 4000 ft", r, ok)
	}
	if r.KeepsFt < VerticalSeparationFt {
		t.Errorf("keeps %.0f ft, want at least %.0f", r.KeepsFt, VerticalSeparationFt)
	}
}

// TestProfileEdges: no turning back to an altitude behind the climb, and
// the vertical speed on past a profile without altitudes (#657 review).
func TestProfileEdges(t *testing.T) {
	a := air(1, "A", 0, 0, 6000, 90, 240, 2400, true) // 4 NM and 2400 ft a minute
	o := ConflictOptions{Profile: func(TrackedAircraft) []RoutePoint {
		return []RoutePoint{pt(4, 0, 5000), pt(40, 0, 24000)}
	}}
	if _, _, alt := trackFor(a, o).at(30 * time.Second); alt < 6900 {
		t.Errorf("climbing past a point at 5000 ft: %.0f ft after 30 s, want on up", alt)
	}
	none := ConflictOptions{Profile: func(TrackedAircraft) []RoutePoint {
		return []RoutePoint{pt(4, 0, 0)}
	}}
	if _, _, alt := trackFor(a, none).at(2 * time.Minute); alt < 10700 || alt > 10900 {
		t.Errorf("no altitudes: %.0f ft after 2 min, want 10800", alt)
	}
}

// TestPathClearAlongProfile: a direct asked for is checked along the
// aircraft's profile, not at its vertical speed for ever (#657 review).
func TestPathClearAlongProfile(t *testing.T) {
	dep := air(1, "DEP", 0, 0, 3000, 90, 210, 2000, true)
	tra := air(2, "TRA", 30, 0, 10000, 270, 250, 0, false)
	all := []TrackedAircraft{dep, tra}
	path := []airport.LatLon{pt(10, 0, 0).Position, pt(40, 0, 0).Position}
	if PathClear(dep, path, all, ConflictOptions{}) {
		t.Errorf("climbing on through 10000 ft: clear, want not")
	}
	sid := ConflictOptions{Profile: func(a TrackedAircraft) []RoutePoint {
		if a.ObjectID == 1 {
			return []RoutePoint{pt(10, 0, 6000), pt(40, 0, 6000)}
		}
		return nil
	}}
	if !PathClear(dep, path, all, sid) {
		t.Errorf("levelling at 6000 ft on the SID: not clear, want clear")
	}
}

// TestPastRouteEnd: two in trail on the same route, the one behind with
// only its last waypoint, passed, left: flown straight on, not back to it
// (live, TVS524 and THY1463, #657).
func TestPastRouteEnd(t *testing.T) {
	trail := air(2, "TVS524", 8, 0, 20400, 270, 250, 0, true)
	o := ConflictOptions{Route: func(a TrackedAircraft) []airport.LatLon {
		if a.ObjectID == 2 {
			return []airport.LatLon{pt(10, 0, 0).Position} // behind it
		}
		return []airport.LatLon{pt(-60, 0, 0).Position}
	}}
	if _, _, alt := trackFor(trail, o).at(time.Minute); alt != 20400 {
		t.Fatalf("alt %.0f", alt)
	}
	if lat, lon, _ := trackFor(trail, o).at(time.Minute); calc.HaversineNM(lat, lon, trail.Position.Lat, trail.Position.Lon) < 4 ||
		calc.BearingDegrees(trail.Position.Lat, trail.Position.Lon, lat, lon) < 260 {
		t.Errorf("not straight on west: %.4f %.4f", lat, lon)
	}
}

// TestDirectWorthIt: a direct not for spacing saves at least 10 % of the
// way and ShortcutMinNM (#670).
func TestDirectWorthIt(t *testing.T) {
	for _, c := range []struct {
		along, direct float64
		want          bool
	}{
		{40, 38.5, false}, // 1.5 NM: under ShortcutMinNM
		{40, 37, false},   // 3 NM of 40: 7.5 %
		{40, 35, true},    // 5 NM of 40: 12.5 %
		{15, 12.5, true},  // 2.5 NM of 15: 17 %
	} {
		if got := DirectWorthIt(c.along, c.direct); got != c.want {
			t.Errorf("%.1f along, %.1f direct: %v, want %v", c.along, c.direct, got, c.want)
		}
	}
	// Along a straight route a fix saves nothing; around a right angle a lot.
	route := []airport.LatLon{pt(10, 0, 0).Position, pt(20, 0, 0).Position, pt(20, 20, 0).Position}
	if along, d := AlongTo(pt(0, 0, 0).Position, route, route[1]); DirectWorthIt(along, d) {
		t.Errorf("straight on: %.1f along, %.1f direct, worth it", along, d)
	}
	if along, d := AlongTo(pt(0, 0, 0).Position, route, route[2]); !DirectWorthIt(along, d) {
		t.Errorf("around the corner: %.1f along, %.1f direct, not worth it", along, d)
	}
}

// TestResolveCrossAtOrAbove: a departure whose SID levels it at 5000 ft
// under traffic at 5500 is told to cross the fix ahead at or above 7000
// feet, the climb going on, rather than stopped (#662).
func TestResolveCrossAtOrAbove(t *testing.T) {
	dep := air(1, "DEP", 0, 0, 3000, 90, 210, 2000, true)
	tra := air(2, "TRA", 15, -15, 5500, 0, 250, 0, false)
	all := []TrackedAircraft{dep, tra}
	o := ConflictOptions{
		Profile: func(a TrackedAircraft) []RoutePoint {
			if a.ObjectID == 1 {
				return []RoutePoint{pt(5, 0, 5000), pt(10, 0, 5000), pt(30, 0, 5000), pt(60, 0, 24000)}
			}
			return nil
		},
		DirectFixes: func(a TrackedAircraft) []DirectFix {
			return []DirectFix{{Ident: "VOZ", Position: pt(10, 0, 0).Position}}
		},
	}
	cs := PredictConflicts(all, o)
	if len(cs) != 1 {
		t.Fatalf("conflicts %+v, want one: levelled at 5000 under 5500", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, o)
	if !ok || r.Kind != ResolveCross || r.Fix != "VOZ" || r.AltFt != 7000 {
		t.Fatalf("%+v %v, want cross VOZ at or above 7000", r, ok)
	}
	tx := Resolved(PosDeparture, r, dep.AltFt, dep.Heading, dep.GroundKts)
	if !strings.HasPrefix(tx.Text, "DEP, cross VOZ at or above 7000 feet") {
		t.Errorf("%q, want cross VOZ at or above 7000 feet", tx.Text)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Cross VOZ at or above 7000 feet, DEP" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	// Flown: the fix at 7000 ft, the point before it raised on the way, not
	// left at 5000 to level off at.
	route := ResolvedRoute(o.Profile(dep), dep, r, 5*time.Minute)
	var fix, before RoutePoint
	for i, p := range route {
		if calc.HaversineNM(p.Position.Lat, p.Position.Lon, r.Direct.Lat, r.Direct.Lon) < 0.1 {
			fix, before = p, route[i-1]
		}
	}
	if fix.AltFt != 7000 || before.AltFt <= 5000 {
		t.Errorf("route %+v: want the fix at 7000 and the point before raised", route)
	}
	// Unable at its rate (the fix too near): no cross.
	slow := dep
	slow.VSFpm = 500
	if r, ok := ResolveConflict(cs[0], []TrackedAircraft{slow, tra}, ours, o); ok && r.Kind == ResolveCross {
		t.Errorf("at 500 fpm: %+v, want no cross", r)
	}
}

// TestMaintainLevel: an arrival level now, its route descending onto a
// level departure below: held at its level, "maintain", never sent up
// (#697).
func TestMaintainLevel(t *testing.T) {
	lvl := air(1, "TVS524", 0, 0, 6000, 90, 210, 0, false)
	arr := air(2, "BAW1413", 30, 0, 10000, 270, 250, 0, true)
	all := []TrackedAircraft{lvl, arr}
	o := ConflictOptions{Profile: func(a TrackedAircraft) []RoutePoint {
		if a.ObjectID == 2 {
			return []RoutePoint{pt(20, 0, 6000), pt(-10, 0, 4000)}
		}
		return nil
	}}
	cs := PredictConflicts(all, o)
	if len(cs) != 1 {
		t.Fatalf("conflicts %+v", cs)
	}
	r, ok := ResolveConflict(cs[0], all, ours, o)
	if !ok || r.Kind != ResolveLevel || !r.Maintain || r.AltFt != 10000 {
		t.Fatalf("%+v %v, want maintain 10000", r, ok)
	}
	tx := Resolved(PosCenter, r, arr.AltFt, arr.Heading, arr.GroundKts)
	if !strings.HasPrefix(tx.Text, "BAW1413, maintain 10000 feet") {
		t.Errorf("%q", tx.Text)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Maintain 10000 feet, BAW1413" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
}

// Two level a standard level apart are separated though the altimetry reads
// a little under 1000 ft (live: CSA111 at 36997 ft and CSA1811 at 35932 ft
// predicted 999 ft apart, CSA111 sent up to FL390); one climbing or
// descending is held to the full minimum.
func TestPredictConflictsLevelTolerance(t *testing.T) {
	level := []TrackedAircraft{air(1, "CSA111", 0, 0, 36997, 90, 450, 0, true), air(2, "CSA1811", 40, 0, 35998, 270, 450, -5, true)}
	if cs := PredictConflicts(level, ConflictOptions{}); len(cs) != 0 {
		t.Errorf("FL370 over FL360, both level: %+v", cs)
	}
	low := []TrackedAircraft{air(1, "CSA111", 0, 0, 36997, 90, 450, 0, true), air(2, "CSA1811", 40, 0, 36100, 270, 450, 0, true)}
	if cs := PredictConflicts(low, ConflictOptions{}); len(cs) == 0 {
		t.Error("897 ft apart, both level: no conflict")
	}
	climbing := []TrackedAircraft{air(1, "CSA111", 0, 0, 36997, 90, 450, 0, true), air(2, "CSA1811", 40, 0, 35600, 270, 450, 400, true)}
	if cs := PredictConflicts(climbing, ConflictOptions{}); len(cs) == 0 {
		t.Error("one climbing toward it: no conflict")
	}
}

// Two arrivals at the terminal area's top level (10000 ft) are in it: the
// terminal minimum applies to them (live: TVS1750 at 10016 ft and TVS554 at
// 10000 ft held to the en-route 5 NM in trail).
func TestTerminalTopLevel(t *testing.T) {
	o := ConflictOptions{}.withDefaults()
	a := air(1, "TVS1750", 0, 0, 10016, 240, 230, 0, true)
	b := air(2, "TVS554", 5, 0, 10000, 240, 230, 0, true)
	a.Airport, b.Airport = "LKPR", "LKPR"
	if m := o.minFor(a, b); m != o.TerminalNM {
		t.Errorf("at 10000 ft arriving: %.0f NM, want the terminal %.0f", m, o.TerminalNM)
	}
	a.AltFt = 11000
	if m := o.minFor(a, b); m != o.MinNM {
		t.Errorf("one at 11000 ft: %.0f NM, want the en-route %.0f", m, o.MinNM)
	}
}

// A route point already passed is left out of the prediction: flown back
// to, the predicted track turned round (E37).
func TestPastEndTrimsPassedPoints(t *testing.T) {
	tr := track{lat: 50, lon: 14, hdg: 90, kts: 250,
		path: []airport.LatLon{{Lat: 50, Lon: 13.9}, {Lat: 50, Lon: 14.2}},
		alts: []float64{10000, 10000}}.pastEnd()
	if len(tr.path) != 1 || tr.path[0].Lon != 14.2 || len(tr.alts) != 1 {
		t.Errorf("path %v alts %v, want the point ahead only", tr.path, tr.alts)
	}
}
