//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"
)

// Every clearance kind: its position, intent and the text the map's log
// has always said.
func TestTransmissionPhrases(t *testing.T) {
	efc := time.Date(2026, 9, 30, 10, 42, 0, 0, time.UTC)
	cases := []struct {
		tx       Transmission
		pos      Position
		intent   Intent
		text     string
		param    string
		paramVal string
	}{
		{ClearedDeparture("CSA1", "VOZ5M", "24"), PosDelivery, IntentDepartureClearance, "CSA1, cleared VOZ5M departure, runway 24", ParamSID, "VOZ5M"},
		{ClearedArrival("CSA1", "GOLO4S", "ILS", "24"), PosApproach, IntentArrivalClearance, "CSA1, cleared GOLO4S arrival, expect ILS approach runway 24", ParamSTAR, "GOLO4S"},
		{ClearedPushback("CSA1"), PosGround, IntentPushback, "CSA1, push back and start-up approved", "", ""},
		{ClearedTaxiToRunway("CSA1", "24", "", []string{"B2", "H", "A"}), PosGround, IntentTaxi, "CSA1, taxi to holding point runway 24 via B2, H, A", ParamTaxiways, "B2, H, A"},
		{ClearedTaxiToRunway("CSA1", "24", "B", nil), PosGround, IntentTaxi, "CSA1, taxi to holding point B runway 24", ParamEntry, "B"},
		{ClearedTaxiToStand("CSA1", "C22", []string{"B", "D"}), PosGround, IntentTaxi, "CSA1, taxi to stand C22 via B, D", ParamStand, "C22"},
		{ClearedTaxiUpTo("CSA1", []string{"H"}, "A"), PosGround, IntentTaxiLimit, "CSA1, taxi via H, hold short of A", ParamLimit, "A"},
		{ClearedTaxiUpTo("CSA1", nil, ""), PosGround, IntentTaxiLimit, "CSA1, taxi, hold position at the marked point", "", ""},
		{ClearedCross("CSA1", "12/30"), PosGround, IntentCross, "CSA1, cross runway 12/30", ParamRunway, "12/30"},
		{ClearedLineUp("CSA1", "06"), PosTower, IntentLineUp, "CSA1, runway 06, line up and wait", ParamRunway, "06"},
		{ClearedTakeoff("CSA1", "06", false), PosTower, IntentTakeoff, "CSA1, runway 06, cleared for take-off", "", ""},
		{ClearedTakeoff("CSA1", "06", true), PosTower, IntentTakeoff, "CSA1, runway 06, line up, cleared for take-off", ParamLineUp, "true"},
		{HoldPosition("CSA1"), PosGround, IntentHoldPosition, "CSA1, hold position", "", ""},
		{Stop("CSA1"), PosTower, IntentStop, "CSA1, stop immediately, I say again, stop immediately", "", ""},
		{CancelTakeoff("CSA1"), PosTower, IntentCancelTakeoff, "CSA1, hold position, cancel take-off clearance, I say again, cancel take-off clearance", "", ""},
		{GoAround("CSA1", ""), PosTower, IntentGoAround, "CSA1, go around, I say again, go around", "", ""},
		{GoAround("CSA1", "GAT1 on the runway"), PosTower, IntentGoAround, "CSA1, go around, I say again, go around — GAT1 on the runway", ParamReason, "GAT1 on the runway"},
		{Sequenced("CSA1", 2, 83*time.Second, Absorption{SpeedKts: 210, ExtraNM: 4.9}), PosApproach, IntentSequence, "CSA1, number 2, delay 1m23s: 210 kt, +4.9 NM", ParamNumber, "2"},
		{Sequenced("CSA1", 1, 0, Absorption{SpeedKts: 233}), PosApproach, IntentSequence, "CSA1, number 1, lose a minute: 233 kt", "", ""},
		{DirectToFinal("CSA1", 3), PosApproach, IntentDirect, "CSA1, proceed direct to the final, number 3", ParamNumber, "3"},
		{HoldAt("CSA1", "PR711", EntryTeardrop, 6000, efc), PosApproach, IntentHold, "CSA1, hold at PR711, teardrop entry, maintain 6000 ft, expect further clearance 10:42", ParamFix, "PR711"},
		{LeaveHoldAt("CSA1", "PR711", 2), PosApproach, IntentLeaveHold, "CSA1, leave the hold at PR711, number 2, continue the arrival", "", ""},
		{HoldDescend("CSA1", 5000), PosApproach, IntentHoldLevel, "CSA1, descend 5000 ft, hold as published", ParamAltitude, "5000"},
	}
	for _, c := range cases {
		if c.tx.Position != c.pos || c.tx.Intent != c.intent || c.tx.Text != c.text {
			t.Errorf("%s: %s %s %q, want %s %s %q", c.intent, c.tx.Position, c.tx.Intent, c.tx.Text, c.pos, c.intent, c.text)
		}
		if c.param != "" && c.tx.Params[c.param] != c.paramVal {
			t.Errorf("%s: %s = %q, want %q", c.intent, c.param, c.tx.Params[c.param], c.paramVal)
		}
	}
}

