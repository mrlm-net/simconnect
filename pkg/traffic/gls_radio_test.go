package traffic

import "testing"

// TestGLSSaid: an arrival on a GLS approach (the kind is the approach
// name's first word, as pkg/traffic/world takes it from BestApproach) is
// vectored, cleared and reports established with "GLS", the way RNP and
// VOR are said; no localizer is mentioned.
func TestGLSSaid(t *testing.T) {
	tx := Vectoring(Vectored("DLH4", Vector{HeadingDeg: 250, For: "spacing"}, 0), "GLS", "25L")
	if want := "DLH4, vectoring for GLS approach runway 25L, fly heading 250, for spacing"; tx.Text != want {
		t.Errorf("vectoring %q, want %q", tx.Text, want)
	}
	tx = ClearedApproachTo("DLH4", ApproachClearance{Kind: "GLS", Runway: "25L", QNH: "1013", ReportEstablished: true})
	if want := "DLH4, cleared GLS approach runway 25L, QNH 1013, report established"; tx.Text != want {
		t.Errorf("clearance %q, want %q", tx.Text, want)
	}
	if s := EstablishedReportFor("DLH4", "25L", "GLS").Text; s != "Established GLS runway 25L, DLH4" {
		t.Errorf("established %q", s)
	}
}
