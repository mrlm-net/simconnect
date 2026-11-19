package worldtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// traffic is the run's overflight and transit settings (#1483–#1487).
func (r overflightRun) traffic() world.OverflightTraffic {
	var o world.OverflightTraffic
	on := true
	if r.flows {
		o.OverflightFlows = &on
	}
	if r.routes {
		o.OverflightRoutes = &on
	}
	if r.transits {
		o.Transits = &on
	}
	if r.keepClear {
		o.CrossingKeepClear = &on
	}
	if r.radio {
		o.OverflightRadio = &on
	}
	return o
}

// rateOf is the peak rate the run asked for: with the flows, none (the
// area's volume) unless set; else the World's (GET /api/schedule).
func rateOf(perHour float64, r overflightRun) float64 {
	if r.flows {
		return r.perHour
	}
	return perHour
}

// TestOverflightFlowsBed runs the test bed with the flows on (#1483): the
// hours planned follow the area's streams (LKPR, EGLL from the table, LZKZ
// the generic fallback), the quiet airport far fewer than the busy ones;
// no conflict-watch firing between two overflights, no jump.
func TestOverflightFlowsBed(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World for hours of traffic time")
	}
	root := dataDir(t)
	planned := map[string]int{}
	for _, icao := range overflightAirports {
		t.Run(icao, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(root, icao, "layout.json")); err != nil {
				t.Skipf("no offline data for %s in %s", icao, root)
			}
			m := runOverflights(t, root, icao, overflightHours(), overflightRun{flows: true, materialiseNM: 40, pairsMeet: true})
			m.report(t)
			m.check(t)
			m.tiers.check(t, m, false)
			if m.confOO > 0 {
				t.Errorf("%s: %d conflict-watch firings between two overflights: %v", icao, m.confOO, m.conflicts)
			}
			planned[icao] = len(m.planned)
		})
	}
	if p, q := planned["LKPR"], planned["LZKZ"]; p > 0 && q > 0 && q*2 > p {
		t.Errorf("LZKZ (the fallback) planned %d, LKPR %d: want far fewer", q, p)
	}
}
