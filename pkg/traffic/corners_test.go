//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A 90° corner at 180 kt becomes an arc of about 1 NM radius (25° of
// bank): its points are that far from the centre, no heading change
// between them exceeds 45°, and the legs are kept straight up to it.
func TestRoundCorners(t *testing.T) {
	wp := func(lat, lon float64) types.SIMCONNECT_DATA_WAYPOINT {
		return types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 4000, KtsSpeed: 180}
	}
	a := wp(50, 14)
	cLat, cLon := calc.DisplaceByHeading(50, 14, 90, 10*1852)
	bLat, bLon := calc.DisplaceByHeading(cLat, cLon, 180, 10*1852)
	got := roundCorners([]types.SIMCONNECT_DATA_WAYPOINT{a, wp(cLat, cLon), wp(bLat, bLon)}, TurnBankDeg)
	if len(got) != 5 || got[0] != a || got[len(got)-1].Latitude != bLat {
		t.Fatalf("%d points", len(got))
	}
	r := turnRadiusMeters(180, StandardBankDeg(180, TurnBankDeg))
	if r < 1600 || r > 2100 {
		t.Errorf("radius %.0f m at 180 kt, want about 1 NM", r)
	}
	// The arc's centre is r south of its first point and r west of its last.
	centreLat, centreLon := calc.DisplaceByHeading(got[1].Latitude, got[1].Longitude, 180, r)
	for _, p := range got[1:4] {
		if d := calc.HaversineMeters(centreLat, centreLon, p.Latitude, p.Longitude); math.Abs(d-r) > 30 {
			t.Errorf("arc point %.0f m from the centre, want %.0f", d, r)
		}
	}
	for i := 1; i+1 < len(got); i++ {
		h1 := calc.BearingDegrees(got[i-1].Latitude, got[i-1].Longitude, got[i].Latitude, got[i].Longitude)
		h2 := calc.BearingDegrees(got[i].Latitude, got[i].Longitude, got[i+1].Latitude, got[i+1].Longitude)
		if d := math.Abs(math.Mod(h2-h1+540, 360) - 180); d > 46 {
			t.Errorf("a %.0f° turn at point %d", d, i)
		}
	}
	// Gentle corners and short legs are kept as they are.
	dLat, dLon := calc.DisplaceByHeading(cLat, cLon, 100, 10*1852)
	if got := roundCorners([]types.SIMCONNECT_DATA_WAYPOINT{a, wp(cLat, cLon), wp(dLat, dLon)}, TurnBankDeg); len(got) != 3 {
		t.Errorf("a 10° corner: %d points", len(got))
	}
}

// The standard turn: rate one up to the airframe's most bank — a jet at
// 250 kt banks 25°, a turboprop at 250 kt 30°, both at 140 kt 21°.
func TestStandardBank(t *testing.T) {
	jet, tprop := MaxBankDeg(ProfileFor("A320")), MaxBankDeg(ProfileFor("AT76"))
	for _, c := range []struct{ kts, max, want float64 }{{250, jet, 25}, {250, tprop, 30}, {140, jet, 21}, {140, tprop, 21}, {180, jet, 25}} {
		if got := StandardBankDeg(c.kts, c.max); math.Abs(got-c.want) > 0.01 {
			t.Errorf("%.0f kt, at most %.0f°: %.1f°, want %.0f°", c.kts, c.max, got, c.want)
		}
	}
}
