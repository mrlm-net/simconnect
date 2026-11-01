package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A step in the runway mesh: placed where the ground was seen before, not
// on the last sample a frame behind (live, LKPR 24 at 76 kt).
func TestGroundUnderUsesTheMap(t *testing.T) {
	i := &Injector{}
	a, b := airport.LatLon{Lat: 50.1, Lon: 14.25}, airport.LatLon{Lat: 50.1, Lon: 14.25003}
	i.ground.put(a, 1200)
	i.ground.put(b, 1199.69)
	o := &injected{groundFt: 1200} // its last sample: at a
	if got := i.groundUnder(o, b); got != float64(float32(1199.69)) {
		t.Errorf("at b: %.2f, want the ground seen there", got)
	}
	if got := i.groundUnder(o, airport.LatLon{Lat: 50.2, Lon: 14.25}); got != 1200 {
		t.Errorf("not seen: %.2f, want its sample", got)
	}
	i.ground.put(b, 1250) // a bridge above: not taken
	if got := i.groundUnder(o, b); got != 1200 {
		t.Errorf("far off its sample: %.2f, want its sample", got)
	}
}

// A pitch passing through the rest pitch is kept off it: there the
// simulator draws compressed struts for a frame.
func TestOffRest(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0.71, 0.71 + offRestDeg}, {0.70, 0.71 - offRestDeg}, {0.80, 0.80}, {1.71, 1.71},
	} {
		if got := offRest(tc.in, 0.71); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("offRest(%.2f): %.3f, want %.3f", tc.in, got, tc.want)
		}
	}
}

// Under a standing aircraft a new ground height (the scenery swapping
// detail) is eased in, not taken in a frame; rolling, followed faster.
func TestGroundShownEasedStanding(t *testing.T) {
	at := time.Unix(1000, 0)
	i := &Injector{now: func() time.Time { return at }}
	p := airport.LatLon{Lat: 50.1, Lon: 14.25}
	o := &injected{groundFt: 1176.92}
	i.ground.put(p, 1177.40)
	if got := i.groundShown(o, GroundPose{Position: p}); math.Abs(got-1177.40) > 0.01 {
		t.Fatalf("first: %.2f", got)
	}
	i.ground.put(p, 1176.92) // 0.48 ft lower, the next frame
	at = at.Add(20 * time.Millisecond)
	if got := i.groundShown(o, GroundPose{Position: p}); 1177.40-got > 0.03 {
		t.Errorf("standing: %.2f, want eased from 1177.40", got)
	}
	at = at.Add(time.Second)
	if got := i.groundShown(o, GroundPose{Position: p}); math.Abs(got-1176.92) > 0.01 {
		t.Errorf("a second on: %.2f, want 1176.92", got)
	}
	i.ground.put(p, 1177.40)
	at = at.Add(20 * time.Millisecond)
	if got := i.groundShown(o, GroundPose{Position: p, GroundSpeedKts: 10}); math.Abs(got-(1176.92+0.16)) > 0.01 {
		t.Errorf("rolling: %.2f, want %.2f in 20 ms", got, 1176.92+0.16)
	}
	at = at.Add(100 * time.Millisecond)
	if got := i.groundShown(o, GroundPose{Position: p, GroundSpeedKts: 10}); math.Abs(got-1177.40) > 0.01 {
		t.Errorf("rolling, later: %.2f, want the ground", got)
	}
	// A bridge or a stale cell's distance away: at once.
	i.ground.cells = nil
	o.groundFt = 1190
	at = at.Add(20 * time.Millisecond)
	if got := i.groundShown(o, GroundPose{Position: p, GroundSpeedKts: 10}); got != 1190 {
		t.Errorf("far off: %.2f, want 1190", got)
	}
}
