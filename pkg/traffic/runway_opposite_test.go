package traffic

import (
	"slices"
	"testing"
	"time"
)

// TestRunwayControllerOppositePlanned (#1661): take-offs from one end and
// landings on the other (a one-way end: TFFJ, EHAM 18R/36L). A departure
// is cleared onto the runway only if, at the arrival's speed now, it is
// airborne and past the far end before the arrival to the other end comes
// inside OppositeArrivalNM; one lined up is held while the arrival is
// inside it, and the arrival is not cleared to land with it there.
func TestRunwayControllerOppositePlanned(t *testing.T) {
	now := time.Now()
	end := func(u RunwayUser, e string) RunwayUser { u.End = e; return u }
	fast := end(final("FAST1", 16), "27")
	fast.GroundKts = 250 // 4 NM to OppositeArrivalNM in 58 s
	c := NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{end(dep("DEP1", "A320", RunwayHoldingShort), "09"), fast})
	if slices.Contains(c.LineUp, "DEP1") || slices.Contains(c.Takeoff, "DEP1") || len(c.LineUpBehind) != 0 {
		t.Errorf("onto 09 with FAST1 at 16 NM, 250 kt, for 27 (inside %.0f NM in 58 s): %+v", OppositeArrivalNM, c)
	}
	slow := end(final("SLOW1", 20), "27")
	slow.GroundKts = 100 // 8 NM in 288 s
	c = NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{end(dep("DEP1", "DHC6", RunwayHoldingShort), "09"), slow})
	if !slices.Contains(c.Takeoff, "DEP1") {
		t.Errorf("onto 09 with SLOW1 at 20 NM, 100 kt, for 27: %+v, want line up and take off", c)
	}
	// Lined up: held inside OppositeArrivalNM, never climbing at the
	// arrival; the arrival is not cleared to land meanwhile.
	c = NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{end(dep("DEP1", "DHC6", RunwayLinedUp), "09"), end(final("ARR1", 9), "27")})
	if slices.Contains(c.Takeoff, "DEP1") || slices.Contains(c.Land, "ARR1") {
		t.Errorf("lined up on 09, ARR1 at 9 NM for 27: %+v, want neither cleared", c)
	}
	c = NewRunwayController(RunwayControllerOptions{}).Decide(now, []RunwayUser{end(dep("DEP1", "DHC6", RunwayLinedUp), "09"), end(final("ARR1", 13), "27")})
	if !slices.Contains(c.Takeoff, "DEP1") {
		t.Errorf("lined up on 09, ARR1 at 13 NM for 27: %+v, want the take-off", c)
	}
}
