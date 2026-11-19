package airport

import "testing"

// TestRouteToRunwayFullLength (#1562): with a full-length holding point
// (within FullLengthMeters of the threshold) among those in reach, the
// route to the runway ends there, not at an intersection a little further
// in that is a shorter taxi (TNCM 10: A at 63 m passed for B at 345 m;
// EGLL 27L: the hold 148 m in passed for the one 303 m in).
func TestRouteToRunwayFullLength(t *testing.T) {
	l := loadAirport(t, "EGLL")
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	rwy, end, ok := l.RunwayEnd("27L")
	if !ok {
		t.Fatal("no 27L")
	}
	along := func(id NodeID) float64 {
		h := g.Nodes[id].HoldShort
		if h == nil {
			return -1
		}
		if end.Name == rwy.Secondary.Name {
			return rwy.Length - h.FromPrimary
		}
		return h.FromPrimary
	}
	full := false
	for _, id := range g.HoldShortNodes(rwy.Index) {
		if a := along(id); a >= 0 && a <= FullLengthMeters {
			full = true
		}
	}
	if !full {
		t.Skip("no full-length hold for 27L in this data")
	}
	routed := 0
	for i := range l.Parking {
		r, err := g.RouteToRunway(i, "27L", RouteOptions{})
		if err != nil {
			continue
		}
		routed++
		if a := along(r.Nodes[len(r.Nodes)-1]); a > FullLengthMeters {
			t.Errorf("stand %s: hold %d %.0f m in, a full-length one is in reach", l.Parking[i].Label(), r.Nodes[len(r.Nodes)-1], a)
		}
	}
	if routed == 0 {
		t.Fatal("no stand reaches 27L")
	}
}
