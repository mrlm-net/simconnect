package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// passDep is a departure on its stand with the push area it will sweep.
type passDep struct {
	departureCtl
	area airport.Occupied
}

func (d *passDep) State() traffic.TaxiState           { return traffic.TaxiAwaitingPushback }
func (d *passDep) Route() *airport.Route              { return nil }
func (d *passDep) PushArea() (airport.Occupied, bool) { return d.area, len(d.area.Points) > 0 }
func (d *passDep) Occupies() (airport.Occupied, bool) { return airport.Occupied{}, false }

// passArr is an arrival taxiing in along route.
type passArr struct {
	arrivalCtl
	state traffic.ArrivalState
	plan  *traffic.ArrivalPlan
}

func (a *passArr) State() traffic.ArrivalState { return a.state }
func (a *passArr) Plan() *traffic.ArrivalPlan  { return a.plan }

// The live case (LKPR 2026-11-11): CSA1889 on C26 pushed onto H1 facing
// north-west, ending at 50.10439 14.26463; TVS1832 taxied in along H1 the
// other way to C27, gave way to the push 245 m out, and the two stood nose
// to nose for good. Ground holds the push while TVS1832 comes; parked, or
// far off, it does not.
func TestPushHeldForTrafficPassingBehind(t *testing.T) {
	b := newJWBed(t)
	c26, err := b.g.Layout.ParkingIndex("C26")
	if err != nil {
		t.Fatal(err)
	}
	c27, err := b.g.Layout.ParkingIndex("C27")
	if err != nil {
		t.Fatal(err)
	}
	st := b.g.Layout.Parking[c26]
	end := airport.LatLon{Lat: 50.10439, Lon: 14.26463}
	var area []airport.LatLon
	for f := 0.0; f <= 1; f += 0.1 { // stand to the push's end, then the tail beyond (facing 306)
		area = append(area, airport.LatLon{Lat: st.Position.Lat + f*(end.Lat-st.Position.Lat), Lon: st.Position.Lon + f*(end.Lon-st.Position.Lon)})
	}
	for d := 5.0; d <= 20; d += 5 {
		lat, lon := calc.DisplaceByHeading(end.Lat, end.Lon, 126, d)
		area = append(area, airport.LatLon{Lat: lat, Lon: lon})
	}
	dep := b.park(1, "CSA1889", "BCS3", c26, 7001, false)
	dep.dep = &passDep{area: airport.Occupied{Points: area, HalfSpan: 17.5}}

	// TVS1832 where it stood, then 250 m back up H1 the way it came.
	at := func(back float64) (*traffic.ArrivalPlan, airport.LatLon) {
		t.Helper()
		lat, lon := calc.DisplaceByHeading(50.10476, 14.26380, 305, back)
		n, ok := b.g.NearestNode(airport.LatLon{Lat: lat, Lon: lon}, 250)
		if !ok {
			t.Fatalf("no node %0.f m up H1", back)
		}
		r, err := b.g.RouteToParking(n, c27, airport.RouteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return &traffic.ArrivalPlan{Route: r}, r.Points[0]
	}
	plan, pos := at(250)
	arr := b.park(2, "TVS1832", "B738", c27, 7002, true)
	a := &passArr{state: traffic.ArrivalTaxiing, plan: plan}
	arr.arr = a
	arr.view.Position = pos

	if o := dep.passingBehind(); o != arr {
		t.Fatalf("push not held for TVS1832 coming along H1 250 m out: %v", o)
	}
	a.state = traffic.ArrivalParked
	if o := dep.passingBehind(); o != nil {
		t.Errorf("held for %s parked", o.Tail)
	}
	a.state = traffic.ArrivalTaxiing
	a.plan, arr.view.Position = at(1000)
	if o := dep.passingBehind(); o != nil {
		t.Errorf("held for %s 1 km out", o.Tail)
	}
}
