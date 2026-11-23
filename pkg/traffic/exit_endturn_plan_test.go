package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// midExitGraph is the builder sample's field (#1631): runway 09/27 of
// 2500 m, a right-angle exit C at mid-length, B and D at the ends leaving
// back by back metres over their 75 m (10: about 98°, no exit for an
// aircraft landing towards them), a parallel taxiway A, two stands.
func midExitGraph(t *testing.T, back float32) *airport.Graph {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	xs := []float32{-1250, -625, 0, 625, 1250}
	exits := map[float32]uint32{-1250: 2, 0: 3, 1250: 4}
	rwy, a := map[float32]int32{}, map[float32]int32{}
	for i, x := range xs {
		rwy[x] = pt(x, 0, 1)
		if i > 0 {
			link(rwy[xs[i-1]], rwy[x], 2, 0, 45, 9)
		}
		name, ok := exits[x]
		if !ok {
			continue
		}
		hx := x - float32(math.Copysign(float64(back), float64(x)))
		if x == 0 {
			hx = 0
		}
		a[x] = pt(hx, 180, 1)
		h := pt(hx, 75, 2)
		link(rwy[x], h, 1, name, 23, 0)
		link(h, a[x], 1, name, 23, 0)
	}
	apron := pt(300, 180, 1)
	link(a[-1250], a[0], 1, 1, 23, 0)
	link(a[0], apron, 1, 1, 23, 0)
	link(apron, a[1250], 1, 1, 23, 0)
	lead := pt(300, 260, 1)
	link(apron, lead, 1, 5, 20, 0)
	link(lead, 0, 3, 0, 20, 0)
	lead2 := pt(380, 260, 1)
	link(apron, lead2, 1, 5, 20, 0)
	link(lead2, 1, 3, 0, 20, 0)
	raw := airport.RawAirport{ICAO: "XMID", Name: "Mid exit", Latitude: 50.1214, Longitude: 13.8124, Altitude: 300,
		Runways: []airport.RawRunway{{Latitude: 50.1214, Longitude: 13.8124, Altitude: 300, Heading: 90, Length: 2500, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking: []airport.RawParking{
			{Name: 14, Number: 1, Type: 9, Heading: 0, Radius: 24, BiasX: 300, BiasZ: 310},
			{Name: 14, Number: 2, Type: 9, Heading: 0, Radius: 24, BiasX: 380, BiasZ: 310},
		},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "C", "D", "S"}}
	layout, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(layout)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestEndTurnPlannedUpFront (#1631): planned onto 27 of the builder
// sample, where C at mid-length is the only exit ahead and out of reach of
// the touchdown at the type's own speed and braking, the plan is the roll
// to the end and back to C from the start: no exit it makes only too fast,
// and the runway time the tower counts on (BacktrackOccupancy) has the
// roll to the end and back. Before, C was planned as a plain exit, its
// occupancy that of a turn-off at mid-length. With B at the end a right
// angle, B is planned and made.
func TestEndTurnPlannedUpFront(t *testing.T) {
	for _, model := range []string{"A321", "A20N", "B738", "B77W"} {
		ap := ProfileFor(model)
		ro := withDefaults(ap.Rollout, DefaultRolloutProfile())
		o := ArrivalOptions{TouchdownKts: ap.Approach.TouchdownKts, BrakeDecel: ro.BrakeDecel}
		reach := exitReach{o.TouchdownKts, o.BrakeDecel}

		g := midExitGraph(t, 10)
		p, err := PlanArrival(g, "27", 0, o)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		t.Logf("%s (%.0f kt, %.1f m/s²), no exit at the end: exit %s at %.0f m, end turn %v, backtrack %.0f m, occupancy +%s",
			model, o.TouchdownKts, o.BrakeDecel, p.Exit.Taxiway, p.Exit.Along, p.EndTurn, p.BacktrackMeters(), BacktrackOccupancy(p.BacktrackMeters()))
		if !p.EndTurn || !p.Backtrack || p.Exit.Taxiway != "C" {
			t.Errorf("%s: planned %s, end turn %v: want the roll to the end and back to C", model, p.Exit.Taxiway, p.EndTurn)
		}
		if m := p.BacktrackMeters(); m < 2*(1254-100) {
			t.Errorf("%s: %.0f m on the runway past C and back, want the roll to the end and back (≥ %d)", model, m, 2*(1254-100))
		}
		if p.ClearAt() >= len(p.Route.Points) || p.Route.Nodes[p.ClearAt()] != p.Exit.Node {
			t.Errorf("%s: off the runway at route point %d, want C's first taxiway node", model, p.ClearAt())
		}

		g = midExitGraph(t, 0)
		p, err = PlanArrival(g, "27", 0, o)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if p.EndTurn || p.Exit.Taxiway != "B" || !reach.makes(g, "27", p.Exit) {
			t.Errorf("%s, B at the end: planned %s, end turn %v; want B, made", model, p.Exit.Taxiway, p.EndTurn)
		}
	}
}

// TestFirmExitOwnTouchdown (#1631): the firm exit is chosen for the type's
// own touchdown speed, not the airliner's planned one: an exit made from
// 125 kt braking at 2.5 m/s² but not from 140 kt is not one for a type
// touching down at 140.
func TestFirmExitOwnTouchdown(t *testing.T) {
	g := midExitGraph(t, 0)
	exits, err := g.RunwayExits("27")
	if err != nil {
		t.Fatal(err)
	}
	airliner, fast := exitReach{TouchdownSpeedKts, 2.5}, exitReach{140, 2.5}
	for _, e := range exits {
		if e.Taxiway != "C" {
			continue
		}
		e.Along = (airliner.firmRollout(ExitSpeed(e)) + fast.firmRollout(ExitSpeed(e))) / 2
		if !airliner.makes(g, "27", e) || fast.makes(g, "27", e) {
			t.Errorf("exit at %.0f m: made at 125 kt %v, at 140 kt %v; want true, false", e.Along, airliner.makes(g, "27", e), fast.makes(g, "27", e))
		}
		if _, _, ok := firmExit(g, "27", []airport.RunwayExit{e}, 0, airport.RouteOptions{}, fast); ok {
			t.Errorf("firmExit picked an exit at %.0f m for a 140 kt touchdown", e.Along)
		}
	}
}
