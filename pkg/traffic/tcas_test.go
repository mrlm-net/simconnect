package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The sensitivity levels by altitude (booklet Table 2).
func TestTCASSensitivity(t *testing.T) {
	for _, c := range []struct {
		alt, agl float64
		sl       int
		raTau    float64
	}{{500, 500, 2, 0}, {2000, 1500, 3, 15}, {4000, 3000, 4, 20}, {8000, 7000, 5, 25}, {15000, 14000, 6, 30}, {35000, 35000, 7, 35}, {45000, 45000, 7, 35}} {
		if lv := TCASSensitivity(c.alt, c.agl); lv.SL != c.sl || lv.RATau != c.raTau {
			t.Errorf("%.0f ft: SL%d RA tau %.0f, want SL%d %.0f", c.alt, lv.SL, lv.RATau, c.sl, c.raTau)
		}
	}
}

// tcasAt puts an aircraft nm east of a point, flying track at kts.
func tcasAt(nmEast, alt, kts, trk, vs float64) TCASTrack {
	lat, lon := calc.DisplaceByHeading(50, 14, 90, nmEast*1852)
	return TCASTrack{Lat: lat, Lon: lon, AltFt: alt, AGLFt: alt - 1000, GroundKts: kts, TrackDeg: trk, VSFpm: vs}
}

// Head-on at FL100 and FL103: far out nothing, then a TA, then an RA, a
// non-crossing descend (the intruder 300 ft above).
func TestTCASHeadOn(t *testing.T) {
	own := tcasAt(0, 10000, 300, 90, 0)
	far := tcasAt(20, 10300, 300, 270, 0)
	if adv, _, _ := Evaluate(own, far); adv != AdvisoryNone {
		t.Errorf("20 NM: %v", adv)
	}
	ta := tcasAt(6, 10300, 300, 270, 0) // 6 NM at 600 kt closing: 36 s
	if adv, lv, _ := Evaluate(own, ta); adv != AdvisoryTA {
		t.Errorf("6 NM: %v (SL%d)", adv, lv.SL)
	}
	ra := tcasAt(3.5, 10300, 300, 270, 0) // 21 s
	adv, lv, _ := Evaluate(own, ra)
	if adv != AdvisoryRA {
		t.Fatalf("3.5 NM: %v (SL%d)", adv, lv.SL)
	}
	r := SelectRA(own, ra, lv, 0)
	if r.Sense != -1 || r.Crossing {
		t.Errorf("intruder 300 ft above: %+v, want a non-crossing descend", r)
	}
	// Coordinated: the intruder chose down, own gets up.
	if r := SelectRA(own, ra, lv, 1); r.Sense != 1 {
		t.Errorf("forced up: %+v", r)
	}
}

// Low: no RA below 1000 ft AGL, no descend RA below 1100 ft AGL.
func TestTCASInhibits(t *testing.T) {
	own := tcasAt(0, 1900, 160, 90, 0)
	own.AGLFt = 900
	in := tcasAt(1, 1950, 160, 270, 0)
	if adv, _, _ := Evaluate(own, in); adv == AdvisoryRA {
		t.Error("an RA below 1000 ft AGL")
	}
	own.AGLFt = 1050
	in.AltFt = 2100 // above: the down sense would not cross, but is inhibited
	lv := TCASSensitivity(own.AltFt, own.AGLFt)
	if r := SelectRA(own, in, lv, 0); r.Sense != 1 {
		t.Errorf("descend below 1100 ft AGL: %+v", r)
	}
}

// Already safe: a preventive monitor; descending into it with room to
// level: level off.
func TestTCASStrength(t *testing.T) {
	lv := TCASSensitivity(8000, 7000)
	own := tcasAt(0, 8000, 250, 90, 0)
	in := tcasAt(2, 8650, 250, 270, 0) // 650 ft above, level: ALIM 350 kept
	if r := SelectRA(own, in, lv, 0); r.Kind != RAMonitor {
		t.Errorf("level, 650 ft below: %+v", r)
	}
	own.VSFpm = 1000 // climbing into it
	in.AltFt = 9000
	if r := SelectRA(own, in, lv, 0); r.Kind != RALevelOff && r.Sense != -1 {
		t.Errorf("climbing into it: %+v", r)
	}
}

// A coordinated descend below 1100 ft AGL is a level off, not a descent
// (#103).
func TestTCASForcedDescendLow(t *testing.T) {
	own := tcasAt(0, 2000, 160, 90, 0)
	own.AGLFt = 1050
	in := tcasAt(1, 2100, 160, 270, 0)
	lv := TCASSensitivity(own.AltFt, own.AGLFt)
	if r := SelectRA(own, in, lv, -1); r.Kind != RALevelOff || r.TargetFpm < 0 {
		t.Errorf("forced down low: %+v", r)
	}
}
