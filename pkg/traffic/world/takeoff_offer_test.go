package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// runwayDep is a departureCtl at or near its runway: the runway next
// (RunwayNext) and the runway clearances given so far.
type runwayDep struct {
	standDep
	next            bool
	lineUp, takeoff bool
}

func (f *runwayDep) RunwayNext() bool { return f.next }

func (f *runwayDep) Cleared(action string) bool {
	switch action {
	case "lineup", "lineupbehind":
		return f.lineUp
	case "takeoff":
		return f.takeoff
	}
	return f.standDep.Cleared(action)
}

// TestTakeoffNotOfferedAgainLinedUp (#1569): a departure lining up or lined
// up that is cleared for take-off already is offered only "abort".
func TestTakeoffNotOfferedAgainLinedUp(t *testing.T) {
	for _, s := range []traffic.TaxiState{traffic.TaxiLiningUp, traffic.TaxiLinedUp} {
		if got := departureActions(s, "", &runwayDep{takeoff: true}); !slices.Equal(got, []string{"abort"}) {
			t.Errorf("%v, cleared for take-off: offered %v, want [abort]", s, got)
		}
		if got := departureActions(s, "", &runwayDep{lineUp: true}); !slices.Equal(got, []string{"takeoff", "abort"}) {
			t.Errorf("%v, told to line up only: offered %v, want [takeoff abort]", s, got)
		}
	}
}

// TestTakeoffOfferedRollingThrough (#1568): taxiing with its runway next and
// close, a departure may be cleared for take-off (or to line up behind)
// beside the line-up; once cleared for take-off nothing of the runway is
// offered again. Further out, none of them.
func TestTakeoffOfferedRollingThrough(t *testing.T) {
	got := departureActions(traffic.TaxiTaxiing, "", &runwayDep{next: true})
	for _, want := range []string{"lineup", "lineupbehind", "takeoff", "hold"} {
		if !slices.Contains(got, want) {
			t.Errorf("runway next: %q not offered, got %v", want, got)
		}
	}
	got = departureActions(traffic.TaxiTaxiing, "", &runwayDep{next: true, takeoff: true})
	for _, gone := range []string{"lineup", "lineupbehind", "takeoff"} {
		if slices.Contains(got, gone) {
			t.Errorf("cleared for take-off rolling through: %q offered again, got %v", gone, got)
		}
	}
	got = departureActions(traffic.TaxiTaxiing, "", &runwayDep{})
	for _, gone := range []string{"lineup", "lineupbehind", "takeoff"} {
		if slices.Contains(got, gone) {
			t.Errorf("runway not next: %q offered, got %v", gone, got)
		}
	}
}

// TestRollThroughOffer (#1568): the take-off and the line-up behind stay on
// a taxiing departure's offer only while the tower's holding-point checks
// pass for them.
func TestRollThroughOffer(t *testing.T) {
	all := []string{"hold", "taxi", "upto", "lineup", "lineupbehind", "takeoff"}
	it := &controlled{}
	if got := it.rollThroughOffer(slices.Clone(all)); slices.Contains(got, "takeoff") || slices.Contains(got, "lineupbehind") || !slices.Contains(got, "lineup") {
		t.Errorf("checks not passed: offered %v, want no take-off nor line-up behind", got)
	}
	it.rollTakeoff.Store(true)
	if got := it.rollThroughOffer(slices.Clone(all)); !slices.Contains(got, "takeoff") || slices.Contains(got, "lineupbehind") {
		t.Errorf("take-off checks passed: offered %v", got)
	}
	it.rollBehind.Store(true)
	if got := it.rollThroughOffer(slices.Clone(all)); !slices.Equal(got, all) {
		t.Errorf("both passed: offered %v, want %v", got, all)
	}
}
