package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestNearRightAngleExitSpeed: a stub mapped a degree or two past a right
// angle (within airport.ExitAngleTolerance) is a 90° exit, taken exactly as
// one, so stubs at 89° and 91° on one runway behave alike (#1686).
func TestNearRightAngleExitSpeed(t *testing.T) {
	right := ExitSpeed(airport.RunwayExit{Angle: 90})
	for _, a := range []float64{88, 89, 91, 92, 90 + airport.ExitAngleTolerance} {
		if got := ExitSpeed(airport.RunwayExit{Angle: a}); got != right {
			t.Errorf("exit at %.0f°: %.0f kt, want %.0f as at 90°", a, got, right)
		}
	}
	if got := ExitSpeed(airport.RunwayExit{Angle: 30, HighSpeed: true}); got != ExitHighSpeedKts {
		t.Errorf("high-speed exit: %.0f kt, want %.0f", got, ExitHighSpeedKts)
	}
}
