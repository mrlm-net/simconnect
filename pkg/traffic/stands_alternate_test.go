package traffic

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// alternateGraph is icao's layout from the simconnect-data clone with stand
// label drawn a second time apart meters behind itself on its centre line,
// at radius, as sceneries draw a medium and a heavy gate (TNCM A1–A4, #1676);
// nil (skipped) without the clone. It returns the graph and both stands.
func alternateGraph(t *testing.T, icao, label string, apart, radius float64) (*airport.Graph, int, int) {
	t.Helper()
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	b, err := os.ReadFile(filepath.Join(dir, icao, "layout.json"))
	if err != nil {
		t.Skipf("no %s layout (SIMCONNECT_DATA): skipped", icao)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	front := -1
	for _, p := range l.Parking {
		if p.Label() == label {
			front = p.Index
		}
	}
	if front < 0 {
		t.Fatalf("%s has no stand %s", icao, label)
	}
	p := raw.Parking[front]
	h := float64(p.Heading) * math.Pi / 180
	p.BiasX -= float32(math.Sin(h) * apart)
	p.BiasZ -= float32(math.Cos(h) * apart)
	p.Radius = float32(radius)
	raw.Parking = append(raw.Parking, p)
	if l, err = airport.BuildLayout(raw); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g, front, len(l.Parking) - 1
}

// TestStandAlternateNeverBothHeld: TNCM A1 and its alternate, a fuselage
// behind it on its centre line with the circles overlapping, are never held
// at once, though two A320s' wings clear each other there (#1676): the front
// one would push back into the one behind.
func TestStandAlternateNeverBothHeld(t *testing.T) {
	g, a1, alt := alternateGraph(t, "TNCM", "A1", 40.8, 22)
	l := g.Layout
	if l.Parking[alt].Label() != "A1" {
		t.Fatalf("alternate is %s", l.Parking[alt].Label())
	}
	d := localDist(l.Parking[a1].Position, l.Parking[alt].Position)
	if 2*DefaultHalfSpanMeters+StandWingtipClearanceMeters >= d {
		t.Fatalf("fixture: A320 wings meet %.1f m apart", d)
	}
	conflicts := func(i, j int) bool {
		for _, c := range l.ParkingConflicts(i) {
			if c == j {
				return true
			}
		}
		return false
	}
	if !conflicts(a1, alt) || !conflicts(alt, a1) {
		t.Fatal("fixture: A1 and its alternate do not overlap")
	}
	for _, order := range [][2]int{{a1, alt}, {alt, a1}} {
		a := NewStandAllocator(nil, g)
		if err := a.Occupy(order[0], "FIRST", DefaultHalfSpanMeters); err != nil {
			t.Fatal(err)
		}
		if a.Free(order[1], DefaultHalfSpanMeters) {
			t.Errorf("stand %d free with an A320 on %d", order[1], order[0])
		}
		if err := a.Occupy(order[1], "SECOND", DefaultHalfSpanMeters); !errors.Is(err, ErrStandTaken) {
			t.Errorf("occupy %d with an A320 on %d: %v", order[1], order[0], err)
		}
	}
	a := NewStandAllocator(nil, g)
	for i := 0; ; i++ {
		if _, err := a.Assign(StandRequirements{Owner: fmt.Sprintf("D%d", i), HalfSpan: DefaultHalfSpanMeters}); err != nil {
			break
		}
	}
	occ := a.Occupancy()
	_, front := occ[a1]
	_, back := occ[alt]
	if front && back {
		t.Error("Assign holds A1 and its alternate at once")
	}
	if !front && !back {
		t.Error("Assign holds neither A1 nor its alternate")
	}
}

// TestStandAlternateSynthetic: a stand and its alternate 40.8 m behind it
// on its centre line, the circles overlapping, are never held at once,
// though two A320s' wings clear each other there (#1676). No data needed.
func TestStandAlternateSynthetic(t *testing.T) {
	const metersPerDeg = 111320.0
	at := func(north float64) airport.LatLon { return airport.LatLon{Lat: 18 + north/metersPerDeg, Lon: -63} }
	l := &airport.Layout{ICAO: "ZZZZ", Latitude: 18, Longitude: -63, Parking: []airport.Parking{
		{Index: 0, Number: 1, Type: 9, Heading: 0, Radius: 22, Position: at(0)},
		{Index: 1, Number: 1, Type: 10, Heading: 0, Radius: 22, Position: at(-40.8)},
	}}
	if d := localDist(l.Parking[0].Position, l.Parking[1].Position); 2*DefaultHalfSpanMeters+StandWingtipClearanceMeters >= d {
		t.Fatalf("fixture: A320 wings meet %.1f m apart", d)
	}
	if c := l.ParkingConflicts(0); len(c) != 1 || c[0] != 1 {
		t.Fatalf("fixture: stand 0 conflicts %v, want [1]", c)
	}
	g := &airport.Graph{Layout: l}
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		a := NewStandAllocator(nil, g)
		if err := a.Occupy(order[0], "FIRST", DefaultHalfSpanMeters); err != nil {
			t.Fatal(err)
		}
		if a.Free(order[1], DefaultHalfSpanMeters) {
			t.Errorf("stand %d free with an A320 on %d", order[1], order[0])
		}
		if err := a.Occupy(order[1], "SECOND", DefaultHalfSpanMeters); !errors.Is(err, ErrStandTaken) {
			t.Errorf("occupy %d with an A320 on %d: %v", order[1], order[0], err)
		}
	}
	a := NewStandAllocator(nil, g)
	for i := 0; i < 3; i++ {
		a.Assign(StandRequirements{Owner: fmt.Sprintf("D%d", i), HalfSpan: DefaultHalfSpanMeters})
	}
	if occ := a.Occupancy(); len(occ) != 1 {
		t.Errorf("Assign holds %d of a stand and its alternate, want 1", len(occ))
	}
}
