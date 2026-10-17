package traffic

import "testing"

// A 35 m stand pulls narrowbodies back; a widebody keeps the stop worked
// out from the circle; a stand without a rule has none.
func TestStandRuleFor(t *testing.T) {
	if r, ok := StandRuleFor("lkpr", "B14", 35.8); !ok || r.ForwardM != -9 {
		t.Errorf("LKPR B14 for an A321: %+v %v, want 9 m back", r, ok)
	}
	if _, ok := StandRuleFor("LKPR", "B14", 60.1); ok {
		t.Error("LKPR B14 for an A350: a rule, want none (narrowbodies only)")
	}
	if _, ok := StandRuleFor("LKPR", "B13", 35.8); ok {
		t.Error("LKPR B13: a rule, want none")
	}
}
