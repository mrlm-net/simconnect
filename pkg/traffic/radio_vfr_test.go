//go:build windows
// +build windows

package traffic

import "testing"

// The VFR circuit calls (#569), as Doc 4444 12.3.4.13–17 words them, and
// their readbacks.
func TestVFRPhrases(t *testing.T) {
	for _, c := range []struct {
		tx       Transmission
		said, rb string
	}{
		{JoinCircuit("OKABC", "left downwind", "24", "wind 240 degrees 8 knots", "1013", ""),
			"OKABC, join left downwind runway 24, wind 240 degrees 8 knots, QNH 1013", "Join left downwind runway 24, QNH 1013, OKABC"},
		{StraightIn("OKABC", "24", "", "1013"), "OKABC, make straight-in approach, runway 24, QNH 1013", "Straight-in approach runway 24, QNH 1013, OKABC"},
		{FollowTraffic("OKABC", 2, "the Airbus A320 on final"), "OKABC, number 2, follow the Airbus A320 on final", "Number 2, OKABC"},
		{FollowTraffic("OKABC", 1, ""), "OKABC, number 1", "Number 1, OKABC"},
		{CircuitInstruction("OKABC", InstrExtendDownwind), "OKABC, extend downwind", "Extend downwind, OKABC"},
		{CircuitDelay("OKABC", DelayOrbitRight), "OKABC, orbit right", "Orbit right, OKABC"},
		{ClearedTouchAndGo("OKABC", "24"), "OKABC, cleared touch and go", "Cleared touch and go, OKABC"},
		{MakeFullStop("OKABC"), "OKABC, make full stop", "Make full stop, OKABC"},
	} {
		if c.tx.Text != c.said {
			t.Errorf("said %q, want %q", c.tx.Text, c.said)
		}
		rb, ok := Readback(c.tx)
		if !ok || rb.Text != c.rb {
			t.Errorf("readback of %q: %q (%v), want %q", c.said, rb.Text, ok, c.rb)
		}
	}
	if s := VFRForLanding("Ruzyne Tower", "OKABC", "Cessna 172", "5 miles north", "2000 feet", "Alpha").Text; s != "Ruzyne Tower, OKABC, Cessna 172, 5 miles north, 2000 feet, information Alpha, for landing" {
		t.Errorf("first call %q", s)
	}
	if s := CircuitReport("OKABC", "downwind").Text; s != "OKABC, downwind" {
		t.Errorf("report %q", s)
	}
}
