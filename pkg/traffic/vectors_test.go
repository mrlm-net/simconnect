package traffic

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// vectorArrival is an arrival on LKPR's star to runway 06, at its first
// point, injected and on its procedure.
func vectorArrival(t *testing.T, star string) (*ArrivalController, []airport.NavPoint) {
	t.Helper()
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("06", star)
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "06", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 90, 250, false))
	return ctl, route
}

// moveTo moves the arrival to p heading hdg.
func moveTo(ctl *ArrivalController, p airport.LatLon, hdg float64) {
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, p, 5000, hdg, 220, false))
}

// flyTo flies the arrival corner by corner up to the one at p, and returns
// the vectors said on the way.
func flyTo(t *testing.T, ctl *ArrivalController, p airport.LatLon) []Vector {
	t.Helper()
	ctl.mu.Lock()
	corners, end := append(ctl.corners[:0:0], ctl.corners...), ctl.cornerIndex(p)
	ctl.mu.Unlock()
	if end < 0 {
		t.Fatalf("no corner at %+v", p)
	}
	var said []Vector
	prev := ctl.last.Position
	for _, w := range corners[:end+1] {
		q := airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		moveTo(ctl, q, calc.BearingDegrees(prev.Lat, prev.Lon, q.Lat, q.Lon))
		prev = q
		for {
			v, ok := ctl.VectorDue()
			if !ok {
				break
			}
			said = append(said, v)
		}
	}
	return said
}

// TestVectorsExtendedDownwind: an extended downwind is flown on vectors
// (#661): "fly heading, for spacing" leaving the STAR at its downwind's
// end, "turn heading, for base" at the new base turn, and cleared for the
// approach with the intercept heading, along the final. No "extend
// downwind": that is the aerodrome circuit's (Doc 4444 12.3.4).
func TestVectorsExtendedDownwind(t *testing.T) {
	ctl, _ := vectorArrival(t, "VLM")
	if _, ok := ctl.InterceptHeading(); ok {
		t.Fatal("on its STAR: an intercept heading, want none")
	}
	if _, err := ctl.AbsorbDelay(3 * time.Minute); err != nil { // speed takes it
		t.Fatal(err)
	}
	a, err := ctl.AbsorbDelay(3 * time.Minute)
	if err != nil || a.ExtraNM < 5 || !a.Downwind {
		t.Fatalf("%+v %v, want the downwind extended", a, err)
	}
	if len(ctl.vectors) != 2 || ctl.vectors[0].For != "spacing" || ctl.vectors[1].For != "base" {
		t.Fatalf("vectors %+v, want leaving the STAR for spacing, then the base turn", ctl.vectors)
	}
	if v, ok := ctl.VectorDue(); ok {
		t.Fatalf("due at the start of the STAR: %+v", v)
	}
	leave, base := ctl.vectors[0], ctl.vectors[1]
	if said := flyTo(t, ctl, leave.At); len(said) != 1 || said[0].For != "spacing" {
		t.Fatalf("to the downwind's end: %+v, want the heading for spacing", said)
	}
	// On to the new base turn: the turn for base.
	if said := flyTo(t, ctl, base.At); len(said) != 1 || said[0].For != "base" || said[0].Turn == "" {
		t.Fatalf("to the base turn: %+v, want the turn for base", said)
	}
	h, ok := ctl.InterceptHeading()
	if !ok || math.Abs(headingDiff(h, ctl.plan.End.Heading)) > 45 {
		t.Errorf("intercept %.0f %v, want within 45° of the runway's %.0f", h, ok, ctl.plan.End.Heading)
	}
	// Near the turn onto the final course: the approach clearance is due,
	// however far out that turn is (live, OKKSC onto a 26 NM final unsaid).
	ctl.mu.Lock()
	turn := ctl.corners[ctl.finalTurn()]
	ctl.mu.Unlock()
	flyTo(t, ctl, airport.LatLon{Lat: turn.Latitude, Lon: turn.Longitude})
	if !ctl.TurningFinal() {
		t.Error("at the turn onto the final course: no approach clearance due")
	}
}

