package traffic

import (
	"testing"
	"time"
)

// TestEmergencyPhrases: a MAYDAY and a PAN PAN by what happened, the
// acknowledgement and its readback.
func TestEmergencyPhrases(t *testing.T) {
	tx := DeclareEmergency(PosApproach, "Ruzyne Approach", "Skytravel 555", EmergencyEngineFailure, "", "24")
	if tx.Text != "MAYDAY MAYDAY MAYDAY, Ruzyne Approach, Skytravel 555, engine failure, request priority landing runway 24" {
		t.Errorf("mayday: %q", tx.Text)
	}
	if tx := DeclareEmergency(PosApproach, "", "CSA1", EmergencyMedical, "", ""); tx.Text != "PAN PAN, PAN PAN, PAN PAN, CSA1, medical emergency on board, request priority landing" {
		t.Errorf("pan pan: %q", tx.Text)
	}
	ack := AcknowledgeEmergency(PosApproach, "Skytravel 555", LevelMayday, "24")
	if ack.Text != "Skytravel 555, roger the MAYDAY, squawk 7700, you are number one, runway 24" {
		t.Errorf("ack: %q", ack.Text)
	}
	if rb, ok := Readback(ack); !ok || rb.Text != "Squawk 7700, number one, runway 24, Skytravel 555" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
	for k, want := range map[EmergencyKind]string{EmergencyFire: LevelMayday, EmergencyPressurisation: LevelMayday, EmergencyFuel: LevelPanPan, EmergencyHydraulic: LevelPanPan} {
		if got := EmergencyLevel(k); got != want {
			t.Errorf("%s: %s, want %s", k, got, want)
		}
	}
}

// TestCustomCall: a scenario's own words said as written, acknowledged
// "Roger".
func TestCustomCall(t *testing.T) {
	tx := CustomCall(PosTower, "CSA4KM", "CSA4KM, bird activity reported on short final runway 24")
	if tx.Text != "CSA4KM, bird activity reported on short final runway 24" || tx.Pilot {
		t.Errorf("said %q (pilot %v)", tx.Text, tx.Pilot)
	}
	if rb := Roger(PosTower, "CSA4KM"); rb.Text != "Roger, CSA4KM" || !rb.Pilot {
		t.Errorf("acknowledged %q", rb.Text)
	}
}

// TestAfterDepartureClimb: tower's restriction before the take-off and
// its readback, as the ICAO phraseology guide words them.
func TestAfterDepartureClimb(t *testing.T) {
	tx := AfterDepartureClimb("OKETU", 5000)
	if tx.Text != "OKETU, after departure climb to altitude 5000 feet" {
		t.Errorf("said %q", tx.Text)
	}
	if rb, ok := Readback(tx); !ok || rb.Text != "After departure climb to 5000 feet, OKETU" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
}

// TestCancelEmergency: a crew cancelling its emergency and ATC's answer,
// with the code going back read back.
func TestCancelEmergency(t *testing.T) {
	if tx := CancelEmergency(PosApproach, "Praha Approach", "CSA123", LevelMayday); tx.Text != "Praha Approach, CSA123, cancel MAYDAY" {
		t.Errorf("cancel: %q", tx.Text)
	}
	if tx := CancelEmergency(PosApproach, "", "CSA123", LevelPanPan); tx.Text != "CSA123, cancel PAN PAN" {
		t.Errorf("cancel pan: %q", tx.Text)
	}
	ack := AcknowledgeCancelEmergency(PosApproach, "CSA123", LevelMayday, "4521")
	if ack.Text != "CSA123, roger, MAYDAY cancelled, squawk 4521" {
		t.Errorf("ack: %q", ack.Text)
	}
	if rb, ok := Readback(ack); !ok || rb.Text != "Squawk 4521, CSA123" {
		t.Errorf("readback %q %v", rb.Text, ok)
	}
}

// TestEmergencyDetails: ATC asks for what the message should carry; the
// crew answers as the guide's example has it, unknown parts left out.
func TestEmergencyDetails(t *testing.T) {
	if q := EmergencyQuestions(PosApproach, "CSA123"); q.Text != "CSA123, report persons on board, fuel endurance and intentions" {
		t.Errorf("asked %q", q.Text)
	}
	d := EmergencyDetails(PosApproach, "CSA123", 156, 2*time.Hour+10*time.Minute, IntentionsLand("24"))
	if d.Text != "CSA123, 156 persons on board, endurance 2 hours 10 minutes, request immediate landing runway 24" || !d.Pilot {
		t.Errorf("answered %q", d.Text)
	}
	if d := EmergencyDetails(PosApproach, "CSA123", 0, 45*time.Minute, ""); d.Text != "CSA123, endurance 45 minutes" {
		t.Errorf("unknown persons: %q", d.Text)
	}
}
