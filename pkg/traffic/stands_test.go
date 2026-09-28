//go:build windows
// +build windows

package traffic

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// scanClient records the by-type requests a StandAllocator sends.
type scanClient struct {
	fakeClient
	byType []types.SIMCONNECT_SIMOBJECT_TYPE
	radius []uint32
}

func (c *scanClient) RequestDataOnSimObjectType(_, _, radius uint32, t types.SIMCONNECT_SIMOBJECT_TYPE) error {
	c.byType, c.radius = append(c.byType, t), append(c.radius, radius)
	return nil
}

// byTypeMsg is one aircraft of a scan answer (entry of outOf, from 1).
func byTypeMsg(req, obj, entry, outOf uint32, d standScanData) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(d)))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	h.DwEntryNumber, h.DwOutOf = types.DWORD(entry), types.DWORD(outOf)
	*(*standScanData)(unsafe.Pointer(&buf[off])) = d
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func stand(t *testing.T, g *airport.Graph, label string) int {
	t.Helper()
	i, err := g.Layout.ParkingIndex(label)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestStandAllocatorOccupy(t *testing.T) {
	g := lkprGraph(t)
	a := NewStandAllocator(nil, g)
	c22 := stand(t, g, "C22")
	if err := a.Occupy(c22, "CSA1", 0); err != nil {
		t.Fatal(err)
	}
	if err := a.Occupy(c22, "DLH2", 0); !errors.Is(err, ErrStandTaken) {
		t.Errorf("second owner: %v, want ErrStandTaken", err)
	}
	if err := a.Occupy(c22, "CSA1", 30); err != nil { // the same owner updates
		t.Errorf("same owner: %v", err)
	}
	if o, ok := a.Occupant(c22); !ok || o.Owner != "CSA1" || o.HalfSpan != 30 {
		t.Errorf("occupant %+v %v", o, ok)
	}
	if a.Free(c22, 10) {
		t.Error("occupied stand is free")
	}
	a.Release(c22)
	if !a.Free(c22, 10) {
		t.Error("released stand is not free")
	}
	if err := a.Occupy(len(g.Layout.Parking), "X", 0); !errors.Is(err, airport.ErrUnknownParking) {
		t.Errorf("unknown stand: %v", err)
	}
}

// TestStandAllocatorOverlap: two aircraft fit on overlapping stands only
// when their half spans and the wingtip clearance fit between the centres;
// S22 and S22A (10 m apart) never both hold an A320-size aircraft.
func TestStandAllocatorOverlap(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	c18, c19 := stand(t, g, "C18"), stand(t, g, "C19")
	if !slices.Contains(l.ParkingConflicts(c18), c19) {
		t.Fatal("C18 and C19 should overlap")
	}
	d := localDist(l.Parking[c18].Position, l.Parking[c19].Position)
	a := NewStandAllocator(nil, g)
	if err := a.Occupy(c18, "BIG", d); err != nil { // wings reaching past C19's centre
		t.Fatal(err)
	}
	if a.Free(c19, DefaultHalfSpanMeters) {
		t.Errorf("C19 free next to a %.0f m half span on C18 (%.1f m apart)", d, d)
	}
	if err := a.Occupy(c19, "A320", 0); !errors.Is(err, ErrStandTaken) {
		t.Errorf("occupy C19: %v", err)
	}
	a.Release(c18)
	small := (d - StandWingtipClearanceMeters) / 2 * 0.9
	if err := a.Occupy(c18, "SMALL", small); err != nil {
		t.Fatal(err)
	}
	if !a.Free(c19, small) {
		t.Errorf("C19 not free for a %.1f m half span next to the same (%.1f m apart)", small, d)
	}
	// A split stand: S22 blocks S22A for anything but a tiny aircraft.
	s22, s22a := stand(t, g, "S22"), stand(t, g, "S22A")
	if err := a.Occupy(s22, "GA", 5); err != nil {
		t.Fatal(err)
	}
	if a.Free(s22a, 5) {
		t.Error("S22A free with S22 taken")
	}
}

// TestStandAllocatorAssign: Assign skips taken stands, never picks a stand
// too small, prefers a short taxi-in from the runway, and runs out.
func TestStandAllocatorAssign(t *testing.T) {
	g := lkprGraph(t)
	a := NewStandAllocator(nil, g)
	seen := map[int]bool{}
	for i := 0; i < 5; i++ {
		s, err := a.Assign(StandRequirements{Owner: string(rune('A' + i)), Runway: "24"})
		if err != nil {
			t.Fatal(err)
		}
		if seen[s] {
			t.Fatalf("stand %s assigned twice", g.Layout.Parking[s].Label())
		}
		seen[s] = true
		if p := g.Layout.Parking[s]; p.Radius < DefaultHalfSpanMeters || p.Size() == airport.StandNone {
			t.Errorf("assigned %s (radius %.0f)", p.Label(), p.Radius)
		}
	}
	// The first pick has the shortest taxi-in of all free candidates it routed.
	b := NewStandAllocator(nil, g)
	first, _ := b.Assign(StandRequirements{Owner: "X", Runway: "24"})
	_, r1, err := bestExit(g, "24", first, airport.RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c22 := stand(t, g, "C22")
	if _, r2, err := bestExit(g, "24", c22, airport.RouteOptions{}); err == nil && first != c22 && r2.Length < r1.Length-1 {
		t.Errorf("assigned %s (%.0f m taxi) though C22 is %.0f m", g.Layout.Parking[first].Label(), r1.Length, r2.Length)
	}
	gates := []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE{types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY}
	n := len(g.Layout.SuitableStands(DefaultHalfSpanMeters, gates...))
	c := NewStandAllocator(nil, g)
	for i := 0; ; i++ {
		_, err := c.Assign(StandRequirements{Owner: string(rune('a' + i)), Types: gates})
		if errors.Is(err, ErrNoStand) {
			if i == 0 || i > n {
				t.Errorf("ran out after %d of %d heavy gates", i, n)
			}
			break
		}
		if err != nil || i > n {
			t.Fatalf("assign %d: %v", i, err)
		}
	}
}

// TestStandAllocatorAirline: at EDDM an airline gets one of its own stands.
func TestStandAllocatorAirline(t *testing.T) {
	b, err := os.ReadFile("../airport/testdata/EDDM-layout.json")
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
	a := NewStandAllocator(nil, g)
	s, err := a.Assign(StandRequirements{Owner: "DLH1", Airline: "DLH"})
	if err != nil {
		t.Fatal(err)
	}
	if p := g.Layout.Parking[s]; len(p.Airlines) == 0 || !p.ServesAirline("DLH") {
		t.Errorf("DLH got %s (airlines %v)", p.Label(), p.Airlines)
	}
	s2, err := a.Assign(StandRequirements{Owner: "ZZZ1", Airline: "ZZZ"})
	if err != nil {
		t.Fatal(err)
	}
	if p := g.Layout.Parking[s2]; len(p.Airlines) != 0 {
		t.Errorf("an unknown airline got %s, assigned to %v", p.Label(), p.Airlines)
	}
}

// TestStandAllocatorScan: a scan finds a parked aircraft on its stand (with
// its span) and the user's aircraft, ignores moving and airborne ones, and
// a later scan without them frees the stands.
func TestStandAllocatorScan(t *testing.T) {
	g := lkprGraph(t)
	c := &scanClient{}
	a := NewStandAllocator(c, g)
	if err := a.Scan(); err != nil {
		t.Fatal(err)
	}
	if len(c.byType) != 2 || c.byType[0] != types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT || c.byType[1] != types.SIMCONNECT_SIMOBJECT_TYPE_USER || c.radius[0] < 1000 {
		t.Fatalf("requests %v radius %v", c.byType, c.radius)
	}
	c22, c20, b15 := stand(t, g, "C22"), stand(t, g, "C20"), stand(t, g, "B15")
	at := func(i int) (float64, float64) {
		p := StandPoint(g.Layout.Parking[i], 0)
		return p.Lat, p.Lon
	}
	lat22, lon22 := at(c22)
	lat20, lon20 := at(c20)
	lat15, lon15 := at(b15)
	ai, user := DefaultStandRequestBase+standReqAircraft, DefaultStandRequestBase+standReqUser
	// The user's stand is reserved too: the scan adds the object and span.
	if err := a.Occupy(b15, "ME", 0); err != nil {
		t.Fatal(err)
	}
	for _, m := range []engine.Message{
		byTypeMsg(ai, 11, 1, 3, standScanData{Lat: lat22, Lon: lon22, OnGround: 1, WingSpanFt: 197}), // a B767 (60 m)
		byTypeMsg(ai, 12, 2, 3, standScanData{Lat: lat20, Lon: lon20, OnGround: 1, GroundSpeedKt: 8}), // pushing back
		byTypeMsg(ai, 13, 3, 3, standScanData{Lat: lat20, Lon: lon20, OnGround: 0}),                   // overhead
		byTypeMsg(user, 1, 1, 1, standScanData{Lat: lat15, Lon: lon15, OnGround: 1, WingSpanFt: 36}),
	} {
		if !a.Handle(m) {
			t.Fatal("scan answer not handled")
		}
	}
	occ := a.Occupancy()
	if o := occ[c22]; !o.Detected || o.ObjectID != 11 || o.HalfSpan < 29 || o.HalfSpan > 31 {
		t.Errorf("C22 %+v", o)
	}
	if o, ok := occ[b15]; !ok || o.ObjectID != 1 || o.Owner != "ME" || !o.Detected || o.HalfSpan > 6 {
		t.Errorf("user on B15: %+v %v", o, ok)
	}
	if _, ok := occ[c20]; ok || len(occ) != 2 {
		t.Errorf("occupancy %v", occ)
	}
	if err := a.Occupy(c22, "CSA1", 0); !errors.Is(err, ErrStandTaken) {
		t.Errorf("occupy a detected stand: %v", err)
	}
	// Next scan: the B767 left, nothing else around.
	a.Handle(byTypeMsg(ai, 0, 0, 0, standScanData{}))
	if _, ok := a.Occupant(c22); ok {
		t.Error("C22 still held after the aircraft left")
	}
	if _, ok := a.Occupant(b15); !ok {
		t.Error("the user's aircraft was dropped by the AI scan")
	}
	if a.Handle(byTypeMsg(999, 1, 1, 1, standScanData{})) {
		t.Error("handled a foreign request")
	}
}

func TestStandAllocatorRoutes(t *testing.T) {
	g := lkprGraph(t)
	a := NewStandAllocator(nil, g)
	if clash := a.ReserveRoute("A", []airport.NodeID{1, 2, 3}); len(clash) != 0 {
		t.Errorf("first route clashes with %v", clash)
	}
	a.ReserveRoute("B", []airport.NodeID{7, 8})
	if clash := a.ReserveRoute("C", []airport.NodeID{3, 8, 9}); !slices.Equal(clash, []string{"A", "B"}) {
		t.Errorf("clash %v, want [A B]", clash)
	}
	a.ReleaseRoute("A")
	if err := a.Occupy(0, "B", 0); err != nil {
		t.Fatal(err)
	}
	a.ReleaseOwner("B")
	if clash := a.ReserveRoute("D", []airport.NodeID{3, 8}); !slices.Equal(clash, []string{"C"}) {
		t.Errorf("clash after releases %v, want [C]", clash)
	}
	if _, ok := a.Occupant(0); ok {
		t.Error("ReleaseOwner kept the stand")
	}
}
