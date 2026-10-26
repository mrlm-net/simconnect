package traffic

import "testing"

// TestVectoringSaysTheApproach: the first vector off the STAR tells the
// approach and runway it leads to; the readback is the vector's.
func TestVectoringSaysTheApproach(t *testing.T) {
	v := Vector{HeadingDeg: 60, For: "spacing"}
	tx := Vectoring(Vectored("CSA1", v, 0), "ILS", "24")
	if want := "CSA1, vectoring for ILS approach runway 24, fly heading 060, for spacing"; tx.Text != want {
		t.Errorf("got %q, want %q", tx.Text, want)
	}
	if r, ok := Readback(tx); !ok || r.Text != "Fly heading 060, CSA1" {
		t.Errorf("readback %q", r.Text)
	}
	// Resume own navigation is no vectoring.
	if d := Vectoring(Vectored("CSA1", Vector{Fix: "GOLOP"}, 0), "ILS", "24"); d.Text != "CSA1, resume own navigation direct GOLOP" {
		t.Errorf("direct: %q", d.Text)
	}
}
