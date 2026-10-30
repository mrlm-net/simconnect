package traffic

import (
	"errors"
	"testing"
)

// TestRunwayTooShort: a departure is not moved to a runway shorter than
// its take-off needs, the day's temperature counted (#1232); on a cooler
// day the same runway is taken, and its own runway stays its.
func TestRunwayTooShort(t *testing.T) {
	g := lkprGraph(t)
	long, _, _ := g.Layout.RunwayEnd("24")
	short, _, _ := g.Layout.RunwayEnd("12")
	if short.Length >= long.Length {
		t.Skipf("LKPR 12/30 %.0f m not shorter than 06/24 %.0f m", short.Length, long.Length)
	}
	ctl, _, _, _ := injectedDeparture(t, TaxiRequest{})
	req := ctl.req
	need0 := req.takeoffRun()
	req.ISADeviationC = 1
	perC := req.takeoffRun() - need0
	isa := ((short.Length+long.Length)/2 - need0) / perC // needs more than 12/30, less than 06/24
	if perC <= 0 || isa <= 0 {
		t.Skipf("A320 needs %.0f m at ISA, %.1f m/°C", need0, perC)
	}
	if err := ctl.ChangeRunway("12", "", nil); err != nil {
		t.Fatalf("ISA: %v", err)
	}
	hot, _, _, _ := injectedDeparture(t, TaxiRequest{ISADeviationC: isa})
	if err := hot.ChangeRunway("12", "", nil); !errors.Is(err, ErrRunwayTooShort) {
		t.Errorf("ISA+%.0f: %v, want ErrRunwayTooShort", isa, err)
	}
	if r := hot.Route(); r == nil || r.RunwayEnd != "24" {
		t.Errorf("refused, its route goes elsewhere (%v), want 24 still", r != nil && r.RunwayEnd == "24")
	}
	if err := hot.ChangeRunway("24", "", nil); err != nil {
		t.Errorf("its own runway: %v", err)
	}
	t.Logf("A320 at LKPR: %.0f m at ISA, ISA+%.0f needs more than 12/30 (%.0f m)", need0, isa, short.Length)
}
