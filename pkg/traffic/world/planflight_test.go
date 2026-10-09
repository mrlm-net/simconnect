package world

import (
	"context"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestPlanFlight: a plan between two loaded airports (LKPR, and LKTB
// without runways; direct without airways); without both
// codes, or without a connection for an airport not loaded, an error.
func TestPlanFlight(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	ctx := context.Background()
	if _, err := w.PlanFlight(ctx, PlanRequest{Departure: "LKPR"}); err == nil {
		t.Error("no arrival: planned")
	}
	if _, err := w.PlanFlight(ctx, PlanRequest{Departure: "LKPR", Arrival: "LKTB"}); err == nil {
		t.Error("no connection, nothing loaded: planned")
	}
	l := lkprLayout(t)
	w.st.cache.Put(l)
	w.st.cache.Put(&airport.Layout{ICAO: "LKTB", Latitude: 49.1513, Longitude: 16.6944, Altitude: 237}) // no runways: from and to the airport
	fp, err := w.PlanFlight(ctx, PlanRequest{Departure: "lkpr", Arrival: "LKTB", Type: "A320", DepartureRunway: "24"})
	if err != nil {
		t.Fatal(err)
	}
	if fp.DepartureRunway != "24" || fp.DistanceNM < 100 || fp.CruiseFL == 0 || len(fp.Waypoints) < 2 {
		t.Errorf("plan %s: runway %q, %.0f NM, FL%d", fp, fp.DepartureRunway, fp.DistanceNM, fp.CruiseFL)
	}
	if _, err := fp.PLN(); err != nil {
		t.Errorf("PLN: %v", err)
	}
}
