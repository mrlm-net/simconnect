package traffic

import "testing"

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
	if ack.Text != "Skytravel 555, roger MAYDAY, squawk 7700, you are number one, runway 24" {
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
