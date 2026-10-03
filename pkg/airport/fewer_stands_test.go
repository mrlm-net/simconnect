//go:build windows
// +build windows

package airport

import (
	"math"
	"testing"
)

// Of routes about as long, the one past fewer stands: LKPR N51 to 06 went
// G, L, D, F past nine stands; G, JO, H, F passes none for 186 m more.
func TestRouteFewerStands(t *testing.T) {
	l := loadLKPR(t)
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	n51, err := l.ParkingIndex("N51")
	if err != nil {
		t.Fatal(err)
	}
	fewerStandsOff = true
	plain, err := g.RouteToRunway(n51, "06", RouteOptions{})
	fewerStandsOff = false
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.RouteToRunway(n51, "06", RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	before, after := g.StandsPassed(plain, RouteOptions{}), g.StandsPassed(r, RouteOptions{})
	if after >= before {
		t.Errorf("past %d stands (%v), the plain route %d (%v): want fewer", after, r.Taxiways, before, plain.Taxiways)
	}
	if extra := r.Length - plain.Length; extra > math.Max(FewerStandsTolerance*plain.Length, FewerStandsMinMeters) {
		t.Errorf("%.0f m longer, beyond the tolerance", extra)
	}
	// A custom route is flown as asked.
	via, err := g.RouteToRunway(n51, "06", RouteOptions{Taxiways: plain.Taxiways})
	if err != nil {
		t.Fatal(err)
	}
	if g.StandsPassed(via, RouteOptions{}) != before {
		t.Errorf("custom route %v changed to pass fewer stands", via.Taxiways)
	}
}
