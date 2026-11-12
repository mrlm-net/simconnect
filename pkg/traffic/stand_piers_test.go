package traffic

import (
	"fmt"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// LKPR's gates fill over all three piers, not C first with A and B empty
// (the user, live 2026-11-12: "most on C, B and A empty"), for either
// runway direction, with nearly every flight Schengen.
func TestStandsSpreadOverPiers(t *testing.T) {
	g := lkprGraph(t)
	gates := []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_SMALL, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_EXTRA,
	}
	for _, rwy := range []string{"24", "06", "30", "12"} {
		a := NewStandAllocator(&eventClient{}, g)
		count := map[string]int{}
		first := map[string]int{}
		for k := range 14 {
			other := "LFPG"
			if k%6 == 5 {
				other = "EGLL"
			}
			s, err := a.Assign(StandRequirements{Owner: fmt.Sprint("F", k), HalfSpan: 17.9, Runway: rwy, Other: other, Types: gates})
			if err != nil {
				break
			}
			p := g.Layout.Parking[s].Label()[:1]
			count[p]++
			if k < 8 {
				first[p]++
			}
		}
		t.Logf("runway %s: %v (first 8: %v)", rwy, count, first)
		if first["A"]+first["B"] < 2 || first["C"] < 2 {
			t.Errorf("runway %s: first 8 gates %v, want every terminal used", rwy, first)
		}
	}
}
