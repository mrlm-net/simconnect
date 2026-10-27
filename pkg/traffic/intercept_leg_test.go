package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestVectorsInterceptLeg: vectored onto LKPR 24 from VLM's extended
// downwind, the base no longer meets the final square (live: every
// vectored arrival heard "turn left heading 330, for base", then "cleared
// ILS approach" with no heading). The leg onto the final is an intercept
// of InterceptAngleDeg, the clearance comes as the arrival turns onto it,
// and its heading is one the arrival can fly to the final (Doc 4444
// 8.9.3.6: 45° or less).
func TestVectorsInterceptLeg(t *testing.T) {
	g := lkprGraph(t)
	for _, star := range []string{"VLM", "GOLOP"} {
		route, err := lkprProcedures(t).Arrival("24", star)
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 0, 250, false))
		ctl.AbsorbDelay(3 * time.Minute)
		a, err := ctl.AbsorbDelay(3 * time.Minute)
		if err != nil || !a.Downwind {
			t.Logf("%s: %+v %v: no downwind extended", star, a, err)
			continue
		}
		if len(ctl.vectors) != 2 || ctl.vectors[1].For != "base" {
			t.Fatalf("%s: vectors %+v", star, ctl.vectors)
		}
		leave, base := ctl.vectors[0], ctl.vectors[1]
		flyTo(t, ctl, leave.At)
		flyTo(t, ctl, base.At)
		h, ok := ctl.InterceptHeading()
		course := ctl.plan.End.Heading
		if off := math.Abs(headingDiff(h, course)); !ok || off > InterceptMaxDeg || off < 15 {
			t.Errorf("%s: intercept heading %.0f %v, %.0f° off the final %.0f: want an intercept of %.0f°", star, h, ok, off, course, InterceptAngleDeg)
		}
		ctl.mu.Lock()
		turn := ctl.corners[ctl.finalTurn()]
		ctl.mu.Unlock()
		flyTo(t, ctl, airport.LatLon{Lat: turn.Latitude, Lon: turn.Longitude})
		if !ctl.TurningFinal() {
			t.Errorf("%s: at the turn onto the intercept: no clearance due", star)
		}
		said := math.Round(h/HeadingStepDeg) * HeadingStepDeg
		if !ctl.InterceptOnHeading(said) {
			t.Errorf("%s: heading %.0f does not reach the final from the intercept turn", star, said)
		}
	}
}

// TestShortDownwindIsSpeed: a downwind extended by less than
// MinDownwindVector of flight is not extended: no "fly heading, for
// spacing" with the base turn seconds after it (live, WZZ1042: 060, then
// 330 for base 17 s later); speed takes the time.
func TestShortDownwindIsSpeed(t *testing.T) {
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "VLM")
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
		InjectApproach: true, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 0, 250, false))
	ctl.AbsorbDelay(3 * time.Minute) // speed takes it
	a, err := ctl.AbsorbDelay(20 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Downwind || len(ctl.vectors) > 0 {
		t.Errorf("20 s more: %+v, vectors %+v; want speed, no downwind extended", a, ctl.vectors)
	}
}

// TestInterceptClearancePhrase: on vectors the approach clearance comes
// with its position, the heading to intercept and the clearance in one
// call (Doc 4444 8.9.3.3, 12.4.2.2 a, g, d, e; FAA 7110.65 5-9-4 example
// 2's order); the crew reads back the heading and the clearance, not the
// position.
func TestInterceptClearancePhrase(t *testing.T) {
	tx := ClearedApproachTo("CSA1", ApproachClearance{Kind: "ILS", Runway: "24", QNH: "1013", ReportEstablished: true,
		Intercept: "270", Turn: "left", FromTouchdownNM: 11})
	if want := "CSA1, 11 miles from touchdown, turn left heading 270 to intercept, cleared ILS approach runway 24, QNH 1013, report established"; tx.Text != want {
		t.Errorf("said %q,\nwant  %q", tx.Text, want)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "Turn left heading 270 to intercept, cleared ILS approach runway 24, QNH 1013, report established, CSA1" {
		t.Errorf("read back %q", rb.Text)
	}
}
