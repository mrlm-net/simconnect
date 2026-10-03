//go:build windows
// +build windows

package traffic

import (
	"strings"
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
		{ClearedDeparture("CSA1", DepartureClearance{Destination: "Frankfurt", SID: "BALTU 7D", Runway: "24", Level: "5000 feet", Squawk: "4521"}), PosDelivery, IntentDepartureClearance, "CSA1, cleared to Frankfurt, BALTU 7D departure, flight planned route, runway 24, climb via SID to 5000 feet, squawk 4521", ParamSquawk, "4521"}, // Doc 4444 6.3.2.3, CAP 413 2.68
		{ClearedDeparture("CSA1", DepartureClearance{SID: "VOZ 5M", Runway: "24"}), PosDelivery, IntentDepartureClearance, "CSA1, cleared VOZ 5M departure, flight planned route, runway 24", ParamSID, "VOZ 5M"},
		{ClearedArrival("CSA1", "GOLOP 4S", "ILS", "24", "flight level 100"), PosApproach, IntentArrivalClearance, "CSA1, cleared GOLOP 4S arrival, runway 24, descend to flight level 100, expect ILS approach", ParamSTAR, "GOLOP 4S"}, // Doc 4444 6.5.2.3, 12.3.3.2 a
		{ClearedStartUp("CSA1"), PosGround, IntentStartUp, "CSA1, start up approved", "", ""},   // Doc 4444 12.3.4.3 c
		{ClearedPushback("CSA1"), PosGround, IntentPushback, "CSA1, pushback approved", "", ""}, // 12.3.4.4 b
		{ClearedTaxiToRunway("CSA1", "24", "", []string{"B2", "H", "A"}), PosGround, IntentTaxi, "CSA1, taxi to and hold short of runway 24 via B2, H, A", ParamTaxiways, "B2, H, A"},
		{ClearedTaxiToRunway("CSA1", "24", "B", nil), PosGround, IntentTaxi, "CSA1, taxi to and hold short of runway 24 at B", ParamEntry, "B"},
		{ClearedTaxiToStand("CSA1", "C22", []string{"B", "D"}), PosGround, IntentTaxi, "CSA1, taxi to stand C22 via B, D", ParamStand, "C22"},
		{ClearedTaxiUpTo("CSA1", []string{"H"}, "A"), PosGround, IntentTaxiLimit, "CSA1, taxi via H, hold short of A", ParamLimit, "A"},
		{ClearedTaxiUpTo("CSA1", nil, ""), PosGround, IntentTaxiLimit, "CSA1, taxi, hold position at the marked point", "", ""},
		{ClearedCross("CSA1", "12"), PosGround, IntentCross, "CSA1, cross runway 12", ParamRunway, "12"}, // 12.3.4.9: one designator
		{ClearedLineUp("CSA1", "06"), PosTower, IntentLineUp, "CSA1, runway 06, line up and wait", ParamRunway, "06"},
		{ClearedTakeoff("CSA1", "06", ""), PosTower, IntentTakeoff, "CSA1, runway 06, cleared for take-off", "", ""},                                                                                  // 12.3.4.11 a
		{ClearedTakeoff("CSA1", "06", "wind 100 degrees 6 knots"), PosTower, IntentTakeoff, "CSA1, runway 06, cleared for take-off, wind 100 degrees 6 knots", ParamWind, "wind 100 degrees 6 knots"}, // CAP 413 4.27: wind after
		{ClearedToLand("CSA1", "06", "wind 100 degrees 6 knots"), PosTower, IntentLanding, "CSA1, runway 06, cleared to land, wind 100 degrees 6 knots", ParamRunway, "06"},                           // 12.3.4.16 a, CAP 413 4.51
		{HoldPosition("CSA1"), PosGround, IntentHoldPosition, "CSA1, hold position", "", ""},
		{Stop("CSA1"), PosTower, IntentStop, "CSA1, stop immediately, CSA1, stop immediately", "", ""},                                       // 12.3.4.11 e
		{CancelTakeoff("CSA1"), PosTower, IntentCancelTakeoff, "CSA1, hold position, cancel take-off, I say again, cancel take-off", "", ""}, // 12.3.4.11 c
		{GoAround("CSA1", ""), PosTower, IntentGoAround, "CSA1, go around, I say again, go around", "", ""},
		{GoAround("CSA1", "traffic on the runway"), PosTower, IntentGoAround, "CSA1, go around, I say again, go around, traffic on the runway", ParamReason, "traffic on the runway"},
		{Sequenced("CSA1", 2, 8*time.Minute, Absorption{SpeedKts: 210, ExtraNM: 4.9, Left: 3 * time.Minute}), PosApproach, IntentSequence, "CSA1, number 2, for spacing reduce speed to 210 knots, expect 8 minutes delay", ParamNumber, "2"}, // CAP 413 6.23, 6.24
		{Sequenced("CSA1", 2, 5*time.Minute, Absorption{SpeedKts: 210, ExtraNM: 4.9}), PosApproach, IntentSequence, "CSA1, number 2, for spacing reduce speed to 210 knots", ParamNumber, "2"}, // five minutes or less: not said
		{Sequenced("OKYDV", 3, 2*time.Minute, Absorption{ExtraNM: 4, Orbit: "left"}), PosApproach, IntentSequence, "OKYDV, number 3, orbit left for spacing", ParamNumber, "3"},
		{Sequenced("CSA1", 0, 2*time.Minute, Absorption{SpeedKts: 190}), PosApproach, IntentSequence, "CSA1, for spacing reduce speed to 190 knots", ParamSpeed, "190"}, // the number told already
		{Sequenced("CSA1", 1, 0, Absorption{SpeedKts: 233}), PosApproach, IntentSequence, "CSA1, number 1, for spacing reduce speed to 233 knots", ParamSpeed, "233"},
		{DirectToFinal("CSA1", 3), PosApproach, IntentDirect, "CSA1, proceed direct to final, number 3", ParamNumber, "3"},
		{HoldAt("CSA1", "PR711", EntryTeardrop, 6000, efc), PosApproach, IntentHold, "CSA1, hold at PR711 as published, maintain 6000 feet, expect further clearance at 1042", ParamFix, "PR711"}, // Doc 4444 12.3.3.3 b, CAP 413 6.11
		{LeaveHoldAt("CSA1", "PR711", 2), PosApproach, IntentLeaveHold, "CSA1, leave PR711, number 2, continue the arrival", "", ""},
		{HoldDescend("CSA1", 5000), PosApproach, IntentHoldLevel, "CSA1, descend to 5000 feet", ParamAltitude, "5000"}, // 12.3.1.2 a
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
		{Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 21000, Why: why}, 20000, 90, 450, "CSA1, climb to flight level 210, due traffic"},
		{Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 8000, Why: why}, 9000, 90, 250, "CSA1, descend to 8000 feet, due traffic"},
		{Resolution{Callsign: "CSA1", Kind: ResolveSpeed, Kts: 384, Why: why}, 30000, 90, 480, "CSA1, reduce speed to 384 knots, due traffic"},
		{Resolution{Callsign: "CSA1", Kind: ResolveHeading, HeadingDeg: 110, Why: why}, 20000, 90, 450, "CSA1, turn right heading 110, due traffic"},
		{Resolution{Callsign: "CSA1", Kind: ResolveHeading, HeadingDeg: 70, Why: why}, 20000, 90, 450, "CSA1, turn left heading 070, due traffic"},
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
	r.Transmit("LKPR", ClearedTakeoff("CSA2", "24", ""))
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
	r.Transmit("LKPR", ClearedTakeoff("CSA2", "24", ""))
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

