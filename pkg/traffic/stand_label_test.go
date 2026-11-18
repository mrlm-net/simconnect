package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestStandLabelSaid: a stand's free-text label goes into the radio text
// as tokens a voice library spells; the sim's labels as they were (#1556).
func TestStandLabelSaid(t *testing.T) {
	for in, want := range map[string]string{
		"C22": "C22", "S22A": "S22A", "22": "22",
		"H17-1": "H17-1", "1B A380": "1B A380", "1b": "1B", "H17/1": "H17 1", " FDX6 ": "FDX6", "Gate  5.": "Gate 5", "TBA South Pen": "TBA South Pen", "h17-1": "H17-1",
	} {
		if got := standSaid(in); got != want {
			t.Errorf("standSaid(%q) = %q, want %q", in, got, want)
		}
	}

	p := airport.Parking{Name: types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_H, Number: 17}
	if got := ClearedTaxiToStand("CSA1", p.Label(), []string{"B", "D"}).Text; got != "CSA1, taxi to stand H17 via B, D" {
		t.Errorf("no label: %q", got)
	}
	for _, c := range []struct{ label, taxi, push, rb string }{
		{"H17-1", "CSA1, taxi to stand H17-1 via B, D", "Ruzyne Ground, CSA1, stand H17-1, information Bravo, request pushback", "Taxi to stand H17-1 via B, D, CSA1"},
		{"1B A380", "CSA1, taxi to stand 1B A380 via B, D", "Ruzyne Ground, CSA1, stand 1B A380, information Bravo, request pushback", "Taxi to stand 1B A380 via B, D, CSA1"},
	} {
		p.LabelText = c.label
		tx := ClearedTaxiToStand("CSA1", p.Label(), []string{"B", "D"})
		if tx.Text != c.taxi {
			t.Errorf("%s taxi: %q, want %q", c.label, tx.Text, c.taxi)
		}
		if tx.Params[ParamStand] != c.label {
			t.Errorf("%s: ParamStand %q", c.label, tx.Params[ParamStand])
		}
		if rb, ok := Readback(tx); !ok || rb.Text != c.rb {
			t.Errorf("%s readback: %q %v, want %q", c.label, rb.Text, ok, c.rb)
		}
		if got := RequestPushback("Ruzyne Ground", "CSA1", p.Label(), "Bravo").Text; got != c.push {
			t.Errorf("%s pushback: %q, want %q", c.label, got, c.push)
		}
		if got := RequestTaxiFrom("CSA1", p.Label()).Text; got != "CSA1, stand "+c.label+", request taxi" {
			t.Errorf("%s request taxi: %q", c.label, got)
		}
	}
}
