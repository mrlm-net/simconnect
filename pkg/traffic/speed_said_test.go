package traffic

import (
	"math"
	"strings"
	"testing"
)

// Speeds as controllers give them (Doc 4444 4.6.1.6): Mach in 0.01 at or
// above FL250, IAS in 10 kt below; the true airspeed flown matches what is
// said (live: OKEAE at FL360 told "reduce speed to 396 knots").
func TestSpeedSaidAt(t *testing.T) {
	s, tas := SpeedSaidAt(36087, 396)
	if s.Mach != 0.69 || s.IASKts != 0 {
		t.Errorf("396 kt TAS at FL360: %+v, want Mach 0.69", s)
	}
	if math.Abs(tas-0.69*soundKts(36087)) > 0.01 {
		t.Errorf("flown %.1f kt, not Mach 0.69", tas)
	}
	s, tas = SpeedSaidAt(10000, 300)
	if s.IASKts != 260 || math.Abs(IASAt(10000, tas)-260) > 0.01 {
		t.Errorf("300 kt TAS at 10000 ft: %+v (flown %.0f), want 260 kt IAS", s, tas)
	}
	if s, _ := SpeedSaidAt(3000, 183); s.IASKts != 180 {
		t.Errorf("183 kt TAS at 3000 ft: %+v, want 180 kt IAS", s)
	}
	r := Resolution{Callsign: "OKEAE", Kind: ResolveSpeed, Kts: 396, Said: SaidSpeed{Mach: 0.69}}
	tx := Resolved(PosCenter, r, 36087, 0, 440)
	if !strings.Contains(tx.Text, "reduce speed to Mach 0.69") || strings.Contains(tx.Text, "knots") {
		t.Errorf("said %q", tx.Text)
	}
	r.Said = SaidSpeed{IASKts: 250}
	if tx := Resolved(PosApproach, r, 9000, 0, 440); !strings.Contains(tx.Text, "reduce speed to 250 knots") {
		t.Errorf("said %q", tx.Text)
	}
}
