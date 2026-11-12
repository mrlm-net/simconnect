package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// An arrival on its way in at the start appears on its STAR by the way it
// still has to fly: between the STAR entry and the initial approach fix,
// never nearer (no straight-in) nor before the STAR (that is en route).
func TestMidwayAlong(t *testing.T) {
	fp := &nav.FlightPlan{DistanceNM: 300, Waypoints: []nav.Waypoint{
		{Ident: "LKPR", Phase: nav.PhaseEnroute, DistanceNM: 0},
		{Ident: "VOZ", Phase: nav.PhaseEnroute, DistanceNM: 200},
		{Ident: "VOZ", Phase: nav.PhaseSTAR, DistanceNM: 230},
		{Ident: "RAPET", Phase: nav.PhaseSTAR, DistanceNM: 260},
		{Ident: "PR530", Phase: nav.PhaseApproach, DistanceNM: 280, IAF: true},
		{Ident: "RW24", Phase: nav.PhaseApproach, DistanceNM: 300},
	}}
	for _, c := range []struct{ left, want float64 }{
		{55, 245},  // on the STAR
		{100, 230}, // further out than the STAR: at its entry
		{10, 280},  // nearer than the IAF: at the IAF
	} {
		if got, ok := midwayAlong(fp, c.left); !ok || got != c.want {
			t.Errorf("%.0f NM to fly: at %.0f NM along (%v), want %.0f", c.left, got, ok, c.want)
		}
	}
	fp.Waypoints[4].IAF = false
	if got, _ := midwayAlong(fp, 5); got != 300-midwayFinalNM {
		t.Errorf("no IAF, 5 NM to fly: at %.0f NM along, want %.0f", got, 300-midwayFinalNM)
	}
	if _, ok := midwayAlong(&nav.FlightPlan{DistanceNM: 100, Waypoints: []nav.Waypoint{{Phase: nav.PhaseEnroute}}}, 20); ok {
		t.Error("a plan without a STAR placed")
	}
}
