package traffic

import "testing"

// The VFR zone calls and their readbacks (the MyCrew app's VFR flow).
func TestVFRZonePhrases(t *testing.T) {
	for _, c := range []struct {
		tx       Transmission
		said, rb string
	}{
		{ReportAt(PosTower, "OKABC", "NOVEMBER"), "OKABC, report at NOVEMBER", "Wilco, OKABC"},
		{ReportLeavingZone(PosTower, "OKABC", "NOVEMBER"), "OKABC, report leaving the zone via NOVEMBER", "Wilco, OKABC"},
		{ReportLeavingZone(PosTower, "OKABC", ""), "OKABC, report leaving the zone", "Wilco, OKABC"},
		{FrequencyChangeApproved(PosTower, "OKABC", ""), "OKABC, frequency change approved", "Frequency change approved, OKABC"},
		{FrequencyChangeApproved(PosTower, "OKABC", "7000"), "OKABC, frequency change approved, squawk 7000", "Frequency change approved, squawk 7000, OKABC"},
		{ContactFIS(PosTower, "OKABC", "Praha Information", "126.1"), "OKABC, contact Praha Information 126.1", "Praha Information 126.1, OKABC"},
	} {
		if c.tx.Text != c.said {
			t.Errorf("said %q, want %q", c.tx.Text, c.said)
		}
		rb, ok := Readback(c.tx)
		if !ok || rb.Text != c.rb {
			t.Errorf("%q read back %q (%v), want %q", c.said, rb.Text, ok, c.rb)
		}
	}
	for _, c := range []struct {
		tx   Transmission
		said string
	}{
		{AtPoint(PosTower, "OKABC", "NOVEMBER", ""), "OKABC, NOVEMBER"},
		{AtPoint(PosTower, "OKABC", "NOVEMBER", "2500 feet"), "OKABC, NOVEMBER, 2500 feet"},
		{LeavingZone(PosTower, "OKABC", "NOVEMBER"), "OKABC, leaving the zone via NOVEMBER"},
	} {
		if c.tx.Text != c.said || !c.tx.Pilot {
			t.Errorf("pilot said %q (pilot %v), want %q", c.tx.Text, c.tx.Pilot, c.said)
		}
		if _, ok := Readback(c.tx); ok {
			t.Errorf("%q: a crew's report is not read back", c.said)
		}
	}
}
