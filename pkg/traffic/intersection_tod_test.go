package traffic

import "testing"

// An intersection leaves the type's take-off distance, by the elevation and
// a margin by the weather: an E190 at Prague (1250 ft) not from D with
// 2163 m left (live, KLM151); wet needs more than dry.
func TestIntersectionMinRemainingByType(t *testing.T) {
	dry := IntersectionMinRemaining("E190", 1247, RunwayDry)
	if dry <= 2163 {
		t.Errorf("E190 at 1247 ft dry: %.0f m, want more than D's 2163 m", dry)
	}
	if wet := IntersectionMinRemaining("E190", 1247, RunwayWet); wet <= dry {
		t.Errorf("wet %.0f m, want more than dry %.0f m", wet, dry)
	}
	if m := IntersectionMinRemaining("E190", 0, RunwayDry); m < 2050*1.1 {
		t.Errorf("E190 at sea level: %.0f m, want its 2050 m and a tenth", m)
	}
	if m, _ := IntersectionRule("DH8D"); IntersectionMinRemaining("DH8D", 1247, RunwayDry) < m {
		t.Error("below the class minimum")
	}
}
