package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// An arrival coming in keeps its way off the runway (#1719): a departure
// taxiing to hold short on that exit stops short of the junction where it
// would turn into it, clear of it, until the arrival has vacated; never on
// the exit, nose to nose with it. One passing the junction goes on, one
// whose stop would be on the arrival's taxi-in does not stop there (the
// arrival plans its way off clear instead), nor one already in the exit.
//
//	A  ----------------J------------------  (lat 50.0016)
//	                   X (exit; H, the holding point, half way)
//	runway ============+==================  (lat 50.0000)
//	                lon 14.010
func TestWayOffKeptClear(t *testing.T) {
	now := time.Now()
	pt := func(lat, lon float64) airport.LatLon { return airport.LatLon{Lat: lat, Lon: lon} }
	line := func(a, b airport.LatLon) []airport.LatLon {
		n := int(math.Max(1, localDist(a, b)/trafficBodyStep))
		out := make([]airport.LatLon, 0, n+1)
		for k := 0; k <= n; k++ {
			f := float64(k) / float64(n)
			out = append(out, pt(a.Lat+(b.Lat-a.Lat)*f, a.Lon+(b.Lon-a.Lon)*f))
		}
		return out
	}
	rwyX, j, h := pt(50, 14.010), pt(50.0016, 14.010), pt(50.0008, 14.010)
	prof := DefaultMotionProfile()
	half := prof.SpanMeters / 2
	way, in := line(rwyX, j), line(j, pt(50.0016, 14.016))
	path := func(pts ...airport.LatLon) *GroundPath {
		var all []airport.LatLon
		for i := 1; i < len(pts); i++ {
			seg := line(pts[i-1], pts[i])
			if i > 1 {
				seg = seg[1:]
			}
			all = append(all, seg...)
		}
		g, err := NewGroundPath(all, prof)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	p := NewGroundPicture()
	toHold := path(pt(50.0016, 14.007), j, h)
	if at, who := p.giveWayTo(1, toHold, 0, GiveWayLookMeters, half, now); !math.IsInf(at, 1) || who != 0 {
		t.Fatalf("no way off kept: stops at %.0f m for %d", at, who)
	}
	p.ReportWayOff(2, way, in, half, now)
	toJ, _ := toHold.DistanceTo(j)
	at, who := p.giveWayTo(1, toHold, 0, GiveWayLookMeters, half, now)
	if who != 2 || at >= toJ-half || at <= 0 {
		t.Errorf("to hold short on the exit: stops at %.0f m for %d; want for 2, short of the junction %.0f m on by more than %.0f m", at, who, toJ, half)
	}
	cases := []struct {
		name string
		path *GroundPath
	}{
		{"passing the junction", path(pt(50.0016, 14.007), pt(50.0016, 14.020))},
		{"its stop on the arrival's taxi-in", path(pt(50.0016, 14.014), j, h)},
		{"in the exit already", path(pt(50.0013, 14.010), h)},
	}
	for _, c := range cases {
		if at, who := p.giveWayTo(1, c.path, 0, GiveWayLookMeters, half, now); !math.IsInf(at, 1) {
			t.Errorf("%s: stops at %.0f m for %d; want on", c.name, at, who)
		}
	}
	if at, _ := p.giveWayTo(1, toHold, 0, GiveWayLookMeters, half, now.Add(TrafficStaleAfter+time.Second)); !math.IsInf(at, 1) {
		t.Errorf("a way off not reported again: still stops at %.0f m", at)
	}
	p.ReportWayOff(2, nil, nil, half, now)
	if at, _ := p.giveWayTo(1, toHold, 0, GiveWayLookMeters, half, now); !math.IsInf(at, 1) {
		t.Errorf("the arrival vacated: still stops at %.0f m", at)
	}
}
