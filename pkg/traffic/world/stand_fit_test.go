package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestStandFitsByUse: the schedule's spawn check (standFits) asks what
// assignStand assigns by, the stand types of the flight's use and the
// model's half span: a passenger B738 does not fit a field whose only
// stand wide enough is a cargo ramp, a freighter does, a B744 fits
// neither, and a model with its livery is judged by its title.
func TestStandFitsByUse(t *testing.T) {
	l := &airport.Layout{ICAO: "ZZZZ", Parking: []airport.Parking{
		{Index: 0, Radius: 15, Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_SMALL},
		{Index: 1, Radius: 28, Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_CARGO},
		{Index: 2, Radius: 25, Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_MIL_COMBAT},
	}}
	for _, c := range []struct {
		model, use string
		want       bool
	}{
		{"B738", standAirline, false},
		{"B738", standCargo, true},
		{"FSLTL_B738_KLM" + liverySep + "KLM", standAirline, false},
		{"FSLTL_B738_KLM" + liverySep + "KLM", standCargo, true},
		{"B744", standCargo, false},
		{"C172", standGA, true},
	} {
		if got := standFits(l, c.model, c.use); got != c.want {
			t.Errorf("standFits(%s, %q) = %v, want %v", c.model, c.use, got, c.want)
		}
	}
	// What it says is what the allocator then does.
	g := &airport.Graph{Layout: l}
	alloc := traffic.NewStandAllocator(nil, g)
	if _, err := assignStand(alloc, traffic.StandRequirements{Owner: "KLM1", HalfSpan: traffic.StandHalfSpan("B738")}, standAirline); err == nil {
		t.Error("a passenger B738 given a stand standFits says it has not")
	}
	if !standFreeFor(alloc, l, "B738", standCargo) {
		t.Error("the cargo ramp is free")
	}
	if _, err := assignStand(alloc, traffic.StandRequirements{Owner: "UPS1", HalfSpan: traffic.StandHalfSpan("B738")}, standCargo); err != nil {
		t.Fatalf("a cargo B738 refused the stand standFits found: %v", err)
	}
	if standFreeFor(alloc, l, "B738", standCargo) {
		t.Error("the cargo ramp is taken: none free")
	}
	alloc.ReleaseOwner("UPS1")
	if !standFreeFor(alloc, l, "B738", standCargo) {
		t.Error("released at once: free again")
	}
}
