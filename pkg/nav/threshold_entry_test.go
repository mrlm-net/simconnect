package nav

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestParallelsThresholdEntry: LROP's parallels 1261 m apart. West wind:
// 26L has no taxiway at its east threshold (a departure would backtrack),
// so take-offs go to 26R and landings to 26L. East wind: both 08 ends
// have one, the parallels stay independent.
func TestParallelsThresholdEntry(t *testing.T) {
	b, err := os.ReadFile("../airport/testdata/LROP-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l airport.Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	lim := RunwayLimits{ThresholdEntry: g.ThresholdEntry}
	west := ActiveRunways(&l, Weather{WindDirTrue: 250, WindKts: 5}, lim)
	if west.Parallel != ParallelSegregated || !slices.Equal(Names(west.Departures), []string{"26R"}) || !slices.Equal(Names(west.Arrivals), []string{"26L"}) {
		t.Errorf("west wind: %s dep %v arr %v", west.Parallel, Names(west.Departures), Names(west.Arrivals))
	}
	east := ActiveRunways(&l, Weather{WindDirTrue: 70, WindKts: 5}, lim)
	if east.Parallel != ParallelIndependent || len(east.Departures) != 2 {
		t.Errorf("east wind: %s dep %v arr %v", east.Parallel, Names(east.Departures), Names(east.Arrivals))
	}
	plain := ActiveRunways(&l, Weather{WindDirTrue: 250, WindKts: 5}, RunwayLimits{})
	if plain.Parallel != ParallelIndependent {
		t.Errorf("without entries known: %s", plain.Parallel)
	}
}
