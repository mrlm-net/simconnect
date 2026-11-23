package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// A flight plan's arrival keeps what the planner needs to fly a DME arc
// straight: which points are the arc's and where they are without it
// (#1646).
func TestPlannedFromKeepsDMEArcs(t *testing.T) {
	straight := airport.LatLon{Lat: 50.1, Lon: 14.1}
	p, err := plannedFrom(&nav.FlightPlan{Waypoints: []nav.Waypoint{
		{Ident: "START", Kind: "W", Phase: nav.PhaseApproach, Position: airport.LatLon{Lat: 50.2, Lon: 14.2}},
		{Phase: nav.PhaseApproach, Position: airport.LatLon{Lat: 50.15, Lon: 14.15}, DMEArc: true, Straight: straight},
		{Ident: "LEAD", Kind: "W", Phase: nav.PhaseApproach, Position: airport.LatLon{Lat: 50, Lon: 14}},
	}}, "arrival")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.route) != 3 || !p.route[1].DMEArc || p.route[1].Straight != straight {
		t.Fatalf("route %+v: the arc's point lost its mark", p.route)
	}
}
