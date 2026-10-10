package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A follow-me car (#890) at LKPR: sent once the CRJ is down, it waits past
// the vacate stop, leads the aircraft along its taxi-in with the aircraft
// never closer than the gap, pulls aside before the stand and drives home
// on the parked aircraft's frames; the fleet gets it back.
func TestArrivalFollowMe(t *testing.T) {
	g := lkprGraph(t)
	ec := &tugClient{}
	inj := NewInjector(ec)
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleFollowMe: 1})
	prof := ProfileFor("CRJ7").Motion
	stand := -1
	for i, p := range g.Layout.Parking {
		if !p.IsGate() && p.Size() != airport.StandNone && p.Radius*2 >= prof.SpanMeters {
			stand = i
			break
		}
	}
	if stand < 0 {
		t.Fatal("no remote stand at LKPR")
	}
	fm := NewSimObjectFollowMe(ec, inj, "FSDT_FollowMe_Hilux", 9100, prof)
	fm.Layout, fm.Graph = g.Layout, g
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithServices(fleet))
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: stand, Model: "FSLTL_CRJ7_CLH-Lufthansa CityLine", Tail: "DLH1531", FollowMe: fm}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	thr := p.End.Threshold
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(-300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(700), 12, p.End.Heading, 125, true))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	s, v := 700.0, 125*ktsToMS
	created, led, closest := false, false, math.Inf(1)
	// Leaving: where it left, how close it came to the aircraft's way onto
	// the stand (its lead-in line), how far off the taxi graph over its first
	// FollowMeAheadMeters.
	var leftAt *GroundPose
	offGraph, onLeadIn := 0.0, math.Inf(1)
	frame := func() {
		now = now.Add(time.Second / 60)
		if !inj.Driven(77) {
			v = math.Max(v-1.5/60, 0)
			s += v / 60
		}
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
		if !created && len(ec.created) == 1 {
			created = true
			ctl.Handle(assignedMsg(9100, 88))
			inj.Handle(groundMsg(DefaultInjectRequestBase+3, 88, 1200, 3))
		}
		if pose, _, ok := fm.Track(); ok && fm.phase == followMeLeaving {
			if leftAt == nil {
				leftAt = &pose
			}
			path := ctl.mover.Path()
			for s := path.Length() - ctl.standJoin + 15; s <= path.Length(); s += 2 {
				onLeadIn = math.Min(onLeadIn, localDist(pose.Position, path.PointAt(s)))
			}
			if localDist(pose.Position, leftAt.Position) < FollowMeAheadMeters {
				offGraph = math.Max(offGraph, graphDist(g, pose.Position))
			}
		}
		if fm.Leading() && ctl.State() == ArrivalTaxiing {
			led = true
			if at, ok := fm.Lead(); ok {
				closest = math.Min(closest, at+FollowMeGapMeters-ctl.mover.Pose().Distance)
			}
		}
	}
	for i := 0; i < 60*1500 && ctl.State() != ArrivalParked; i++ {
		frame()
	}
	if ctl.State() != ArrivalParked {
		at, ok := fm.Lead()
		t.Fatalf("state %v, want parked: car phase %d at %.0f (%.1f m/s), lead %.0f %v; aircraft at %.0f of %.0f, stopped by %q",
			ctl.State(), fm.phase, fm.s, fm.v, at, ok, ctl.mover.Pose().Distance, ctl.mover.Path().Length(), ctl.last.StoppedBy)
	}
	if !created || !led {
		t.Fatalf("follow-me created %v, led %v", created, led)
	}
	if closest < FollowMeGapMeters-TrafficOverrunMeters-1 {
		t.Errorf("aircraft came within %.1f m of the car along its way", closest)
	}
	if leftAt == nil {
		t.Fatal("the car never left")
	}
	// It leads to the stand's lead-in junction, not FollowMeLeaveMeters
	// short of the stand, and goes on along the taxilane: never down the
	// lead-in line, never across the grass (live at LOWI).
	if j := ctl.mover.Path().Length() - ctl.standJoin; math.Abs(leftAt.Distance-j) > 10 {
		t.Errorf("left at %.0f m along the way, the lead-in junction is at %.0f", leftAt.Distance, j)
	}
	if offGraph > 8 {
		t.Errorf("drove %.1f m off the taxi graph leaving", offGraph)
	}
	if onLeadIn < 8 {
		t.Errorf("drove down the stand's lead-in: %.1f m from it", onLeadIn)
	}
	if out, _ := fleet.Out(VehicleFollowMe); out != 1 && !fm.Done() {
		t.Errorf("car out of the fleet: %d", out)
	}
	// Parked: its frames drive the car home.
	for i := 0; i < 60*900 && !fm.Done(); i++ {
		frame()
	}
	if !fm.Done() {
		t.Fatal("the car never got home")
	}
	if out, _ := fleet.Out(VehicleFollowMe); out != 0 {
		t.Errorf("car not given back: %d out", out)
	}
	t.Logf("closest along the way %.1f m; leaving: %.1f m off the graph, %.1f m from the lead-in", closest, offGraph, onLeadIn)
}

// Without a car free in the fleet the arrival taxis in on its own.
func TestArrivalFollowMeNoneFree(t *testing.T) {
	g := lkprGraph(t)
	ec := &tugClient{}
	inj := NewInjector(ec)
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleFollowMe: 1})
	fleet.Take(VehicleFollowMe, "OTHER")
	fm := NewSimObjectFollowMe(ec, inj, "FSDT_FollowMe_Hilux", 9100, ProfileFor("CRJ7").Motion)
	fm.Layout = g.Layout
	c18, _ := g.Layout.ParkingIndex("C18")
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithServices(fleet))
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c18, Model: "FSLTL_CRJ7_CLH-Lufthansa CityLine", Tail: "DLH1531", FollowMe: fm}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	thr := p.End.Threshold
	onRunway := func(m float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, p.End.Heading, m)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(-300), 120, p.End.Heading, 140, false))
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(700), 12, p.End.Heading, 125, true))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	s, v := 700.0, 125*ktsToMS
	for i := 0; i < 60*1500 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		if !inj.Driven(77) {
			v = math.Max(v-1.5/60, 0)
			s += v / 60
		}
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("state %v, want parked", ctl.State())
	}
	if len(ec.created) != 0 {
		t.Errorf("a car was created with none free: %d", len(ec.created))
	}
}

// graphDist is how far p is from g's nearest taxi edge (meters).
func graphDist(g *airport.Graph, p airport.LatLon) float64 {
	best := math.Inf(1)
	for from, es := range g.Adj {
		for _, e := range es {
			best = math.Min(best, segmentDist(p, g.Nodes[from].Position, g.Nodes[e.To].Position))
		}
	}
	return best
}
