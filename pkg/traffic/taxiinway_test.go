package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestTaxiInWayHasStandJoin (#1694): taxiing in, an arrival's TaxiInWay is
// the ground path it drives from where it is to its stop on the stand: it
// ends where that path ends and takes in the join onto the stand, off the
// route it follows; in the air there is none.
func TestTaxiInWayHasStandJoin(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c18, _ := g.Layout.ParkingIndex("C18")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c18, Model: "FSLTL_CRJ7_CLH-Lufthansa CityLine", Tail: "DLH1531", RollThroughChance: 1}); err != nil {
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
	if w := ctl.TaxiInWay(); w != nil {
		t.Errorf("on final: a taxi-in way of %d points", len(w))
	}
	ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(700), 12, p.End.Heading, 125, true))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	s, v := 700.0, 125*ktsToMS
	var way []airport.LatLon
	for i := 0; i < 60*1200 && way == nil; i++ {
		now = now.Add(time.Second / 60)
		if !inj.Driven(77) {
			v = math.Max(v-1.5/60, 0)
			s += v / 60
		}
		ctl.Handle(arrivalPositionMsg(mon, 77, onRunway(s), 12, p.End.Heading, v/ktsToMS, true))
		if ctl.State() == ArrivalTaxiing {
			way = ctl.TaxiInWay()
		}
	}
	if len(way) < 2 {
		t.Fatalf("taxiing: a taxi-in way of %d points", len(way))
	}
	ctl.mu.Lock()
	pts := ctl.mover.Path().Points()
	route := ctl.plan.Route.Points
	ctl.mu.Unlock()
	if d := localDist(way[len(way)-1], pts[len(pts)-1]); d > 0.5 {
		t.Errorf("ends %.1f m from its ground path's end", d)
	}
	// Off the route: the join onto the stand, more than a few metres from
	// every stretch of the route.
	off := 0.0
	for _, q := range way {
		near := math.Inf(1)
		for i := 1; i < len(route); i++ {
			a, b := route[i-1], route[i]
			n := max(1, int(localDist(a, b)))
			for k := 0; k <= n; k++ {
				f := float64(k) / float64(n)
				near = math.Min(near, localDist(q, airport.LatLon{Lat: a.Lat + f*(b.Lat-a.Lat), Lon: a.Lon + f*(b.Lon-a.Lon)}))
			}
		}
		off = math.Max(off, near)
	}
	if off < 3 {
		t.Errorf("its way at most %.1f m off the route: no join onto the stand", off)
	}
}
