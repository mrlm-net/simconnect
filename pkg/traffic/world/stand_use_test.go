package world

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Stands by use (#833) on LKPR as the scenery types them: an airliner at a
// gate (not the north GA ramps, live: TVS at N51–N58), a business jet on a
// GA ramp, a freighter on a cargo stand (E3–E7); an airliner never on a
// cargo stand, even with the gates full.
func TestAssignStandByUse(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	alloc := traffic.NewStandAllocator(nil, g)
	typeOf := func(s int) types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE { return g.Layout.Parking[s].Type }
	req := func(owner string) traffic.StandRequirements {
		return traffic.StandRequirements{Owner: owner, HalfSpan: 17.9, OffBlock: time.Now().Add(time.Hour)}
	}
	s, err := assignStand(alloc, req("TVS1"), standAirline)
	if err != nil || !g.Layout.Parking[s].IsGate() {
		t.Errorf("airliner: %s (%v), %v", g.Layout.Parking[s].Label(), typeOf(s), err)
	}
	biz := req("OKPDE")
	biz.HalfSpan = 8
	if s, err := assignStand(alloc, biz, standGA); err != nil || !containsType(gaRamps, typeOf(s)) {
		t.Errorf("business jet: %v, %v", typeOf(s), err)
	}
	if s, err := assignStand(alloc, req("DHL1"), standCargo); err != nil || typeOf(s) != types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO {
		t.Errorf("freighter: %v, %v", typeOf(s), err)
	}
	// Gates full: airliners go on, never to a cargo stand.
	for i := 0; i < 80; i++ {
		s, err := assignStand(alloc, req("AIR"+string(rune('A'+i%26))+string(rune('A'+i/26))), standAirline)
		if err != nil {
			break
		}
		if typeOf(s) == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO {
			t.Fatalf("airliner %d on cargo stand %s", i, g.Layout.Parking[s].Label())
		}
	}
}

func containsType(list []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE, t types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE) bool {
	for _, x := range list {
		if x == t {
			return true
		}
	}
	return false
}
