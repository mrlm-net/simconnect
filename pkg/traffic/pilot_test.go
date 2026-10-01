//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"
)

// Each clearance is read back as docs/traffic-phraseology.md quotes it
// (Doc 4444 4.5.7.5, CAP 413): its items, then the call sign; on the
// controller's frequency.
func TestReadbacks(t *testing.T) {
	for _, c := range []struct {
		clr  Transmission
		want string
	}{
		{ClearedDeparture("CSA1", DepartureClearance{Destination: "Frankfurt", SID: "BALTU 7D", Runway: "24", Level: "5000 feet", Squawk: "4521"}), "Cleared to Frankfurt, BALTU 7D departure, flight planned route, runway 24, climb via SID to 5000 feet, squawk 4521, CSA1"}, // CAP 413 2.68
		{ClearedStartUp("CSA1"), "Start up approved, CSA1"},
		{ClearedPushback("CSA1"), "Pushback approved, CSA1"},
		{ClearedTaxiToRunway("CSA1", "24", "B", []string{"H", "A"}), "Taxi to and hold short of runway 24 at B via H, A, CSA1"},
		{GiveWay("CSA1", "A320 passing left to right"), "Giving way to the A320 passing left to right, CSA1"},
		{ClearedTaxiToStand("CSA1", "C22", []string{"B", "D"}), "Taxi to stand C22 via B, D, CSA1"},
		{ClearedTaxiUpTo("CSA1", []string{"H"}, "A"), "Holding short of A, CSA1"},
		{ClearedCross("CSA1", "12"), "Cross runway 12, CSA1"},
		{ClearedLineUp("CSA1", "24"), "Runway 24, line up and wait, CSA1"},
		{ClearedTakeoff("CSA1", "24", "wind 100 degrees 6 knots"), "Runway 24, cleared for take-off, CSA1"}, // CAP 413 4.30
		{ClearedToLand("CSA1", "06", "wind 100 degrees 6 knots"), "Runway 06, cleared to land, CSA1"},
		{WhenVacatedContact("CSA1", PosTower, PosGround, "Ruzyne Ground", "121.91"), "When vacated Ruzyne Ground 121.91, CSA1"}, // CAP 413 4.68
		{HoldPosition("CSA1"), "Holding, CSA1"},                                                                                 // Doc 4444 12.3.4.8 note
		{GoAround("CSA1", "GAT1 on the runway"), "Going around, CSA1"},
		{Handoff("CSA1", PosGround, PosTower, "Ruzyne Tower", "134.56"), "Ruzyne Tower 134.56, CSA1"},
		{Resolved(PosCenter, Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 21000}, 20000, 90, 450), "Climb to flight level 210, CSA1"},
		{Resolved(PosCenter, Resolution{Callsign: "CSA1", Kind: ResolveSpeed, Kts: 250}, 20000, 90, 300), "Reduce speed to 250 knots, CSA1"},
	} {
		rb, ok := Readback(c.clr)
		if !ok || rb.Text != c.want || !rb.Pilot || rb.Position != c.clr.Position || rb.Params[ParamIntent] != string(c.clr.Intent) {
			t.Errorf("%s: %+v (%v), want %q", c.clr.Intent, rb, ok, c.want)
		}
	}
}

// A wrong readback is corrected with the clearance again, "negative"; a
// right one (as said, however written) passes.
func TestCheckReadback(t *testing.T) {
	clr := ClearedTaxiUpTo("CSA1", []string{"H"}, "A")
	if c, ok := CheckReadback(clr, map[string]string{ParamLimit: "B"}); ok || c.Text != "CSA1, negative, taxi via H, hold short of A" || c.Intent != IntentCorrection {
		t.Errorf("wrong limit: %+v %v", c, ok)
	}
	if _, ok := CheckReadback(ClearedLineUp("CSA1", "06"), map[string]string{ParamRunway: "6"}); !ok {
		t.Error("runway 6 for 06 refused")
	}
	lvl := Resolved(PosCenter, Resolution{Callsign: "CSA1", Kind: ResolveLevel, AltFt: 21000}, 20000, 90, 450)
	if _, ok := CheckReadback(lvl, map[string]string{ParamLevel: "FL210"}); !ok {
		t.Error("FL210 for flight level 210 refused")
	}
	if _, ok := CheckReadback(lvl, map[string]string{}); ok {
		t.Error("a level not read back passed")
	}
}

