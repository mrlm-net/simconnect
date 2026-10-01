//go:build windows
// +build windows

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
