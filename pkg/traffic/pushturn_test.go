//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
)

// TestDubins: the path starts and ends at the given poses, heading as asked,
// never turns tighter than the radius and is no longer than needed (a U-turn
// to a point 2r to the side is a half circle).
func TestDubins(t *testing.T) {
	for _, c := range []struct {
		name         string
		east, north  float64
		headA, headB float64
		r, maxLen    float64
	}{
		{"u-turn", 20, 0, 0, 180, 10, math.Pi*10 + 0.5},
		{"straight", 0, 50, 0, 0, 10, 50.5},
		{"quarter", 20, 20, 0, 90, 20, math.Pi*10 + 0.5},
		{"s-bend", 10, 60, 0, 0, 15, 70},
	} {
		b := offset(lkpr, c.east, c.north)
		pts := dubins(lkpr, c.headA, b, c.headB, c.r, 0.25)
		if pts == nil {
			t.Fatalf("%s: no path", c.name)
		}
		if d := localDist(pts[len(pts)-1], b); d > 0.2 {
			t.Errorf("%s: ends %.2f m from the target", c.name, d)
		}
		n := len(pts)
		if d := math.Abs(headingDiff(localBearing(pts[0], pts[2]), c.headA)); d > 3 {
			t.Errorf("%s: starts %.1f° off", c.name, d)
		}
		if d := math.Abs(headingDiff(localBearing(pts[n-3], pts[n-1]), c.headB)); d > 3 {
			t.Errorf("%s: ends %.1f° off", c.name, d)
		}
		if l := pathLen(pts); l > c.maxLen {
			t.Errorf("%s: %.1f m long, want at most %.1f", c.name, l, c.maxLen)
		}
		for i := 2; i+2 < n; i += 2 {
			turn := math.Abs(headingDiff(localBearing(pts[i-2], pts[i]), localBearing(pts[i], pts[i+2])))
			if turn > 1e-6 {
				if rad := localDist(pts[i-2], pts[i+2]) / (turn * math.Pi / 180); rad < c.r*0.95 {
					t.Fatalf("%s: turns on %.1f m", c.name, rad)
				}
			}
		}
	}
}

// A pushback does not leave the aircraft on another taxiway: at LKPR A4 the
// tail went north on the B1 lane and the aircraft stood across H; it now
// goes south into the B1 alley (a wider swing) and blocks nothing. Over all
// stands, far fewer pushes end on another taxiway's junction (31 of 103
// before).
func TestPushbackDoesNotBlockTaxiways(t *testing.T) {
	g := lkprGraph(t)
	blocks := func(stand, rwy string) (int, bool) {
		i, err := g.Layout.ParkingIndex(stand)
		if err != nil {
			return -1, false
		}
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL A320 Air France SL", Tail: "T1"}); err != nil || !ctl.havePushBranch {
			return -1, false
		}
		k := ctl.route.Nodes[ctl.pushJunction]
		for _, e := range g.Adj[k] {
			if e.To == ctl.pushBranch {
				return ctl.pushBlocks(k, e), true
			}
		}
		return -1, false
	}
	for _, s := range []string{"A4", "C21"} {
		if n, ok := blocks(s, "06"); !ok || n != 0 {
			t.Errorf("%s → 06: blocks %d junctions (planned %v)", s, n, ok)
		}
	}
	total, blocking := 0, 0
	for _, p := range g.Layout.Parking {
		if n, ok := blocks(p.Label(), "06"); ok {
			total++
			if n > 0 {
				blocking++
			}
		}
	}
	if blocking > total/8 {
		t.Errorf("%d of %d pushes end on another taxiway", blocking, total)
	}
	t.Logf("%d of %d pushes to 06 end on another taxiway", blocking, total)
}

// No push leaves the nose facing back at the stand: straight on through a
// junction behind it (LKPR A4, live: an E190 pushed across B1 and faced the
// dead-end lead-in, where an A320 swung into the B1 alley). Every type
// from A4 swings south into the alley, nose north-west.
func TestPushbackDoesNotFaceTheStand(t *testing.T) {
	g := lkprGraph(t)
	a4, _ := g.Layout.ParkingIndex("A4")
	for _, m := range []string{"FSLTL_E190_LOT_GRZESKI", "FSLTL A320 Air France SL", "FSLTL_FAIB_B738_TVS-Smartwings_NC"} {
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: a4, Runway: "06", Model: m, Tail: "T1"}); err != nil {
			t.Fatal(err)
		}
		p, err := ctl.pushPath()
		if err != nil {
			t.Fatal(err)
		}
		end := p.PointAt(p.Length())
		nose := localBearing(end, p.PointAt(p.Length()-5))
		if d := math.Abs(headingDiff(nose, 327)); d > 25 {
			t.Errorf("%s from A4: nose %.0f°, want about 327° (into the B1 alley)", m, nose)
		}
	}
	// Across the stands, no ordinary push goes straight on across a named
	// taxiway (straight on along an unnamed lead-in is a push along it).
	for i := range g.Layout.Parking {
		ec := &eventClient{}
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "06", Model: "FSLTL_E190_LOT_GRZESKI", Tail: "T1"}); err != nil || !ctl.havePushBranch || ctl.pushPts != nil {
			continue
		}
		j := ctl.pushJunction
		in := localBearing(ctl.route.Points[j-1], ctl.route.Points[j])
		out := localBearing(ctl.route.Points[j], g.Nodes[ctl.pushBranch].Position)
		name := ""
		for _, e := range g.Adj[ctl.route.Nodes[j]] {
			if e.To == ctl.pushBranch {
				name = e.Name
			}
		}
		if name != "" && math.Abs(headingDiff(in, out)) < minPushSwingDeg-0.01 {
			t.Errorf("%s: pushed straight on past the junction (swing %.0f°)", g.Layout.Parking[i].Label(), math.Abs(headingDiff(in, out)))
		}
	}
}

// A push does not tow the aircraft along through taxiways (LKPR A3, live:
// an A321 pushed 190 m up A1 and along Z, into the taxi-out of the aircraft
// pushed before it; the two met head on), nor leave the nose facing away
// from the way out (LKPR A5, live: KLM594 faced south-east on B1 and turned
// 127° back onto B2): A3 pushes onto A1 nose north, A5 onto B1 nose
// north-west.
func TestPushbackNoTowNoHairpin(t *testing.T) {
	g := lkprGraph(t)
	for _, c := range []struct {
		stand string
		nose  float64
	}{{"A3", 334}, {"A5", 326}} {
		i, _ := g.Layout.ParkingIndex(c.stand)
		for _, m := range []string{"FSLTL_FAIB_A321_WZZ-Wizz Air", "FSLTL_SBAI_BCS3_CSA-Lines", "FSLTL A320 Air France SL"} {
			ec := &eventClient{}
			ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: "06", Model: m, Tail: "T1"}); err != nil {
				t.Fatal(err)
			}
			p, err := ctl.pushPath()
			if err != nil {
				t.Fatal(err)
			}
			end := p.PointAt(p.Length())
			nose := localBearing(end, p.PointAt(p.Length()-5))
			if d := math.Abs(headingDiff(nose, c.nose)); d > 20 || p.Length() > 150 {
				t.Errorf("%s from %s: push %.0f m, nose %.0f°, want at most 150 m, nose about %.0f°", m, c.stand, p.Length(), nose, c.nose)
			}
		}
	}
}
