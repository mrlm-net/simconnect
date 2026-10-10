package nav

import (
	"slices"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestPlanChoices: the player's SID and approach are flown when they fit
// the runways; one for another runway is chosen as without it, with a
// note; the player's route is flown over the airways, what is not known
// noted.
func TestPlanChoices(t *testing.T) {
	g := loadLKPRAirways(t)
	lkpr := lkprInfo(t)
	sids := lkpr.Procedures.SIDsFor("24")
	if len(sids) < 2 {
		t.Skip("too few SIDs from 24")
	}
	want := sids[len(sids)-1].Name
	fp, err := Plan(FlightPlanRequest{Departure: lkpr, Arrival: eddm, Type: "A20N", DepartureRunway: "24", SID: want, CruiseFL: 240}, g)
	if err != nil {
		t.Fatal(err)
	}
	if fp.SID != want || fp.CruiseFL != 240 || len(fp.Notes) != 0 {
		t.Errorf("SID %q (want %s), FL%d, notes %v", fp.SID, want, fp.CruiseFL, fp.Notes)
	}
	other := ""
	for _, s := range lkpr.Procedures.SIDsFor("06") {
		if !slices.ContainsFunc(sids, func(x airport.Procedure) bool { return x.Name == s.Name }) {
			other = s.Name
			break
		}
	}
	if other != "" {
		fp, err = Plan(FlightPlanRequest{Departure: lkpr, Arrival: eddm, Type: "A20N", DepartureRunway: "24", SID: other}, g)
		if err != nil {
			t.Fatal(err)
		}
		if fp.SID == other || len(fp.Notes) != 1 || !strings.Contains(fp.Notes[0], "not one from runway 24") {
			t.Errorf("a SID for 06 from 24: SID %q, notes %v", fp.SID, fp.Notes)
		}
	}
	// The approach: RNAV 24 asked, ILS 06 for another runway.
	fp, err = Plan(FlightPlanRequest{Departure: eddm, Arrival: lkpr, Type: "A20N", ArrivalRunway: "24", Approach: "rnav 24"}, g)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Approach != "RNAV 24" {
		t.Errorf("approach %q, want RNAV 24 (notes %v)", fp.Approach, fp.Notes)
	}
	fp, err = Plan(FlightPlanRequest{Departure: eddm, Arrival: lkpr, Type: "A20N", ArrivalRunway: "24", Approach: "ILS 06"}, g)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Approach != "ILS 24" || len(fp.Notes) == 0 {
		t.Errorf("approach %q, notes %v", fp.Approach, fp.Notes)
	}
	// A route: two fixes of an airway, and one not known.
	var a, b, awy string
	for _, w := range g.Airways {
		if len(w.Segments) > 0 {
			a, b, awy = w.Segments[0].From.Ident, w.Segments[0].To.Ident, w.Name
			break
		}
	}
	fp, err = Plan(FlightPlanRequest{Departure: lkpr, Arrival: eddm, Type: "A20N", DepartureRunway: "24", Route: a + " " + awy + " " + b + " NOSUCHFIX"}, g)
	if err != nil {
		t.Fatal(err)
	}
	ids := planIdents(fp.Waypoints)
	if !strings.Contains(ids, a) || !strings.Contains(ids, b) {
		t.Errorf("route %s %s %s not flown: %s", a, awy, b, ids)
	}
	if !slices.ContainsFunc(fp.Notes, func(n string) bool { return strings.Contains(n, "NOSUCHFIX") }) {
		t.Errorf("unknown fix not noted: %v", fp.Notes)
	}
}
