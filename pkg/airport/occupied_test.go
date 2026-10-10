package airport

import (
	"slices"
	"testing"
)

// A B738 pushed back from LKPR C19 onto JB: a CRJ taxiing in along J to C18
// cannot pass beside it and goes by JO; a wide-body, kept off JO by its span
// limit, has no way round (Route.Occupied).
func TestRouteAvoidsOccupied(t *testing.T) {
	l := loadLKPR(t)
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	c18, err := l.ParkingIndex("C18")
	if err != nil {
		t.Fatal(err)
	}
	c19, err := l.ParkingIndex("C19")
	if err != nil {
		t.Fatal(err)
	}
	pushed := Occupied{Points: []LatLon{l.Parking[c19].Position, g.Nodes[980].Position, g.Nodes[981].Position}, HalfSpan: 17.9}
	crj := RouteOptions{HalfSpan: 11.6}

	plain, err := g.RouteToParkingFrom(786, 787, c18, crj)
	if err != nil {
		t.Fatal(err)
	}
	if !g.PassesOccupied(plain, 0, RouteOptions{HalfSpan: 11.6, Occupied: []Occupied{pushed}}) {
		t.Fatalf("plain route %v does not pass the pushed aircraft", plain.Taxiways)
	}

	crj.Occupied = []Occupied{pushed}
	r, err := g.RouteToParkingFrom(786, 787, c18, crj)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CRJ: %v (plain %v)", r.Taxiways, plain.Taxiways)
	if r.Occupied || !slices.Contains(r.Taxiways, "JO") || g.PassesOccupied(r, 0, crj) {
		t.Errorf("CRJ route %v (occupied %v), want one by JO clear of the pushed aircraft", r.Taxiways, r.Occupied)
	}

	wide := RouteOptions{HalfSpan: 30, Occupied: []Occupied{pushed}}
	r, err = g.RouteToParkingFrom(786, 787, c18, wide)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Occupied || slices.Contains(r.Taxiways, "JO") {
		t.Errorf("wide-body route %v (occupied %v), want the plain one marked Occupied", r.Taxiways, r.Occupied)
	}
}

// TestTurnsBack: a route that goes out along an edge and back is turning
// back; the plain taxi-in is not.
func TestTurnsBack(t *testing.T) {
	l := loadLKPR(t)
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	c18, err := l.ParkingIndex("C18")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := g.RouteToParkingFrom(786, 787, c18, RouteOptions{HalfSpan: 11.6})
	if err != nil {
		t.Fatal(err)
	}
	if g.TurnsBack(plain) {
		t.Error("plain taxi-in counted as turning back")
	}
	back := &Route{Nodes: []NodeID{787, 786, 787}}
	if !g.TurnsBack(back) {
		t.Error("out and back not counted as turning back")
	}
}
