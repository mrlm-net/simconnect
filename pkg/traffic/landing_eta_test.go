package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestLandingETA: 100 NM out in a straight line at 420 kt: 60 NM at 420,
// 25 at 250, 10 at 180, 5 at 140 — about 22 minutes.
func TestLandingETA(t *testing.T) {
	thr := airport.LatLon{Lat: 50.1, Lon: 14.26}
	pos := offsetHeading(thr, 63, 100*1852)
	got := LandingETA(pos, 420, nil, thr)
	h := 60.0/420 + 25.0/250 + 10.0/180 + 5.0/140
	want := time.Duration(h * float64(time.Hour))
	if d := got - want; d > 30*time.Second || d < -30*time.Second {
		t.Errorf("ETA %v, want about %v", got.Round(time.Second), want.Round(time.Second))
	}
}
