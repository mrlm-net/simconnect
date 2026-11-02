package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

type occupiesCounter struct {
	departureCtl
	state traffic.TaxiState
	asked int
}

func (f *occupiesCounter) State() traffic.TaxiState { return f.state }
func (f *occupiesCounter) Occupies() (airport.Occupied, bool) {
	f.asked++
	return airport.Occupied{HalfSpan: 17}, true
}

// TestOccupiedAsksOnlyWhereItCan: over a link Occupies is a round trip, and
// outside these states the controller answers false anyway.
func TestOccupiedAsksOnlyWhereItCan(t *testing.T) {
	for state, want := range map[traffic.TaxiState]bool{
		traffic.TaxiAwaitingPushback: true,
		traffic.TaxiPushback:         true,
		traffic.TaxiAwaitingTaxi:     true,
		traffic.TaxiTaxiing:          false,
		traffic.TaxiHoldingShort:     false,
	} {
		f := &occupiesCounter{state: state}
		o, ok := occupiedBy(f)
		if ok != want || (f.asked == 1) != want {
			t.Errorf("%v: ok %v, asked %d times, want %v", state, ok, f.asked, want)
		}
		if want && o.HalfSpan != 17 {
			t.Errorf("%v: the answer got lost: %+v", state, o)
		}
	}
}
