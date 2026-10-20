package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestRescueTruck: the standby points lie beside runway 24, off its edge;
// a truck drives there from the depot and waits; after touchdown it
// follows the aircraft down the runway, never turning faster than its
// limit nor jumping, and stops RescueGapMeters behind it; then it drives
// home and is removed.
func TestRescueTruck(t *testing.T) {
	g := lkprGraph(t)
	rwy, end, ok := g.Layout.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	spots := RescueStandby(g.Layout, "24", 2)
	if len(spots) != 2 {
		t.Fatalf("%d standby points, want 2", len(spots))
	}
	for _, s := range spots {
		along, cross := runwayFrame(end, s.Position)
		if along < 0 || along > rwy.Length || math.Abs(cross) < rwy.Width/2+RescueStandbyAsideMeters-5 {
			t.Errorf("standby %.0f m along, %.0f m across: not beside the runway", along, cross)
		}
	}
	ec := &tugClient{}
	inj := NewInjector(ec)
	r := NewSimObjectRescue(ec, inj, "Truck Fire Airport Medium", 9200)
	r.Layout = g.Layout
	if err := r.Standby(spots[0]); err != nil {
		t.Fatal(err)
	}
	if !r.hasDepot {
		t.Error("no road from a depot: created at its standby")
	}
	r.Handle(assignedMsg(9200, 501))
	const dt = 1.0 / 60
	for i := 0; i < 60*60*10 && r.mover != nil; i++ {
		if err := r.Update(dt); err != nil {
			t.Fatal(err)
		}
	}
	if d := localDist(r.pose.Position, spots[0].Position); r.mover != nil || d > 10 {
		t.Fatalf("not at standby: %.0f m off", d)
	}
	// The aircraft rolls out from 400 m in at 120 kt down to a stop.
	s, kts := 400.0, 120.0
	at := func() GroundPose {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, s)
		return GroundPose{Position: airport.LatLon{Lat: lat, Lon: lon}, Heading: end.Heading}
	}
	prev := r.pose
	for i := 0; i < 60*240; i++ {
		s += kts * knot * dt
		kts = math.Max(0, kts-1.5/knot*dt)
		if err := r.Follow(at(), kts, dt); err != nil {
			t.Fatal(err)
		}
		if turn := math.Abs(headingDiff(prev.Heading, r.pose.Heading)); turn > rescueTurnDegS*dt+1e-6 {
			t.Fatalf("frame %d: turned %.2f° in a frame", i, turn)
		}
		if step := localDist(prev.Position, r.pose.Position); step > RescueFollowKts*knot*dt+0.01 {
			t.Fatalf("frame %d: jumped %.2f m", i, step)
		}
		prev = r.pose
	}
	if d := localDist(r.pose.Position, at().Position); math.Abs(d-RescueGapMeters) > 20 {
		t.Errorf("stopped %.0f m from the aircraft, want about %.0f", d, RescueGapMeters)
	}
	if err := r.Home(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60*60*15 && !r.Done(); i++ {
		if err := r.Update(dt); err != nil {
			t.Fatal(err)
		}
	}
	if !r.Done() {
		t.Error("never home")
	}
}

// runwayFrame is p along the runway end's direction from its threshold
// and across it (right positive), in metres.
func runwayFrame(end airport.RunwayEnd, p airport.LatLon) (along, cross float64) {
	d := localDist(end.Threshold, p)
	a := (localBearing(end.Threshold, p) - end.Heading) * math.Pi / 180
	return d * math.Cos(a), d * math.Sin(a)
}
