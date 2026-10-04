package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// In a crosswind from the right the aircraft crabs right on final, by the
// drift angle, and touches down aligned with the runway.
func TestApproachCrosswind(t *testing.T) {
	m := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 5000, DefaultApproachProfile())
	m.SetCrosswind(15)
	p := m.Pose()
	crab := headingDiff(244, p.Heading)
	if want := math.Asin(15/p.GroundSpeedKts) * 180 / math.Pi; math.Abs(crab-want) > 0.5 {
		t.Errorf("crab %.1f°, want %.1f°", crab, want)
	}
	for i := 0; i < 20*600 && !p.OnGround; i++ {
		p = m.Step(0.05)
	}
	if !p.OnGround || math.Abs(headingDiff(244, p.Heading)) > 0.01 {
		t.Errorf("touchdown heading %.2f (on ground %v), want 244", p.Heading, p.OnGround)
	}
	calm := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 5000, DefaultApproachProfile()).Pose()
	if calm.Heading != 244 {
		t.Errorf("calm heading %.2f", calm.Heading)
	}
}

// In the flare the upwind wing goes down as the crab comes out, level
// again after touchdown; the aiming point moves the touchdown by as much.
func TestApproachWingLowAndAim(t *testing.T) {
	m := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 3000, DefaultApproachProfile())
	m.SetCrosswind(12) // from the right
	maxBank, p := 0.0, m.Pose()
	if p.BankDeg != 0 {
		t.Errorf("bank on final %.1f°, want wings level (crabbed)", p.BankDeg)
	}
	for i := 0; i < 20*600 && p.Phase != ApproachDone; i++ {
		p = m.Step(0.05)
		maxBank = math.Max(maxBank, p.BankDeg)
	}
	if maxBank < 2 || maxBank > maxWingLowDeg {
		t.Errorf("wing low %.1f°, want right wing down 2–%.0f°", maxBank, maxWingLowDeg)
	}
	if p.BankDeg != 0 {
		t.Errorf("bank %.1f° after the nose came down", p.BankDeg)
	}
	base := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 3000, DefaultApproachProfile())
	shifted := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, 3000, DefaultApproachProfile())
	shifted.SetAimShift(-8)
	a, b := base.Pose(), shifted.Pose()
	for i := 0; i < 20*600 && !(a.OnGround && b.OnGround); i++ {
		a, b = base.Step(0.05), shifted.Step(0.05)
	}
	if d := b.Touchdown - a.Touchdown; math.Abs(d+8) > 0.01 {
		t.Errorf("touchdown moved %.2f m, want -8", d)
	}
}
