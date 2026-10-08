package traffic

import "testing"

// TestDepartureOrder: the tower's answer to a departure checking in, and
// its readback (CAP 413 4.19, 4.20).
func TestDepartureOrder(t *testing.T) {
	for _, c := range []struct {
		ahead    int
		said, rb string
	}{
		{0, "CSA1, report ready for departure", "Wilco, CSA1"},
		{2, "CSA1, hold short of runway 24, 2 aircraft to depart before you", "Holding short of runway 24, CSA1"},
	} {
		tx := DepartureOrder("CSA1", "24", c.ahead)
		if tx.Text != c.said {
			t.Errorf("%d ahead: %q, want %q", c.ahead, tx.Text, c.said)
		}
		rb, ok := Readback(tx)
		if !ok || rb.Text != c.rb {
			t.Errorf("%d ahead: readback %q (%v), want %q", c.ahead, rb.Text, ok, c.rb)
		}
	}
	rb, ok := Readback(CircuitInstruction("AUA762", InstrContinue))
	if !ok || rb.Text != "Continue approach, AUA762" {
		t.Errorf("continue approach readback %q (%v)", rb.Text, ok)
	}
}

// TestClearOfConflictTo: the crew names the clearance it returns to, the
// ICAO way (Doc 4444 12.3.1.2 t); none: the FAA's "assigned altitude".
func TestClearOfConflictTo(t *testing.T) {
	for _, c := range []struct {
		tx   Transmission
		want string
	}{
		{Descend(PosApproach, "CSA1", 10000, 5000), "flight level 100"},
		{Descend(PosApproach, "CSA1", 4000, 5000), "4000 feet"},
		{ClearedVisual(PosApproach, "CSA1", "24"), "visual approach runway 24"},
	} {
		got, ok := AssignedClearance(c.tx)
		if !ok || got != c.want {
			t.Errorf("%q: %q (%v), want %q", c.tx.Text, got, ok, c.want)
		}
	}
	if _, ok := AssignedClearance(GoingAround("CSA1")); ok {
		t.Error("a pilot's call taken as a clearance")
	}
	if s := ClearOfConflictTo(PosTower, "Ruzyne Tower", "CSA1", "4000 feet").Text; s != "Ruzyne Tower, CSA1, clear of conflict, returning to 4000 feet" {
		t.Errorf("%q", s)
	}
	if s := ClearOfConflict(PosCenter, "", "UAL321").Text; s != "UAL321, clear of conflict, returning to assigned altitude" {
		t.Errorf("FAA: %q", s)
	}
}

// TestPhraseGaps: the hold short in a taxi clearance and its readback, the
// ready and vacated calls with the station, the established report by
// approach kind.
func TestPhraseGaps(t *testing.T) {
	tx := WithHoldShort(ClearedTaxiToRunway("CSA1", "24", "", []string{"A", "B"}), "12")
	if tx.Text != "CSA1, taxi to and hold short of runway 24 via A, B, hold short of runway 12" {
		t.Errorf("taxi: %q", tx.Text)
	}
	if rb, _ := Readback(tx); rb.Text != "Taxi to and hold short of runway 24 via A, B, hold short of runway 12, CSA1" {
		t.Errorf("readback: %q", rb.Text)
	}
	if s := WithHoldShort(ClearedTaxiToRunway("CSA1", "24", "", []string{"A"}), "12", "30").Text; s != "CSA1, taxi to and hold short of runway 24 via A, hold short of runways 12 and 30" {
		t.Errorf("two: %q", s)
	}
	if s := ReadyForDepartureTo("Ruzyne Tower", "CSA1", "24", "").Text; s != "Ruzyne Tower, CSA1, holding short runway 24, ready for departure" {
		t.Errorf("ready: %q", s)
	}
	if s := VacatedAt("Frankfurt Ground", "CSA1", "25R", "N8").Text; s != "Frankfurt Ground, CSA1, runway 25R vacated via N8" {
		t.Errorf("vacated: %q", s)
	}
	if s := EstablishedReportFor("CSA1", "25R", "RNP").Text; s != "Established RNP runway 25R, CSA1" {
		t.Errorf("RNP: %q", s)
	}
	if s := EstablishedReportFor("CSA1", "24", "ILS").Text; s != "Localizer established runway 24, CSA1" {
		t.Errorf("ILS: %q", s)
	}
}
