package airport

import "testing"

// TestSelfAnnounce: fields as the simulator gives their frequencies
// (simconnect-data): a towered one, one with approach and departure but no
// tower (KVUO, clearance via Portland), one with only a UNICOM (AGGH) and
// one whose only frequency is a MULTICOM for the lights (AYBK). Without a
// tower, nobody works tower, ground or clearance, and pilots announce
// themselves on the common frequency (#1296).
func TestSelfAnnounce(t *testing.T) {
	layout := func(raw ...RawFrequency) *Layout { return &Layout{Frequencies: frequenciesOf(raw)} }
	lkpr := layout(RawFrequency{Type: 8, Hz: 118310000, Name: "RUZYNE"}, RawFrequency{Type: 5, Hz: 121910000, Name: "RUZYNE"},
		RawFrequency{Type: 6, Hz: 134560000, Name: "RUZYNE"}, RawFrequency{Type: 3, Hz: 126100000, Name: "PRAGUE INFORMATION"})
	kvuo := layout(RawFrequency{Type: 8, Hz: 118100000, Name: "PORTLAND"}, RawFrequency{Type: 13, Hz: 135125000, Name: "KVUO"},
		RawFrequency{Type: 7, Hz: 119000000, Name: "PORTLAND"}, RawFrequency{Type: 9, Hz: 124350000, Name: "PORTLAND"})
	aggh := layout(RawFrequency{Type: 3, Hz: 118100000, Name: "HENDERSON INFORMATION"})
	aybk := layout(RawFrequency{Type: 2, Hz: 121700000, Name: "ACTIVATE LIGHT ONLY"})

	if !lkpr.Controlled() {
		t.Error("LKPR has a tower")
	}
	if f, ok := lkpr.ControlledFrequency(FreqTower); !ok || f.String() != "134.56" {
		t.Errorf("LKPR tower %v %v", f, ok)
	}
	for _, c := range []struct {
		name string
		l    *Layout
		self string // the self-announce frequency, "" none
	}{
		{"KVUO", kvuo, ""}, // none given
		{"AGGH", aggh, "118.10"},
		{"AYBK", aybk, ""},
	} {
		if c.l.Controlled() {
			t.Errorf("%s: controlled", c.name)
		}
		for _, k := range []string{FreqTower, FreqGround} {
			if f, ok := c.l.ControlledFrequency(k); ok {
				t.Errorf("%s: %s on %s", c.name, k, f)
			}
		}
		if c.self != "" {
			if f, ok := c.l.FrequencyFor(FreqTower); !ok || f.Kind != FreqCTAF {
				t.Errorf("%s: FrequencyFor(tower) %v %v, want the common frequency as before", c.name, f, ok)
			}
		}
		f, ok := c.l.SelfAnnounceFrequency()
		if got := map[bool]string{true: f.String()}[ok]; got != c.self {
			t.Errorf("%s: self-announce on %q, want %q", c.name, got, c.self)
		}
	}
	if f, ok := kvuo.ControlledFrequency(FreqClearance); !ok || f.String() != "119.00" {
		t.Errorf("KVUO clearance %v %v: its own, by Portland", f, ok)
	}
	if f, ok := aggh.ControlledFrequency(FreqClearance); ok {
		t.Errorf("AGGH clearance on %s: none of its own", f)
	}
	if f, ok := kvuo.ControlledFrequency(FreqApproach); !ok || f.String() != "118.10" {
		t.Errorf("KVUO approach %v %v: an IFR flight's release comes from there", f, ok)
	}
	if f, _ := aggh.SelfAnnounceFrequency(); f.Service != ServiceUNICOM {
		t.Errorf("AGGH: service %q, want unicom", f.Service)
	}
}
