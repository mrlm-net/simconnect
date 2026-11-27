package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// An arrival taxiing in and a departure meaning to taxi the other way
// along one taxiway: the departure holds at its last junction before the
// shared stretch, whichever is further from it, and the arrival does not
// give way to the departure's way there; one holds, never both (nightly
// v0.199.0, XDEC: XDA1 held for XDD1, which gave way to XDA1's way up to
// that junction, and each stood for the other).
func TestOncomingArrivalKeepsPriority(t *testing.T) {
	now := time.Now()
	line := func(lon0, lon1 float64) []airport.LatLon { // along lat 50, from lon0 to lon1
		var out []airport.LatLon
		n := int(math.Abs(lon1-lon0)/0.0002 + 0.5)
		for k := 0; k <= n; k++ {
			out = append(out, airport.LatLon{Lat: 50, Lon: lon0 + (lon1-lon0)*float64(k)/float64(n)})
		}
		return out
	}
	p := NewGroundPicture()
	prof := DefaultMotionProfile()
	// 1 at 14.000 going east, short of the shared stretch from 14.002 to
	// 14.004; 2 coming south from a side way into it at 14.004, turning
	// west: neither in it yet.
	side := []airport.LatLon{{Lat: 50.0010, Lon: 14.004}, {Lat: 50.0005, Lon: 14.004}}
	way2 := append(append([]airport.LatLon(nil), side...), line(14.004, 14.002)...)
	p.Report(1, airport.LatLon{Lat: 50, Lon: 14.000}, 90, prof, now)
	p.Report(2, side[0], 180, prof, now)
	p.ReportIntent(2, way2)
	p.ReportIntent(1, append(line(14.000, 14.002), line(14.002, 14.006)[1:]...))
	path1, _ := NewGroundPath(append(line(14.000, 14.002), line(14.002, 14.006)[1:]...), prof)
	path2, _ := NewGroundPath(way2, prof)
	for _, inbound := range []uint32{1, 2} {
		p.ReportInbound(1, inbound == 1)
		p.ReportInbound(2, inbound == 2)
		_, _, hold1 := p.oncoming(1, path1, 0, 17, now)
		_, _, hold2 := p.oncoming(2, path2, 0, 17, now)
		if hold1 == (inbound == 1) || hold2 == (inbound == 2) {
			t.Errorf("arrival %d: 1 holds %v, 2 holds %v; want the departure only", inbound, hold1, hold2)
		}
	}
	// Held for the other at its junction: not given way to by it.
	p.ReportInbound(1, false)
	p.ReportInbound(2, false)
	p.ReportPath(1, line(14.000, 14.0019), prof.SpanMeters/2)
	p.ReportPath(2, way2, prof.SpanMeters/2)
	before, who := p.giveWayTo(2, path2, 0, GiveWayLookMeters, prof.SpanMeters/2, now)
	p.ReportHeldFor(1, 2)
	after, _ := p.giveWayTo(2, path2, 0, GiveWayLookMeters, prof.SpanMeters/2, now)
	if math.IsInf(before, 1) || who != 1 {
		t.Fatalf("2 gives way to nobody before 1 holds for it (%v, %d): nothing checked", before, who)
	}
	if !math.IsInf(after, 1) {
		t.Errorf("2 gives way at %.0f m to 1, which holds for it", after)
	}
}