// Calls: requests, a first call with its report and the ATIS letter.
func TestPilotCalls(t *testing.T) {
	for _, c := range []struct {
		tx   Transmission
		want string
	}{
		{RequestClearance("Ruzyne Delivery", "CSA1", "A4", "Bravo", "Frankfurt"), "Ruzyne Delivery, CSA1, stand A4, information Bravo, request clearance to Frankfurt"},
		{RequestPushback("Ruzyne Ground", "CSA1", "A4", "Bravo"), "Ruzyne Ground, CSA1, stand A4, information Bravo, request pushback"}, // Doc 4444 12.3.4.4 a, CAP 413 4.9 order
		{RequestStartUp("", "CSA1", "", ""), "CSA1, request start up"},                                                                  // Doc 4444 12.3.4.3 a
		{ReadbackCorrect(PosDelivery, "CSA1"), "CSA1, readback correct"},
		{RequestTaxi("CSA1"), "CSA1, request taxi"},
		{ReadyForDeparture("CSA1", "24", ""), "CSA1, holding short runway 24, ready for departure"},
		{ReadyForDeparture("CSA1", "24", "Z"), "CSA1, holding short runway 24 at Z, ready for departure"},
		{HoldingShortReport("CSA1", "12", "F"), "CSA1, holding short of runway 12 at F"},
		{CheckIn(PosTower, "Ruzyne Tower", "CSA1", "established ILS runway 06", ""), "Ruzyne Tower, CSA1, established ILS runway 06"},
		{Vacated("CSA1", "06"), "CSA1, runway vacated"}, // Doc 4444 12.3.4.7 z
		{SayAgain(PosTower, "CSA1"), "CSA1, say again"},
		{SayAgain(PosTower, ""), "Station calling, say again your call sign"},
	} {
		if c.tx.Text != c.want {
			t.Errorf("%s: %q, want %q", c.tx.Intent, c.tx.Text, c.want)
		}
	}
}

// With ReadBack the pilot reads back after the clearance, on its frequency.
func TestRadioReadsBack(t *testing.T) {
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	var heard []Transmission
	r := NewRadio(RadioOptions{ReadBack: true, Now: func() time.Time { return at },
		FrequencyOf:    func(string, Position) string { return "134.56" },
		OnTransmission: func(t Transmission) { heard = append(heard, t) }})
	r.Transmit("LKPR", ClearedTakeoff("CSA1", "24", ""))
	if len(heard) != 2 || !heard[1].Pilot || heard[1].Frequency != "134.56" || !heard[1].At.After(heard[0].At) {
		t.Fatalf("%+v", heard)
	}
	r.Transmit("LKPR", RequestTaxi("CSA2")) // a pilot's call is not read back
	if len(heard) != 3 {
		t.Errorf("%d transmissions after a pilot's call", len(heard))
	}
}

func TestATISInformation(t *testing.T) {
	tx := ATISInformation("Bravo", "Ruzyne information B, runway 24")
	if tx.Position != PosATIS || tx.Intent != IntentATIS || tx.Params[ParamInfo] != "Bravo" || tx.Pilot {
		t.Fatalf("ATIS transmission = %+v", tx)
	}
	r := NewRadio(RadioOptions{FrequencyOf: func(_ string, pos Position) string {
		if pos == PosATIS {
			return "122.155"
		}
		return ""
	}})
	if got := r.Transmit("LKPR", tx); got.Frequency != "122.155" {
		t.Errorf("ATIS on %q, want its frequency", got.Frequency)
	}
	if _, ok := Readback(tx); ok {
		t.Error("an ATIS is not read back")
	}
}
