package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Departures in trail on one route (live, RYR1427 and TVS1620 both at
// FL240 on BALTU, closing): the second identified within
// departureTrailWithin is cleared 2000 ft lower, the third 2000 ft lower
// again; one on another route keeps the level.
func TestLevelInTrail(t *testing.T) {
	cc := &controlCenter{clock: traffic.NewSimClock(), items: map[int]*controlled{}}
	dep := func(id int, tail, route string) *controlled {
		it := &controlled{cc: cc, ICAO: "LKPR", Tail: tail, dep: (*traffic.TaxiController)(nil), radarFt: departureClimbFt, radarSaid: departureClimbSaid}
		it.view.Procedure = route
		cc.items[id] = it
		return it
	}
	first := dep(1, "RYR1427", "BALT4B")
	first.identified, first.identifiedAt = true, cc.clock.Now()
	second := dep(2, "TVS1620", "BALT4B")
	second.levelInTrail()
	if second.radarFt != departureClimbFt-2000 {
		t.Fatalf("second on BALT4B: %.0f ft, want %.0f", second.radarFt, departureClimbFt-2000)
	}
	second.identified, second.identifiedAt = true, cc.clock.Now()
	third := dep(3, "CSA1", "BALT4B")
	third.levelInTrail()
	if third.radarFt != departureClimbFt-4000 {
		t.Fatalf("third on BALT4B: %.0f ft, want %.0f", third.radarFt, departureClimbFt-4000)
	}
	other := dep(4, "DLH1", "VOZ4B")
	other.levelInTrail()
	if other.radarFt != departureClimbFt {
		t.Fatalf("on another route: %.0f ft, want %.0f", other.radarFt, departureClimbFt)
	}
}
