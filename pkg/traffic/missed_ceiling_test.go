package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestMissedWaypointsCeiling: a go-around climbs to the missed approach's
// minimum, not to a hold's ceiling, and to a ceiling where that is the
// climb level (#1659).
func TestMissedWaypointsCeiling(t *testing.T) {
	pos, next := airport.LatLon{Lat: 18.04, Lon: -63.25}, airport.LatLon{Lat: 18.10, Lon: -63.40}
	point := func(p airport.LatLon, lo, hi float64) airport.NavPoint {
		return airport.NavPoint{Position: p, AltMin: lo / ftPerMeter, AltMax: hi / ftPerMeter}
	}
	cases := []struct {
		name    string
		circuit float64
		points  []airport.NavPoint
		want    float64
	}{
		{"TNCM RNAV 10", 3013, []airport.NavPoint{point(pos, 4000, 14000)}, 4000},
		{"a hold at another fix than the minimum", 3013, []airport.NavPoint{point(pos, 4000, 0), point(next, 0, 14000)}, 4000},
		{"EBBR ILS 25R", 3184, []airport.NavPoint{point(pos, 700, 0), point(next, 0, 3000)}, 3184},
		{"EBBR ILS 25L X", 3184, []airport.NavPoint{point(pos, 700, 0), point(next, 0, 4000)}, 4000},
		{"KNPA ILS 07L Y", 3030, []airport.NavPoint{point(pos, 600, 0), point(next, 1500, 11000)}, 3030},
		{"at 5000", 3013, []airport.NavPoint{point(pos, 5000, 5000)}, 5000},
		{"only a ceiling", 3013, []airport.NavPoint{point(pos, 0, 4000)}, 4000},
	}
	for _, c := range cases {
		wps, top := missedWaypoints(c.points, c.circuit)
		if math.Abs(top-c.want) > 1 || len(wps) != len(c.points) {
			t.Errorf("%s: %d points at %.0f ft, want %.0f", c.name, len(wps), top, c.want)
			continue
		}
		for i, w := range wps {
			if math.Abs(w.Altitude-c.want) > 1 {
				t.Errorf("%s: point %d at %.0f ft, want %.0f", c.name, i+1, w.Altitude, c.want)
			}
		}
	}
}

// TestGoAroundCeilingWithHighFAF: a final approach fix above circuit height
// doesn't turn a hold's ceiling into the climb level, and the missed
// approach's points keep up with that fix (#1659).
func TestGoAroundCeilingWithHighFAF(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "A320", Tail: "CSA9",
		InjectApproach: true, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	for i := 0; i < 60*600 && ctl.approach != nil && ctl.approach.Pose().HeightFt > 300; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	ahead := func(nm float64) airport.LatLon {
		return offsetHeading(p.End.LandingThreshold(), p.End.Heading, nm*1852)
	}
	ctl.req.Procedure = []airport.NavPoint{{Ident: "FAF", Position: ahead(-6), AltMin: 5000 / ftPerMeter}}
	ctl.req.MissedApproach = []airport.NavPoint{
		{Ident: "CLIMB", Position: ahead(8), AltMin: 4500 / ftPerMeter},
		{Ident: "HOLD", Position: ahead(14), AltMax: 14000 / ftPerMeter},
	}
	if err := ctl.GoAround(); err != nil {
		t.Fatal(err)
	}
	if math.Abs(ctl.goAroundFt-5000) > 1 {
		t.Fatalf("climbs to %.0f ft, want the final approach fix's 5000", ctl.goAroundFt)
	}
	if w := ctl.proc.Waypoints[0]; math.Abs(w.Altitude-5000) > 1 {
		t.Errorf("first point of the missed approach at %.0f ft, want 5000", w.Altitude)
	}
}