// Wind, procedures and levels as said (docs/traffic-phraseology.md).
func TestSaidValues(t *testing.T) {
	for got, want := range map[string]string{
		WindSaid(104, 6, 0):              "wind 100 degrees 6 knots", // Doc 4444 12.3.1.8 a
		WindSaid(268, 18, 28):            "wind 270 degrees 18 knots gusting 28 knots",
		WindSaid(3, 5, 0):                "wind 360 degrees 5 knots",
		WindSaid(90, 0.4, 0):             "wind calm",
		SaidProcedure("BALT7D", "BALTU"): "BALTU 7D", // CAP 413 2.68: "Wicken 3 Delta departure"
		SaidProcedure("VLM6T", ""):       "VLM 6T",
		SaidProcedure("GOLO4S", "LOMKI"): "GOLO 4S",
		LevelSaidAbove(5000, 5000):       "5000 feet", // Doc 4444 12.3.1.1 c
		LevelSaidAbove(7000, 5000):       "flight level 070",
		LevelSaid(21000):                 "flight level 210",
		LevelSaid(9000):                  "9000 feet",
	} {
		if got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
}

func TestRunwayChangePhrase(t *testing.T) {
	tx := RunwayChange(PosGround, "CSA1", "06", "VOZ 2D", "", "")
	if tx.Text != "CSA1, runway change, runway 06 in use, VOZ 2D departure" {
		t.Errorf("departure: %q", tx.Text)
	}
	tx = RunwayChange(PosApproach, "CSA2", "06", "", "GOLOP 2B", "ILS")
	if tx.Text != "CSA2, runway change, runway 06 in use, GOLOP 2B arrival, expect ILS approach" {
		t.Errorf("arrival: %q", tx.Text)
	}
	if rb, _ := Readback(tx); !strings.Contains(rb.Text, "Runway 06 in use, GOLOP 2B arrival") {
		t.Errorf("readback: %q", rb.Text)
	}
}

func TestPushbackFacingPhrase(t *testing.T) {
	tx := WithFacing(ClearedPushbackAndStartUp("CSA1"), "east")
	if tx.Text != "CSA1, pushback and start up approved, facing east" {
		t.Errorf("%q", tx.Text)
	}
	if rb, _ := Readback(tx); !strings.HasPrefix(rb.Text, "Pushback and start up approved, facing east") {
		t.Errorf("readback %q", rb.Text)
	}
	if tx := WithFacing(ClearedStartUp("CSA1"), "east"); strings.Contains(tx.Text, "facing") {
		t.Errorf("start-up %q", tx.Text)
	}
}

func TestApproachClearancePhrase(t *testing.T) {
	tx := ClearedApproach("CSA1", "ILS", "24")
	if tx.Text != "CSA1, cleared ILS approach runway 24" || tx.Position != PosApproach {
		t.Errorf("%q from %s", tx.Text, tx.Position)
	}
	if rb, _ := Readback(tx); !strings.HasPrefix(rb.Text, "Cleared ILS approach runway 24") {
		t.Errorf("readback %q", rb.Text)
	}
}

// FAA wording at US airports, ICAO elsewhere (#463); the radio says a
// clearance the FAA's way by the airport, and so does the readback.
func TestPhraseologyByRegion(t *testing.T) {
	for icao, want := range map[string]Phraseology{"KJFK": PhraseologyFAA, "PHNL": PhraseologyFAA, "TJSJ": PhraseologyFAA, "LKPR": PhraseologyICAO, "EGLL": PhraseologyICAO, "K1": PhraseologyICAO} {
		if got := PhraseologyFor(icao); got != want {
			t.Errorf("%s: %s, want %s", icao, got, want)
		}
	}
	r := NewRadio(RadioOptions{ReadBack: true})
	var said []string
	r.opts.OnTransmission = func(tx Transmission) { said = append(said, tx.Text) }
	r.Transmit("KJFK", ClearedTakeoff("AAL1", "04L", "wind 040 degrees 8 knots"))
	r.Transmit("KJFK", ClearedTaxiToRunway("AAL1", "04L", "", []string{"B", "A"}))
	r.Transmit("LKPR", ClearedTakeoff("CSA1", "24", "wind 240 degrees 8 knots"))
	want := []string{
		"AAL1, runway 04L, cleared for takeoff", "Runway 04L, cleared for takeoff, AAL1",
		"AAL1, runway 04L, taxi via B, A", "Runway 04L, taxi via B, A, AAL1",
		"CSA1, runway 24, cleared for take-off, wind 240 degrees 8 knots",
	}
	for i, w := range want {
		if i >= len(said) || said[i] != w {
			t.Fatalf("said %q, want %q at %d", said, w, i)
		}
	}
	tx := Say(Transmission{Phraseology: PhraseologyFAA, Callsign: "AAL1", Intent: IntentDepartureClearance, Params: map[string]string{ParamDest: "Boston", ParamSID: "KENNEDY 5", ParamLevel: "5000 feet", ParamSquawk: "4521"}})
	if tx.Text != "AAL1, cleared to Boston airport, KENNEDY 5 departure, then as filed, climb via SID except maintain 5000, squawk 4521" {
		t.Errorf("FAA departure clearance %q", tx.Text)
	}
}

func TestIdentifiedRushAndApproach(t *testing.T) {
	if tx := Identified(PosDeparture, "CSA1", "flight level 240"); tx.Text != "CSA1, identified, climb to flight level 240" {
		t.Errorf("%q", tx.Text)
	}
	if rb, ok := Readback(Identified(PosDeparture, "CSA1", "flight level 240")); !ok || rb.Text != "Climb to flight level 240, CSA1" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	for in, want := range map[*Transmission]string{
		ptr(Rushed(ClearedTakeoff("CSA1", "24", ""))): "CSA1, runway 24, cleared for immediate take-off",
		ptr(Rushed(ClearedLineUp("CSA1", "24"))):      "CSA1, runway 24, line up, be ready for immediate departure",
		ptr(Rushed(ClearedCross("CSA1", "12"))):       "CSA1, expedite crossing runway 12",
		ptr(Rushed(HoldPosition("CSA1"))):             "CSA1, hold position",
	} {
		if in.Text != want {
			t.Errorf("%q, want %q", in.Text, want)
		}
	}
	tx := ClearedApproachTo("CSA1", ApproachClearance{Kind: "ILS", Runway: "24", QNH: "1013", ReportEstablished: true})
	if tx.Text != "CSA1, cleared ILS approach runway 24, QNH 1013, report established" {
		t.Errorf("%q", tx.Text)
	}
}

func ptr(t Transmission) *Transmission { return &t }

func TestWeatherAndDirectRequests(t *testing.T) {
	if tx := RequestWeather(PosTower, "CSA1"); tx.Text != "CSA1, request weather" || !tx.Pilot {
		t.Errorf("%+v", tx)
	}
	w := WeatherReport(PosTower, "CSA1", "wind 240 degrees 8 knots", "1013", "")
	if w.Text != "CSA1, wind 240 degrees 8 knots, QNH 1013" {
		t.Errorf("%q", w.Text)
	}
	if rb, ok := Readback(w); !ok || rb.Text != "QNH 1013, CSA1" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	faa := Say(Transmission{Phraseology: PhraseologyFAA, Callsign: "AAL1", Intent: IntentWeather, Params: map[string]string{ParamWind: "wind 040 at 8", ParamAltimeter: "2992"}})
	if faa.Text != "AAL1, wind 040 at 8, altimeter 2992" {
		t.Errorf("%q", faa.Text)
	}
	if d := ClearedDirectTo(PosApproach, "CSA1", "GOLOP"); d.Text != "CSA1, cleared direct to GOLOP" {
		t.Errorf("%q", d.Text)
	}
}

func TestClearedTakeoffNoDelayReadback(t *testing.T) {
	rb, ok := Readback(ClearedTakeoffNoDelay("CSA716", "24", "", 0.4))
	if !ok || rb.Text != "Runway 24, cleared for take-off, no delay, CSA716" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	if tx := ClearedTakeoffNoDelay("CSA716", "24", "", 0.4); !strings.HasSuffix(tx.Text, "traffic on 1 mile final") {
		t.Errorf("%q", tx.Text)
	}
}
