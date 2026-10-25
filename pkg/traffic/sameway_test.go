package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestSameWayFollows: one pushed onto the taxiway ahead and taxiing the
// same way is on the same way (followed); one crossing it is not (live,
// CSA429 told to give way to TVS1169 "passing left to right", both via H1
// and H).
func TestSameWayFollows(t *testing.T) {
	p := NewGroundPicture()
	now := time.Now()
	start := airport.LatLon{Lat: 50.1, Lon: 14.26}
	line := func(from airport.LatLon, hdg, m float64) []airport.LatLon {
		var out []airport.LatLon
		for d := 0.0; d <= m; d += trafficBodyStep {
			out = append(out, offsetHeading(from, hdg, d))
		}
		return out
	}
	prof := DefaultMotionProfile()
	p.Report(1, start, 316, prof, now)
	p.ReportIntent(1, line(start, 316, 400))
	// Turning in from the left 60 m ahead, then along the same way.
	ahead := offsetHeading(start, 316, 60)
	in := append(line(offsetHeading(ahead, 226, 40), 46, 40), line(ahead, 316, 300)...)
	p.Report(2, in[0], 46, prof, now)
	p.ReportIntent(2, in)
	if !p.SameWay(1, 2) {
		t.Error("not the same way: onto its taxiway ahead, the same direction")
	}
	// Crossing it.
	x := offsetHeading(offsetHeading(start, 316, 100), 226, 150)
	p.Report(3, x, 46, prof, now)
	p.ReportIntent(3, line(x, 46, 300))
	if p.SameWay(1, 3) {
		t.Error("the same way: it only crosses")
	}
	// Coming the other way.
	o := offsetHeading(start, 316, 300)
	p.Report(4, o, 136, prof, now)
	p.ReportIntent(4, line(o, 136, 300))
	if p.SameWay(1, 4) {
		t.Error("the same way: it comes the other way")
	}
}
