package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// On LKPR B14 (a 35 m circle) an A320 stops 9 m further back than the
// circle says, a local rule over the shipped one wins, and saved it is
// read again (the GSX way).
func TestStandRulesPlace(t *testing.T) {
	defer dict.Reset("traffic.standRules")
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	si, err := g.Layout.ParkingIndex("B14")
	if err != nil {
		t.Fatal(err)
	}
	var cc *controlCenter
	base, _ := cc.standPlaceSim(g, si, "FSLTL_FAIB_A320_AUA-Austrian")
	if base <= 0 {
		base = traffic.DefaultNoseOffsetMeters
	}
	nose, side := cc.standPlace(g, si, "FSLTL_FAIB_A320_AUA-Austrian")
	if nose != base+9 || side != 0 {
		t.Errorf("B14 A320: nose offset %.2f side %.2f, want %.2f, 0", nose, side, base+9)
	}
	dir := t.TempDir()
	w := New(Options{DataDir: dir})
	if err := w.SetStandRule(traffic.StandRuleItem{Stand: "lkpr  b14", ForwardM: -10, RightM: 0.5}); err != nil {
		t.Fatal(err)
	}
	if nose, side := cc.standPlace(g, si, "FSLTL_FAIB_A320_AUA-Austrian"); nose != base+10 || side != 0.5 {
		t.Errorf("local rule: nose offset %.2f side %.2f, want %.2f, 0.5", nose, side, base+10)
	}
	if err := dict.Reset("traffic.standRules"); err != nil {
		t.Fatal(err)
	}
	New(Options{DataDir: dir})
	if r, ok := traffic.StandRuleFor("LKPR", "B14", 35); !ok || r.ForwardM != -10 {
		t.Errorf("read again: %+v %v, want the local -10 m", r, ok)
	}
}
