package traffic

import (
	"testing"
	"time"
)

// TestLandsInSlowsToFinal: an arrival 9 NM out at 180 kt lands after it
// has slowed to its approach speed, not at 180 kt all the way; the
// departure lined up behind the one landing now still goes before it
// (live, LKPR: CSA1911 told to hold position, CSA1080 landed 4 minutes
// after the one before).
func TestLandsInSlowsToFinal(t *testing.T) {
	next := RunwayUser{Callsign: "CSA1080", Wake: WakeFor("A320"), Phase: RunwayFinal, Arrival: true, DistanceNM: 9, GroundKts: 180, FinalKts: 138}
	if in := next.LandsIn(); in < 200*time.Second || in > 210*time.Second {
		t.Errorf("9 NM at 180 kt, final 138 kt: lands in %v, want about 204 s", in)
	}
	// Inside FinalSlowNM: at its speed now.
	close := next
	close.DistanceNM = 3
	if in := close.LandsIn(); in != 60*time.Second {
		t.Errorf("3 NM at 180 kt: lands in %v, want 60 s", in)
	}
	rc := NewRunwayController(RunwayControllerOptions{})
	first := RunwayUser{Callsign: "CSA312", Wake: WakeFor("A320")}
	if !rc.BehindRoom(first, next.LandsIn(), RunwayUser{Callsign: "CSA1911", Wake: WakeFor("A320")}) {
		t.Errorf("no room behind the landing A320 with the next %v out", next.LandsIn())
	}
}
