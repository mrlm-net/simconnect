package traffic

import "testing"

// TestSpacingToldOnce: "for spacing" is said with the first number of an
// approach; a new number later goes without it (the user, live: CSA1080
// heard "for spacing" four times on one approach).
func TestSpacingToldOnce(t *testing.T) {
	first := Sequenced("CSA1080", 4, 0, Absorption{SpeedKts: 230})
	if want := "CSA1080, number 4, for spacing reduce speed to 230 knots"; first.Text != want {
		t.Errorf("first: %q, want %q", first.Text, want)
	}
	again := SpacingTold(Sequenced("CSA1080", 3, 0, Absorption{SpeedKts: 220}))
	if want := "CSA1080, number 3, reduce speed to 220 knots"; again.Text != want {
		t.Errorf("again: %q, want %q", again.Text, want)
	}
	final := SpacingTold(SequencedFinalSpeed(PosTower, "CSA1080", 2))
	if want := "CSA1080, number 2, reduce to final approach speed"; final.Text != want {
		t.Errorf("final: %q, want %q", final.Text, want)
	}
}
