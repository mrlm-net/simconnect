package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A trailer is stopped where it is, above the one ahead, only while that
// one descends away; never under it, never behind one level on its
// intercept (live, LKPR: DLH299 at 4800 ft told 4500 behind AFR971 level at
// 4600 ft, 2.0 NM and 235 ft apart).
func TestHoldAboveDescending(t *testing.T) {
	ac := func(alt, vs float64) traffic.TrackedAircraft {
		var a traffic.TrackedAircraft
		a.AltFt, a.VSFpm = alt, vs
		return a
	}
	if h, ok := holdAboveDescending(ac(4802, -200), ac(4640, 0)); ok {
		t.Errorf("behind one level at 4640 ft: held at %.0f ft, want no hold (slowed and vectored)", h)
	}
	h, ok := holdAboveDescending(ac(4802, -800), ac(4640, -900))
	if !ok || h < 4640 {
		t.Errorf("behind one descending: %.0f ft %v, want held at or above 4640", h, ok)
	}
	if h, ok := holdAboveDescending(ac(4500, -800), ac(4640, -900)); ok && h < 4640 {
		t.Errorf("just under the one ahead: held at %.0f ft, under it", h)
	}
}
