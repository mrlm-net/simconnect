package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestSunElevation against the geometry: at Prague (50.1° N) the noon sun
// stands 90 − 50.1 + declination high (about 63° at the June solstice, 40°
// at the equinox, 17° at the December solstice); the sun is down at
// midnight; it is day at 10:00 and night at 22:00 local in October.
func TestSunElevation(t *testing.T) {
	prg := airport.LatLon{Lat: 50.1, Lon: 14.26}
	noon := func(y int, m time.Month, d int) time.Time {
		// Local solar noon: 12:00 UTC less the longitude's 4 min per degree.
		return time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Add(-57 * time.Minute)
	}
	for _, c := range []struct {
		at   time.Time
		want float64
	}{
		{noon(2026, 6, 21), 63.3}, {noon(2026, 3, 20), 40}, {noon(2026, 12, 21), 16.5},
	} {
		if got := SunElevation(prg, c.at); math.Abs(got-c.want) > 1 {
			t.Errorf("%v: sun at %.1f°, want about %.1f°", c.at, got, c.want)
		}
	}
	if e := SunElevation(prg, time.Date(2026, 6, 21, 23, 0, 0, 0, time.UTC)); e > -10 {
		t.Errorf("midnight sun at %.1f°", e)
	}
	cest := time.FixedZone("CEST", 2*3600)
	if !Daylight(prg, time.Date(2026, 10, 2, 10, 0, 0, 0, cest)) || Daylight(prg, time.Date(2026, 10, 2, 22, 0, 0, 0, cest)) {
		t.Error("October day/night wrong")
	}
}
