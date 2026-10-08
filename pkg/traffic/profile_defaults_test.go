package traffic

import "testing"

// A profile given in part keeps the defaults of the fields it leaves out
// (E34).
func TestWithDefaults(t *testing.T) {
	def := DefaultTakeoffProfile()
	got := withDefaults(TakeoffProfile{ClimbPitch: 20}, def)
	if got.ClimbPitch != 20 || got.RotateKts != def.RotateKts || got.LiftoffPitch != def.LiftoffPitch {
		t.Errorf("partial take-off profile %+v", got)
	}
	if got := withDefaults(TakeoffProfile{}, def); got != def {
		t.Errorf("empty profile %+v, want the default", got)
	}
}
