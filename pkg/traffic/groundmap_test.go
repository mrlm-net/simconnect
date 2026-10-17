package traffic

import (
	"math"
	"testing"

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
