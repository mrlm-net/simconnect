//go:build windows
// +build windows

package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTaxiControllerAirportHandover: the injected take-off hands over at
// the airport's height; LKPR's SIDs climb to 1700 ft only, so the floor
// applies; an airport whose SID climbs higher hands over higher.
func TestTaxiControllerAirportHandover(t *testing.T) {
	procs := lkprProcedures(t)
	lim := airport.LimitsFor(lkprGraph(t).Layout, &procs)
	if lim.ClimbHandoverFt != airport.MinClimbHandoverFt {
		t.Fatalf("LKPR hand-over %.0f ft, want the %.0f ft floor", lim.ClimbHandoverFt, airport.MinClimbHandoverFt)
	}
	lim.ClimbHandoverFt = 2500 // as a SID climbing to about 3700 ft would give
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1, Airport: &lim})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiComplete, 60*1500) {
		t.Fatalf("state %v, want complete", ctl.State())
	}
	if h := ctl.last.HeightFt; h < lim.ClimbHandoverFt || h > lim.ClimbHandoverFt+100 {
		t.Errorf("handed over at %.0f ft, want %.0f", h, lim.ClimbHandoverFt)
	}
}

// TestTaxiControllerAirportTaxiSpeeds: the airport's TaxiMaxKts caps the
// profile's taxi speed and ApronMaxKts the apron taxilanes.
func TestTaxiControllerAirportTaxiSpeeds(t *testing.T) {
	lim := airport.Limits{TaxiMaxKts: 12, ApronMaxKts: 6}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1, Airport: &lim})
	go func() {
		for range ctl.Events() {
		}
	}()
	if got := ctl.profile().CruiseKts; got != lim.TaxiMaxKts {
		t.Errorf("taxi speed %.1f kt, want %.0f", got, lim.TaxiMaxKts)
	}
	if !run(TaxiTaxiing, 60*600) {
		t.Fatalf("state %v, want taxiing", ctl.State())
	}
	g, route, path := ctl.req.Graph, ctl.Route(), ctl.mover.Path()
	checked := 0
	for i := ctl.pushJunction + 1; i+1 < len(route.Nodes); i++ {
		if !g.Apron(route.Nodes[i]) && !g.Apron(route.Nodes[i+1]) {
			continue
		}
		a, b := route.Points[i], route.Points[i+1]
		along, off := path.DistanceTo(airport.LatLon{Lat: (a.Lat + b.Lat) / 2, Lon: (a.Lon + b.Lon) / 2})
		if off > 5 {
			continue
		}
		checked++
		if v := path.SpeedLimitKts(along); v > lim.ApronMaxKts+0.01 {
			t.Errorf("apron %d-%d: limit %.1f kt, want at most %.0f", route.Nodes[i], route.Nodes[i+1], v, lim.ApronMaxKts)
		}
	}
	if checked == 0 {
		t.Error("no apron stretch on the taxi-out from C22")
	}
}
