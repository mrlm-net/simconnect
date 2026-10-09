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
		{CircuitInstruction("OKABC", InstrExtendCallBase), "OKABC, extend downwind, I'll call your base", "Extend downwind, OKABC"},
		{CircuitInstruction("OKABC", InstrTurnBase), "OKABC, turn base now", "Turn base now, OKABC"},
		{ClearedLineUpBehindDeparting("OKABC", "Airbus A320", "24"), "OKABC, behind the departing Airbus A320, line up and wait runway 24, behind", "Behind the departing Airbus A320, line up and wait runway 24, behind, OKABC"},
		{CircuitDelay("OKABC", DelayOrbitRight), "OKABC, orbit right", "Orbit right, OKABC"},
		{ClearedTouchAndGo("OKABC", "24"), "OKABC, cleared touch and go", "Cleared touch and go, OKABC"},
		{MakeFullStop("OKABC"), "OKABC, make full stop", "Make full stop, OKABC"},
		{ClearedStopAndGo("OKABC", "24"), "OKABC, cleared stop and go", "Cleared stop and go, OKABC"},
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

func TestStandDelayPhrases(t *testing.T) {
	tx := RequestStandDelay("Ruzyne Ground", "CSA1", "B9", 10, "waiting for passengers")
	if want := "Ruzyne Ground, CSA1, stand B9, request delay on stand, about 10 minutes, waiting for passengers"; tx.Text != want || !tx.Pilot {
		t.Errorf("request %q (pilot %v)", tx.Text, tx.Pilot)
	}
	if got := StandDelayApproved("CSA1", false).Text; got != "CSA1, roger, call when ready for pushback" {
		t.Errorf("answer %q", got)
	}
	if got := StandDelayApproved("OKABC", true).Text; got != "OKABC, roger, call when ready for start-up" {
		t.Errorf("answer %q", got)
	}
}

func TestRunwayRequestPhrases(t *testing.T) {
	if got := RequestTaxiRunway("CSA1", "30").Text; got != "CSA1, request taxi, request runway 30 for departure" {
		t.Errorf("request %q", got)
	}
	if got := UnableRunway("CSA1", "24").Text; got != "CSA1, unable, runway 24 in use" {
		t.Errorf("answer %q", got)
	}
}