// Conflict resolutions as ATC says them.
func TestResolvedPhrases(t *testing.T) {
	why := "traffic DLH2, 0.8 NM in 2m40s"
	for _, c := range []struct {
		r            Resolution
		alt, hdg, kt float64
		want         string
	}{
		{Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 21000, Why: why}, 20000, 90, 450, "CSA1, climb flight level 210, " + why},
		{Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 8000, Why: why}, 9000, 90, 250, "CSA1, descend altitude 8000 feet, " + why},
		{Resolution{Callsign: "CSA1", Kind: ResolveSpeed, Kts: 384, Why: why}, 30000, 90, 480, "CSA1, reduce speed 384 knots, " + why},
		{Resolution{Callsign: "CSA1", Kind: ResolveHeading, HeadingDeg: 110, Why: why}, 20000, 90, 450, "CSA1, turn right heading 110, " + why},
		{Resolution{Callsign: "CSA1", Kind: ResolveHeading, HeadingDeg: 70, Why: why}, 20000, 90, 450, "CSA1, turn left heading 070, " + why},
	} {
		if got := Resolved(PosCenter, c.r, c.alt, c.hdg, c.kt); got.Text != c.want || got.Position != PosCenter {
			t.Errorf("%q, want %q", got.Text, c.want)
		}
	}
}

// The radio stamps, keeps (up to Keep) and hands on every transmission;
// Recent is by airport, oldest first.
func TestRadio(t *testing.T) {
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	var heard []Transmission
	r := NewRadio(RadioOptions{Keep: 3, Now: func() time.Time { return at }, OnTransmission: func(t Transmission) { heard = append(heard, t) }})
	r.Transmit("LKPR", ClearedPushback("CSA1"))
	r.Transmit("LKPR", ClearedLineUp("CSA2", "24"))
	r.Transmit("LKTB", ClearedPushback("TVS3"))
	r.Transmit("LKPR", ClearedTakeoff("CSA2", "24", false))
	if len(heard) != 4 || heard[0].At != at || heard[0].Airport != "LKPR" {
		t.Fatalf("heard %+v", heard)
	}
	got := r.Recent("LKPR", 0)
	if len(got) != 2 || got[0].Intent != IntentLineUp || got[1].Intent != IntentTakeoff {
		t.Errorf("recent at LKPR (3 kept): %+v", got)
	}
	if all := r.Recent("", 2); len(all) != 2 || all[1].Callsign != "CSA2" {
		t.Errorf("recent 2: %+v", all)
	}
}

// Who works an aircraft in each state: delivery, ground, tower, departure;
// approach, tower once on the final, ground after vacating. Crossings stay
// with ground.
func TestPositions(t *testing.T) {
	for _, c := range []struct {
		s    TaxiState
		own  bool
		want Position
	}{
		{TaxiSpawning, false, PosDelivery}, {TaxiAwaitingPushback, false, PosGround}, {TaxiTaxiing, false, PosGround},
		{TaxiHoldingShort, false, PosGround}, {TaxiHoldingShort, true, PosTower}, {TaxiLinedUp, true, PosTower},
		{TaxiDeparting, true, PosTower}, {TaxiComplete, true, PosDeparture},
	} {
		if got := DeparturePosition(c.s, c.own); got != c.want {
			t.Errorf("departure %v (own runway %v): %s, want %s", c.s, c.own, got, c.want)
		}
	}
	for _, c := range []struct {
		s       ArrivalState
		onFinal bool
		want    Position
	}{
		{ArrivalSpawning, false, PosApproach}, {ArrivalApproaching, false, PosApproach}, {ArrivalApproaching, true, PosTower},
		{ArrivalRollout, false, PosTower}, {ArrivalVacating, false, PosTower}, {ArrivalAwaitingTaxi, false, PosGround},
		{ArrivalHoldingShort, false, PosGround}, {ArrivalParked, false, PosGround},
	} {
		if got := ArrivalPosition(c.s, c.onFinal); got != c.want {
			t.Errorf("arrival %v (on final %v): %s, want %s", c.s, c.onFinal, got, c.want)
		}
	}
}

// A handoff is said by the position handing over, with the next station
// and its frequency.
func TestHandoff(t *testing.T) {
	h := Handoff("CSA1", PosGround, PosTower, StationName("PRAHA TOWER", PosTower), "118.105")
	if h.Position != PosGround || h.Intent != IntentContact || h.Text != "CSA1, contact Praha Tower 118.105" || h.Params[ParamPosition] != "tower" {
		t.Errorf("%+v", h)
	}
	if h := Handoff("CSA1", PosTower, PosDeparture, "", ""); h.Text != "CSA1, contact Departure" {
		t.Errorf("without name and frequency: %q", h.Text)
	}
	for name, want := range map[string]string{"RUZYNE": "Ruzyne Tower", "PRAGUE INFORMATION": "Prague Information", "": "Tower"} {
		if got := StationName(name, PosTower); got != want {
			t.Errorf("StationName(%q) = %q, want %q", name, got, want)
		}
	}
}

// The radio puts a transmission on its position's frequency, and one at a
// time: the next on a busy frequency is said when the last has been.
func TestRadioFrequencies(t *testing.T) {
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	freqs := map[Position]string{PosTower: "118.105", PosGround: "121.905"}
	var heard []Transmission
	r := NewRadio(RadioOptions{Now: func() time.Time { return at },
		FrequencyOf:    func(_ string, p Position) string { return freqs[p] },
		OnTransmission: func(t Transmission) { heard = append(heard, t) }})
	r.Transmit("LKPR", ClearedLineUp("CSA1", "24"))
	r.Transmit("LKPR", ClearedTakeoff("CSA2", "24", false))
	r.Transmit("LKPR", ClearedPushback("CSA3"))
	if heard[0].Frequency != "118.105" || heard[2].Frequency != "121.905" {
		t.Fatalf("frequencies %s %s %s", heard[0].Frequency, heard[1].Frequency, heard[2].Frequency)
	}
	if want := at.Add(SpeakingTime(heard[0].Text) + time.Second); !heard[1].At.Equal(want) {
		t.Errorf("second on tower at %v, want %v (after the first)", heard[1].At, want)
	}
	if !heard[2].At.Equal(at) {
		t.Errorf("ground, not busy: at %v", heard[2].At)
	}
}