// TestVectorsDogLeg: a dog-leg off a STAR leg is "fly heading, for
// spacing" out, then back onto the STAR at its next fix, "resume own
// navigation direct" when it is named (#661).
func TestVectorsDogLeg(t *testing.T) {
	ctl, _ := vectorArrival(t, "LOMKI") // no downwind to extend: a dog-leg
	a, err := ctl.AbsorbDelay(4 * time.Minute)
	if err != nil || a.ExtraNM <= 0 || a.Orbit != "" {
		t.Fatalf("%+v %v, want a dog-leg", a, err)
	}
	if len(ctl.vectors) != 2 || ctl.vectors[0].For != "spacing" {
		t.Fatalf("vectors %+v, want spacing then back", ctl.vectors)
	}
	back := ctl.vectors[1]
	if back.For != "" || back.Fix == "" {
		t.Errorf("back onto the STAR %+v: direct to its next fix, no reason said", back)
	}
	if _, ok := ctl.InterceptHeading(); !ok {
		t.Error("on vectors: no intercept heading")
	}
	// A new procedure: on its own navigation again.
	ctl.mu.Lock()
	ctl.setCorners(ctl.corners, ctl.cornerNames)
	ctl.mu.Unlock()
	if _, ok := ctl.InterceptHeading(); ok || len(ctl.vectors) != 0 {
		t.Error("vectors kept over a new procedure")
	}
}

// TestVectorPhrases: Doc 4444 12.4.1.3 d/e with the reason (12.4.1.5 note),
// 12.4.1.4 b, and the intercept before the approach clearance (12.4.2.2 g),
// magnetic, with their readbacks (#661).
func TestVectorPhrases(t *testing.T) {
	for _, c := range []struct {
		v        Vector
		mv       float64
		said, rb string
	}{
		{Vector{HeadingDeg: 56, For: "spacing"}, 4, "CSA1, fly heading 060, for spacing", "Fly heading 060, CSA1"},
		{Vector{HeadingDeg: 146, Turn: "left", For: "base"}, 4, "CSA1, turn left heading 150, for base", "Turn left heading 150, CSA1"},
		{Vector{HeadingDeg: 357, Turn: "right"}, 3, "CSA1, turn right heading 360", "Turn right heading 360, CSA1"},
		{Vector{Fix: "GOLOP"}, 4, "CSA1, resume own navigation direct GOLOP", "Resume own navigation direct GOLOP, CSA1"},
	} {
		tx := Vectored("CSA1", c.v, c.mv)
		if tx.Text != c.said {
			t.Errorf("%+v: %q, want %q", c.v, tx.Text, c.said)
		}
		if rb, ok := Readback(tx); !ok || rb.Text != c.rb {
			t.Errorf("%+v: readback %q %v, want %q", c.v, rb.Text, ok, c.rb)
		}
	}
	tx := ClearedApproachTo("CSA1", ApproachClearance{Kind: "ILS", Runway: "24", QNH: "1013", ReportEstablished: true, Intercept: "270", Turn: "left"})
	if want := "CSA1, turn left heading 270 to intercept, cleared ILS approach runway 24, QNH 1013, report established"; tx.Text != want {
		t.Errorf("%q, want %q", tx.Text, want)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Turn left heading 270 to intercept, cleared ILS approach runway 24, QNH 1013, report established, CSA1" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	if TurnTo(350, 10) != "right" || TurnTo(10, 350) != "left" {
		t.Error("TurnTo across north")
	}
}

// TestWithVector: the sequence call and the vector in one, read back
// together (#707).
func TestWithVector(t *testing.T) {
	tx := Sequenced("KLM868", 3, time.Minute, Absorption{SpeedKts: 210})
	tx = WithVector(tx, Vector{HeadingDeg: 138, For: "spacing"}, 4)
	if want := ", fly heading 140, for spacing"; !strings.HasSuffix(tx.Text, want) || !strings.HasPrefix(tx.Text, "KLM868, number 3") {
		t.Errorf("%q, want the sequence call ending %q", tx.Text, want)
	}
	rb, ok := Readback(tx)
	if !ok || !strings.Contains(rb.Text, "210 knots") || !strings.HasSuffix(rb.Text, "fly heading 140, KLM868") {
		t.Errorf("readback %q %v, want the speed and the heading", rb.Text, ok)
	}
}

