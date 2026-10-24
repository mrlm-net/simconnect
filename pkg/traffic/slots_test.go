package traffic

import (
	"fmt"
	"testing"
	"time"
)

// TestCoordinateSlots: eight arrivals due in one bin are spread over the
// bins after it, three to a bin, their departures moved with them; a VFR
// flight keeps its time.
func TestCoordinateSlots(t *testing.T) {
	var fs []Flight
	for i := 0; i < 8; i++ {
		fs = append(fs, flight(fmt.Sprintf("CSA%d", i), "EDDF", "LKPR", t0, time.Hour))
	}
	vfr := flight("OKABC", "LKPR", "LKPR", t0, time.Hour)
	vfr.Rules = "VFR"
	out := CoordinateSlots(append(fs, vfr), []string{"LKPR"}, SlotCapacity{})
	if len(out) != 9 {
		t.Fatalf("%d flights out of 9", len(out))
	}
	bins := map[time.Time]int{}
	for _, f := range out {
		if f.Rules == "VFR" {
			if !f.STD.Equal(t0) {
				t.Errorf("VFR flight moved to %v", f.STD)
			}
			continue
		}
		if f.STA.Sub(f.STD) != time.Hour {
			t.Errorf("%s block time %v, want its hour kept", f.Callsign, f.STA.Sub(f.STD))
		}
		bins[f.STA.Truncate(10*time.Minute)]++
	}
	for b, n := range bins {
		if n > 3 {
			t.Errorf("%d arrivals in the bin of %s", n, b.Format("15:04"))
		}
	}
	if len(bins) != 3 {
		t.Errorf("arrivals in %d bins, want 3 (3, 3, 2)", len(bins))
	}
}
