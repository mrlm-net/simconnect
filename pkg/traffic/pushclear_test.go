package traffic

import "testing"

// A push keeps pushWingtipClearanceMeters from the aircraft that may park
// beside it, measured on their outlines, not the stands' circles (live
// LKPR: TVS139 from B9 for 06 ended with its wing over A7's stand line,
// 1.8 m from an A321 parked there).
func TestPushClearOfParkedNeighbours(t *testing.T) {
	t.Cleanup(standardPushes.Clear)
	g := airportGraph(t, "LKPR")
	for _, label := range []string{"B9", "B10", "A6", "C27"} {
		i, err := g.Layout.ParkingIndex(label)
		if err != nil {
			t.Fatal(err)
		}
		for _, rwy := range []string{"06", "24"} {
			ec := &eventClient{}
			ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1, StandOccupied: func(int) bool { return true }}); err != nil {
				t.Fatal(err)
			}
			pp, err := ctl.pushPath()
			if err != nil {
				t.Fatalf("%s for %s: %v", label, rwy, err)
			}
			pts, prof := pp.Points(), ctl.profile()
			if ctl.towPts != nil {
				pts = append(pts, towGear(ctl.towPts, prof)...)
			}
			pv := newFlatPave(pavement{}, pts[0])
			parked := pushOutlines(g, i, pv)
			want := min(pushWingtipClearanceMeters, outlineClearance(pv, parked, pts[0], g.Layout.Parking[i].Heading, prof)-0.5)
			if got := pushClearance(pv, parked, pts, prof); got < want-0.1 {
				t.Errorf("%s for %s: %.1f m from an aircraft parked beside it, want %.1f", label, rwy, got, want)
			}
		}
	}
}