// TestJoined: departure's "identified, climb" and the answer to the
// crew's request for direct in one call (live, LOT924).
func TestJoined(t *testing.T) {
	tx := Joined(Identified(PosDeparture, "LOT924", "flight level 240"), ClearedDirectTo(PosDeparture, "LOT924", "ARTUP"))
	if want := "LOT924, identified, climb to flight level 240, cleared direct to ARTUP"; !strings.HasPrefix(tx.Text, "LOT924, identified") || !strings.HasSuffix(tx.Text, "ARTUP") {
		t.Errorf("%q, want like %q", tx.Text, want)
	}
	rb, ok := Readback(tx)
	if !ok || !strings.Contains(rb.Text, "flight level 240") || !strings.Contains(rb.Text, "direct") || !strings.HasSuffix(rb.Text, "ARTUP, LOT924") {
		t.Errorf("readback %q %v, want the climb and the direct", rb.Text, ok)
	}
	t.Log(tx.Text, " / ", rb.Text)
}

// TestJoinedReadbackWithoutOwn: a joined call whose first part needs no
// readback ("identified" alone) still reads the joined one back (MyCrew).
func TestJoinedReadbackWithoutOwn(t *testing.T) {
	tx := Joined(Identified(PosApproach, "CSA1", ""), ClearedApproachTo("CSA1", ApproachClearance{Kind: "ILS", Runway: "09", QNH: "1018", ReportEstablished: true}))
	rb, ok := Readback(tx)
	if !ok || !strings.HasPrefix(rb.Text, "Cleared ILS approach runway 09, QNH 1018") || !strings.HasSuffix(rb.Text, ", CSA1") {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	t.Log(rb.Text)
}

// TestAtEntry: from an intersection, the runway is said with it (JO
// 7110.65 3-9-4, 3-9-10), in the clearance and its readback.
func TestAtEntry(t *testing.T) {
	tx := AtEntry(ClearedTakeoff("BAW1367", "24", "wind 360 degrees 5 knots"), "B")
	if want := "BAW1367, runway 24 at B, cleared for take-off, wind 360 degrees 5 knots"; tx.Text != want {
		t.Errorf("%q, want %q", tx.Text, want)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Runway 24 at B, cleared for take-off, BAW1367" {
		t.Errorf("readback %q", rb.Text)
	}
	if lu := AtEntry(ClearedLineUp("BAW1367", "24"), "B"); lu.Text != "BAW1367, runway 24 at B, line up and wait" {
		t.Errorf("line-up %q", lu.Text)
	}
	if full := AtEntry(ClearedLineUp("BAW1367", "24"), ""); full.Text != "BAW1367, runway 24, line up and wait" {
		t.Errorf("full length %q", full.Text)
	}
}

// TestWithWake: "heavy" or "super" after the call sign in a first call
// (Doc 4444 4.9.2); nothing for the rest.
func TestWithWake(t *testing.T) {
	tx := WithWake(CheckIn(PosDeparture, "Ruzyne Radar", "BAW1367", "passing 3000 feet", ""), WakeHeavy)
	if !strings.Contains(tx.Text, "BAW1367 heavy,") {
		t.Errorf("%q", tx.Text)
	}
	if tx := WithWake(CheckIn(PosDeparture, "Ruzyne Radar", "UAE139", "", ""), WakeSuper); !strings.Contains(tx.Text, "UAE139 super") {
		t.Errorf("%q", tx.Text)
	}
	if tx := WithWake(CheckIn(PosDeparture, "Ruzyne Radar", "CSA1", "", ""), WakeMedium); strings.Contains(tx.Text, "heavy") {
		t.Errorf("%q", tx.Text)
	}
}

// TestVectorsNoCircuitPointAsFix: back from a dog-leg onto a go-around's
// circuit, radar gives the heading for base; the circuit's own turning
// points are no fixes (live, LKPR: KLM1683 "resume own navigation direct
// BASE").
func TestVectorsNoCircuitPointAsFix(t *testing.T) {
	var plain []types.SIMCONNECT_DATA_WAYPOINT
	for i := 0; i < 6; i++ {
		plain = append(plain, types.SIMCONNECT_DATA_WAYPOINT{Latitude: 50 + 0.05*float64(i), Longitude: 14 + 0.03*float64(i%2)})
	}
	names := []string{"", "", "DOWNWIND", "BASE", "", "FINAL"}
	vs := vectorsFor(airport.LatLon{Lat: 49.95, Lon: 14}, plain, names, -1, 1)
	if len(vs) != 2 {
		t.Fatalf("vectors %+v, want out and back", vs)
	}
	if back := vs[1]; back.Fix != "" || back.For != "base" {
		t.Fatalf("back %+v: want a heading for base, no circuit point as its fix", back)
	}
}
