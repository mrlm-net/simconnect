package traffic

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// A CRJ taxiing in along J to LKPR C18 while a B738 pushes back from C19
// onto JB: re-planned by JO, it parks on C18 without passing the pushback
// (live: DLH1531 and TVS906 met head-on).
func TestArrivalAvoidOccupied(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c18, _ := g.Layout.ParkingIndex("C18")
	c19, _ := g.Layout.ParkingIndex("C19")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c18, Model: "FSLTL_CRJ7_CLH-Lufthansa CityLine", Tail: "DLH1531", RollThroughChance: 1}); err != nil {
		t.Fatal(err)
	}
	p := ctl.Plan()
	if !slices.Contains(p.Route.Taxiways, "J") {
		t.Skipf("taxi-in %v does not use J", p.Route.Taxiways)
	}
	pushed := airport.Occupied{Points: []airport.LatLon{g.Layout.Parking[c19].Position, g.Nodes[980].Position, g.Nodes[981].Position}, HalfSpan: 17.9}
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
	replanned, nearest := false, math.Inf(1)
	for i := 0; i < 60*1200 && ctl.State() != ArrivalParked; i++ {
		now = now.Add(time.Second / 60)
		if !inj.Driven(77) {
			v = math.Max(v-1.5/60, 0)
			s += v / 60
		}
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
		if !replanned && ctl.State() == ArrivalTaxiing && ctl.last.Taxiway == "J" && localDist(ctl.last.Position, g.Nodes[980].Position) < 400 {
			if !ctl.AvoidOccupied([]airport.Occupied{pushed}) {
				t.Fatalf("not re-planned on J %.0f m from the pushback", localDist(ctl.last.Position, g.Nodes[980].Position))
			}
			replanned = true
		}
		if replanned && ctl.State() == ArrivalTaxiing {
			for _, q := range pushed.Points[1:] {
				nearest = math.Min(nearest, localDist(ctl.last.Position, q))
			}
		}
	}
	if !replanned {
		t.Fatal("never on J near the pushback")
	}
	if ctl.State() != ArrivalParked {
		t.Fatalf("state %v, want parked", ctl.State())
	}
	r := ctl.Plan().Route
	t.Logf("taxi-in %v → %v, nearest the pushed aircraft on JB %.0f m", p.Route.Taxiways, r.Taxiways, nearest)
	if !slices.Contains(r.Taxiways, "JO") {
		t.Errorf("taxi-in %v, want by JO", r.Taxiways)
	}
	if nearest < 11.6+17.9 {
		t.Errorf("passed the pushback %.0f m away, want at least %.1f", nearest, 11.6+17.9)
	}
}
