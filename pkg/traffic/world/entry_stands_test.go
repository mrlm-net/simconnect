package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestStandBesideEntry (#1694): a stand moved onto the middle of a runway
// entry path stands beside it, for its own span with that of one its size
// passing; where it was, and a stand away from every entry, it does not.
func TestStandBesideEntry(t *testing.T) {
	cc, g0 := pairBed(t)
	paths := runwayEntryPaths(g0)
	if len(paths) == 0 {
		t.Fatal("no runway entry paths")
	}
	half := halfSpanOf(pairModel)
	var free []int
	for _, i := range g0.Layout.SuitableStands(half) {
		if !cc.besideEntry(g0, i, pairModel) {
			free = append(free, i)
		}
	}
	if len(free) < 2 {
		t.Fatalf("%d stands clear of the entries: the bed is no use", len(free))
	}
	k := free[0]
	l := *g0.Layout
	l.Parking = slices.Clone(l.Parking)
	mid := paths[0][len(paths[0])/2]
	l.Parking[k].Position = mid
	g, err := airport.BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	if !cc.besideEntry(g, k, pairModel) {
		t.Errorf("stand %s on a runway entry path: not beside it", l.Parking[k].Label())
	}
	if !slices.Contains(cc.entryStands(g, half, pairModel), k) {
		t.Errorf("stand %s on a runway entry path: not among the entry stands", l.Parking[k].Label())
	}
	if cc.besideEntry(g, free[1], pairModel) {
		t.Errorf("stand %s clear of every entry: beside one", l.Parking[free[1]].Label())
	}
}
