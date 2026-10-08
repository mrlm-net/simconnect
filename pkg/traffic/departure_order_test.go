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
