package world

import (
	"strings"
	"testing"
)

// Behind one of ours early on its roll, the line-up is conditional; late
// in it (about to lift off) or off the runway, a plain line-up (the user,
// live: "behind the departing Boeing 737, line up and wait runway 24,
// behind" with the 737 about to lift off).
func TestLineUpBehindDepartingSaid(t *testing.T) {
	for _, c := range []struct {
		kts     float64
		rolling bool
		behind  bool
	}{{30, true, true}, {79, true, true}, {80, true, false}, {140, true, false}, {0, false, false}} {
		tx := lineUpBehindDepartingSaid("OKKBC", "Boeing 737", "24", c.kts, c.rolling)
		if got := strings.Contains(tx.Text, "behind"); got != c.behind {
			t.Errorf("%v kt (rolling %v): %q, behind %v, want %v", c.kts, c.rolling, tx.Text, got, c.behind)
		}
		if !strings.Contains(tx.Text, "line up and wait") {
			t.Errorf("%v kt: %q, no line-up", c.kts, tx.Text)
		}
	}
}
