package traffic

import (
	"testing"
	"time"
)

// TestNoLineUpBehindLongInterval: a jet behind a slow aircraft cleared for
// take-off at an intersection, on the same route, does not line up behind it
// with an arrival due before its own interval runs out (live, LKPR: TVS1109
// lined up behind OKNRF, a Citation at E, waited 2 minutes 13 lined up, and
// LOT1058 went around).
func TestNoLineUpBehindLongInterval(t *testing.T) {
	now := time.Now()
	r := NewRunwayController(RunwayControllerOptions{})
	c25 := dep("OKNRF", "C25C", RunwayHoldingShort)
	c25.AlongM, c25.ClimbKts = 800, 160
	if c := r.Decide(now, []RunwayUser{c25}); len(c.Takeoff) != 1 {
		t.Fatalf("OKNRF alone: %+v, want cleared for take-off", c)
	}
	c25.Phase = RunwayLinedUp
	jet := dep("TVS1109", "B738", RunwayHoldingShort)
	jet.ClimbKts = 250
	rush := dep("CSA986", "A320", RunwayHoldingShort)
	iv := DepartureIntervalSpeeds(c25.Wake, jet.Wake, true, c25.ClimbKts, jet.ClimbKts)
	if iv < 90*time.Second {
		t.Fatalf("interval %s behind the Citation: the case needs a long one", iv)
	}
	for _, nm := range []float64{5, 6, 7, 8} {
		c := r.Decide(now.Add(5*time.Second), []RunwayUser{c25, jet, rush, final("LOT1058", nm)})
		if c.LineUpBehindDeparting["TVS1109"] != "" {
			t.Errorf("arrival on a %.0f NM final, %s to wait behind OKNRF: TVS1109 lined up behind it", nm, iv.Round(time.Second))
		}
	}
}
